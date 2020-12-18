// Copyright (2020) Cobalt Speech and Language Inc.

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

// Package audio provides audio I/O to an application by piping the
// audio data to an external application over stdin or stdout.
package audio

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Config contains the information to run an external
// application for the audio I/O.
type Config struct {
	Application string
	Args        string
}

// ArgList returns the arguments as a list of strings
func (ac *Config) ArgList() []string {
	return strings.Fields(ac.Args)
}

// Recorder launches an external application to handle recording audio.
type Recorder struct {
	// Internal data
	appConfig Config
	cmd       *exec.Cmd
	ctx       context.Context
	cancel    context.CancelFunc
	stdout    io.ReadCloser
}

// NewRecorder returns a new recorder object based the given configuration.
func NewRecorder(cfg Config) Recorder {
	return Recorder{
		appConfig: cfg,
	}
}

// Start the external recording application.
func (rec *Recorder) Start() error {
	if rec.cancel != nil {
		// Ignore if we are already recording
		return nil
	}

	// Create the command context so we can cancel it in the stop function.
	// This is how we can kill the external application.
	ctx, cancel := context.WithCancel(context.Background())

	// Create the record command and get its stdout pipe
	cmd := exec.CommandContext(ctx,
		rec.appConfig.Application,
		rec.appConfig.ArgList()...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return err
	}

	// Run the application
	if err = cmd.Start(); err != nil {
		cancel()
		return err
	}

	// Save the command
	rec.cmd = cmd
	rec.ctx = ctx
	rec.cancel = cancel
	rec.stdout = stdout

	return nil
}

// Stop the external recording application.
func (rec *Recorder) Stop() {
	if rec.cancel == nil || rec.cmd == nil {
		// Ignore if it is already stopped.
		return
	}

	// By the time we exit this function, we want everything to be reset
	defer func() {
		rec.ctx = nil
		rec.cancel = nil
		rec.cmd = nil
		rec.stdout = nil
	}()

	// Cancel the context, which should kill the executable. Then wait
	// for it to finish.
	rec.cancel()
	rec.cmd.Wait() //nolint: errcheck // Error is likely from being killed.
}

// Output returns an io.Reader that reads audio from the application.
// Should be called after Start() has been called.
func (rec *Recorder) Output() io.Reader {
	return rec.stdout
}

// Read audio data from the external recording application and put it into p.
func (rec *Recorder) Read(p []byte) (n int, err error) {
	if rec.stdout == nil {
		// It is an error to call this if the recorder isn't running.
		return 0, fmt.Errorf("recorder application is not running")
	}

	// Grab data from stdout.
	return rec.stdout.Read(p)
}

// Player represents the external playback executable
// with args.
type Player struct {
	appConfig Config
	cmd       *exec.Cmd
	stdin     io.WriteCloser
}

// NewPlayer creates a new player object based on the
// given config.
func NewPlayer(cfg Config) Player {
	return Player{
		appConfig: cfg,
	}
}

// Start the external playback application.
func (p *Player) Start() error {
	// Ignore if it is already running
	if p.cmd != nil {
		return nil
	}

	// Setup the command and get its stdin pipe
	cmd := exec.Command(p.appConfig.Application, p.appConfig.ArgList()...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	// Run the application
	if err := cmd.Start(); err != nil {
		return err
	}

	// Save the command
	p.cmd = cmd
	p.stdin = stdin

	return nil
}

// Stop the external playback application.
func (p *Player) Stop() error {
	// Ignore if it is not running
	if p.cmd == nil {
		return nil
	}

	// By the time we exit this function, we want everything to be reset
	defer func() {
		p.cmd = nil
		p.stdin = nil
	}()

	// Close the stdin pipe (which should also close the application)
	// and wait for the app to complete
	p.stdin.Close()
	if err := p.cmd.Wait(); err != nil {
		return err
	}

	return nil
}

// PushAudio data to the player app. Start() should
// be called prior to using this function.
func (p *Player) PushAudio(audio []byte) error {
	if p.stdin == nil {
		return fmt.Errorf("player application is not running")
	}

	// Write the audio data to stdin
	return binary.Write(p.stdin, binary.LittleEndian, audio)
}

// Input returns an io.Writer that TTS audio can be pushed to.
func (p *Player) Input() io.Writer {
	return p.stdin
}
