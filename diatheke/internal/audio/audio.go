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
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
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

// StoppableReader wraps an existing Reader that can be "stopped"
// with a function call where the next Read() will return EOF, but future
// reads will be sucessful. This reader can also be "rewound" to a point in
// the past that allows some old data to be re-read before any new data is
// read. The Rewind() functionality has a limitation that Rewind() cannot
// go back more than the maximum buffer size and Rewind() cannot be called
// two times without Reset() being called in-between.
// The maximum buffer size parameter is not a hard limit, but if the
// buffer grows past maxBufferSize*bufferSizeFactor then it will be pruned
// down to a size of maxBufferSize the oldest bytes will be removed from
// the buffer.
type StoppableReader struct {
	mu                 sync.Mutex
	origReader         io.Reader    // the original reader
	appendReader       io.Reader    // the wrapped reader (buffers what is read)
	currentReader      io.Reader    // the current reader, may include buffered bytes if Rewind() was called
	buffer             bytes.Buffer // a buffer of previously read bytes
	bufferStartOffset  int          // "actual" index at the start of the buffer
	maxBufferSize      int          // number of bytes stored in the buffer before it may be trimmed
	bufferSizeFactor   float32      // buffer will be pruned if it grows to maxaBufferSize*bufferSizeFactor
	pauseRead          bool         // set to true if the next Read() should return EOF
	rewindWithoutReset bool         // Check to make sure Rewind() is not called twice without a Reset() (buffer is modified by reading a rewound reader)
}

// NewStoppableReader creates a new stoppable reader that wraps the
// provided reader and sets the specified maximum buffer size.
func NewStoppableReader(reader io.Reader, maxBufferSize int) *StoppableReader {
	var sr StoppableReader
	sr.origReader = reader
	sr.appendReader = io.TeeReader(reader, &sr.buffer)
	sr.currentReader = sr.appendReader
	sr.bufferStartOffset = 0
	sr.maxBufferSize = maxBufferSize
	sr.bufferSizeFactor = 2.0
	sr.pauseRead = false
	sr.rewindWithoutReset = false
	return &sr
}

// Read bytes from the wrapped Reader and append the bytes to the buffer.
func (sr *StoppableReader) Read(p []byte) (n int, err error) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	if sr.pauseRead {
		sr.pauseRead = false // let the next Read() call return data
		return 0, io.EOF
	}

	// Shrink the buffer if is too large
	if sr.buffer.Len() > int(sr.bufferSizeFactor)*sr.maxBufferSize {
		origLen := sr.buffer.Len()
		sr.buffer = *bytes.NewBuffer(sr.buffer.Bytes()[sr.buffer.Len()-sr.maxBufferSize:])
		sr.bufferStartOffset += origLen - sr.maxBufferSize
	}
	return sr.currentReader.Read(p)
}

// Stop forces the next Read() of the StoppableReader to return EOF, but
// additional Read() operations will continue to read bytes as usual.
func (sr *StoppableReader) Stop() {
	sr.mu.Lock()
	sr.pauseRead = true
	sr.mu.Unlock()
}

// Rewind returns a new Reader that starts at the specified byte offset.
// This may allow data that had been previously read to be re-read.
// If the StoppableAudioReader has a maximum buffer size and data
// was not fully retained for the specified offset an error will be
// returned, but a reader with as much buffered context as possible
// will still be returned (what to do in this situation is up to
// the caller).
// if resetTimeZero is true, then future "offset" values sent to Rewing()
// will consider the start of the last rewound stream to be zero, otherwise
// the "zero time" will remain the start of the original input stream.
func (sr *StoppableReader) Rewind(offset int, resetTimeZero bool) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	var err error
	if sr.rewindWithoutReset {
		// This is only occasionally an error (depending on how it is used). The caller may choose
		// to ignore the error, but it is discouraged.
		err = fmt.Errorf("Rewind() should not be called twice without a Reset() between them because reading a rewound buffer may modify the buffered data")
	} else {
		sr.rewindWithoutReset = true
	}

	//adjustedOffset := offset
	var adjustedOffset int
	if offset < sr.bufferStartOffset {
		// Offset value is before the start of the buffer, data returned will be incomplete.
		adjustedOffset = 0
		err = fmt.Errorf("StoppableAudioReader::Rewind() error, the requested offset %d is smaller that the start of the buffer: %d",
			offset, sr.bufferStartOffset)
	} else if offset > (sr.bufferStartOffset + sr.buffer.Len()) {
		// Offset value is after the end of the buffer (impossible to buffer future data).
		err = fmt.Errorf("StoppableAudioReader::Rewind() error, the requested offset %d is larger that the end of the buffer (in the future!): %d",
			offset, sr.bufferStartOffset+sr.buffer.Len())
		sr.currentReader = sr.appendReader
		return err
	} else {
		// The requested offset is in the current buffer.
		adjustedOffset = offset - sr.bufferStartOffset
	}

	// Return a MultiReader that will first read bytes from a (selected) copy of the buffer,
	// and will read from the wrapped Reader afterwards (and will continue to add to the
	// buffer when reading new data).
	partialBuffer := bytes.NewReader(sr.buffer.Bytes()[adjustedOffset:])
	sr.currentReader = io.MultiReader(partialBuffer, sr.appendReader)

	// If the flag is set to consider the sbuftart of the rewound reader to be the new time
	// zero, adjust the buffer start offset.  This will be a value <=0 because the data
	// at the start of the buffer will be *before* the new time zero.
	if resetTimeZero {
		sr.bufferStartOffset = -(adjustedOffset + sr.buffer.Len() - adjustedOffset)
	}

	return err
}

// Reset clears the buffered data.
// It is recommended to call Reset() between calls to Rewind().
func (sr *StoppableReader) Reset() {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.buffer.Reset()
	sr.appendReader = io.TeeReader(sr.origReader, &sr.buffer)
	sr.currentReader = sr.appendReader
	sr.bufferStartOffset = 0
	sr.pauseRead = false
	sr.rewindWithoutReset = false
}
