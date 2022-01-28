// Copyright (2021-present) Cobalt Speech and Language, Inc. All rights reserved.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmdserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cobaltspeech/log"
)

// Server represents an http server that handles Diatheke
// commands. With this server, all commands go to the same
// URL, so the Diatheke model should be set accordingly.
// Once received, commands are sent to Handlers added with
// the SetHandler function.
type Server struct {
	logger   log.Logger
	registry handlerRegistry
}

// NewServer returns a new command server.
func NewServer(logger log.Logger) Server {
	if logger == nil {
		logger = log.NewDiscardLogger()
	}

	return Server{
		logger:   logger,
		registry: newRegistry(),
	}
}

// SetCommand registers the provided Handler to be called for
// the specified command ID.
//
// If there are multiple potential handlers for a command
// registered (using SetCommand, SetModel, or SetModelCommand)
// the server will attempt to use the most specific handler
// available, with precedence shown below:
//  1. Model+Command ID handler (most specific)
//  2. Command ID handler
//  3. Model ID handler (least specific)
func (svr *Server) SetCommand(cmdID string, h Handler) {
	svr.registry.setCmd(cmdID, h)
}

// SetModel registers the provided Handler to be called for
// the specified model ID.
//
// If there are multiple potential handlers for a command
// registered (using SetCommand, SetModel, or SetModelCommand)
// the server will attempt to use the most specific handler
// available, with precedence shown below:
//  1. Model+Command ID handler (most specific)
//  2. Command ID handler
//  3. Model ID handler (least specific)
func (svr *Server) SetModel(modelID string, h Handler) {
	svr.registry.setModel(modelID, h)
}

// SetModelCommand registers the provided Handler to be called
// for the given model and command ID combination.
//
// If there are multiple potential handlers for a command
// registered (using SetCommand, SetModel, or SetModelCommand)
// the server will attempt to use the most specific handler
// available, with precedence shown below:
//  1. Model+Command ID handler (most specific)
//  2. Command ID handler
//  3. Model ID handler (least specific)
func (svr *Server) SetModelCommand(modelID, cmdID string, h Handler) {
	svr.registry.setModelCmd(modelID, cmdID, h)
}

// Run starts the http server and listens at the given address
// (e.g., ":8072", "localhost:1515", "127.0.0.1:3535") until
// either an error occurs or the interrupt signal is received.
func (svr *Server) Run(address string) error {
	// Create the tcp connection
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}

	// Get the address in case ":0" was used as the port number,
	// in which case a port was automatically selected.
	address = lis.Addr().(*net.TCPAddr).String()

	// Set up the http server.
	hsvr := &http.Server{
		Addr:    address,
		Handler: svr,
	}

	// Use an error channel to collect errors from the go
	// routine that listens on the port.
	errCh := make(chan error, 1)

	// Listen in a different go routine so that we can still
	// respond to the keyboard interrupt.
	go func() {
		errCh <- hsvr.Serve(lis)
	}()
	svr.logger.Info(
		"msg", "server started",
		"httpAddr", address,
	)

	// Catch the interrupt signal to gracefully shutdown the server
	const maxInterrupts = 10
	interrupt := make(chan os.Signal, maxInterrupts)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	// Wait for an error or an interrupt
	select {
	case err = <-errCh:
		return err

	case <-interrupt:
		svr.logger.Info("msg", "shutting down http server...")

		// Gracefully shut down the server
		const timeout = 10 * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		return hsvr.Shutdown(ctx)
	}
}

// ServeHTTP implements the http.Handler interface. It decodes
// the command, forwards the data to the correct command Handler,
// then encodes the result to send back to Diatheke.
func (svr *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Read the JSON request
	decoder := json.NewDecoder(r.Body)
	var input Input
	if err := decoder.Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Find the appropriate handler
	handler, found := svr.registry.findHandler(input)
	if !found || handler == nil {
		http.Error(
			w,
			fmt.Sprintf("could not find handler for command %q", input.CommandID),
			http.StatusInternalServerError,
		)
		svr.logger.Error(
			"msg", "could not find command handler",
			"cmd", input.CommandID,
		)
		return
	}

	// Run the command
	output := Output{
		CommandID:  input.CommandID,
		Parameters: make(Params),
		Metadata:   input.Metadata,
	}
	err := handler(input, &output)
	if err != nil {
		output.Error = err.Error()
	}

	// Send the command result
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	if err = encoder.Encode(&output); err != nil {
		svr.logger.Error(
			"msg", "failed to write command response",
			"error", err,
		)
	}
}

type cmdModelPair struct {
	cmdID   string
	modelID string
}

type handlerRegistry struct {
	cmdModelFuncs map[cmdModelPair]Handler
	cmdFuncs      map[string]Handler
	modelFuncs    map[string]Handler
}

func newRegistry() handlerRegistry {
	return handlerRegistry{
		cmdModelFuncs: make(map[cmdModelPair]Handler),
		cmdFuncs:      make(map[string]Handler),
		modelFuncs:    make(map[string]Handler),
	}
}

func (hr *handlerRegistry) setCmd(cmdID string, h Handler) {
	hr.cmdFuncs[cmdID] = h
}

func (hr *handlerRegistry) setModel(modelID string, h Handler) {
	hr.modelFuncs[modelID] = h
}

func (hr *handlerRegistry) setModelCmd(modelID, cmdID string, h Handler) {
	pair := cmdModelPair{
		modelID: modelID,
		cmdID:   cmdID,
	}

	hr.cmdModelFuncs[pair] = h
}

func (hr *handlerRegistry) findHandler(in Input) (Handler, bool) {
	// Check our maps from specific to general.
	pair := cmdModelPair{
		cmdID:   in.CommandID,
		modelID: in.ModelID,
	}
	handler, found := hr.cmdModelFuncs[pair]
	if found {
		return handler, true
	}

	handler, found = hr.cmdFuncs[in.CommandID]
	if found {
		return handler, true
	}

	handler, found = hr.modelFuncs[in.ModelID]
	return handler, found
}
