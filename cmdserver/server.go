// Copyright (2021) Cobalt Speech and Language Inc.

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

// Handler is a function that takes a map of input parameters
// and returns a (possibly empty or nil) map of output parameters,
// as expected by a Diatheke command.
type Handler func(Params) (Params, error)

// Server represents an http server that handles Diatheke
// commands. With this server, all commands go to the same
// URL, so the Diatheke model should be set accordingly.
// Once received, commands are sent to Handlers added with
// the SetHandler function.
type Server struct {
	logger       log.Logger
	handlerFuncs map[string]Handler
}

// NewServer returns a new command server.
func NewServer(logger log.Logger) Server {
	if logger == nil {
		logger = log.NewDiscardLogger()
	}

	return Server{
		logger:       logger,
		handlerFuncs: make(map[string]Handler),
	}
}

// SetHandler registers the provided Handler to be called for
// the specified command ID.
func (svr *Server) SetHandler(cmdID string, h Handler) {
	svr.handlerFuncs[cmdID] = h
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
	var cmd cmdRequest
	if err := decoder.Decode(&cmd); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Find the appropriate handler
	handler, found := svr.handlerFuncs[cmd.ID]
	if !found || handler == nil {
		http.Error(
			w,
			fmt.Sprintf("could not find handler for command %q", cmd.ID),
			http.StatusInternalServerError,
		)
		svr.logger.Error(
			"msg", "could not find command handler",
			"cmd", cmd.ID,
		)
		return
	}

	// Run the command
	outParams, err := handler(cmd.InputParameters)
	result := cmdResponse{
		ID:            cmd.ID,
		OutParameters: outParams,
	}
	if err != nil {
		result.Error = err.Error()
	}

	// Send the command result
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	if err = encoder.Encode(&result); err != nil {
		svr.logger.Error(
			"msg", "failed to write command response",
			"error", err,
		)
	}
}

type cmdRequest struct {
	ID              string            `json:"id"`
	InputParameters map[string]string `json:"inputParameters"`
}

type cmdResponse struct {
	ID            string            `json:"id"`
	OutParameters map[string]string `json:"outParameters,omitempty"`
	Error         string            `json:"error,omitempty"`
}
