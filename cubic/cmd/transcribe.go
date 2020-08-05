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

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cobaltspeech/examples-go/cubic/internal/config"
	"github.com/cobaltspeech/log"
	"github.com/cobaltspeech/log/pkg/level"
	cubic "github.com/cobaltspeech/sdk-cubic/grpc/go-cubic"
	"github.com/cobaltspeech/sdk-cubic/grpc/go-cubic/cubicpb"
	pbduration "google.golang.org/protobuf/types/known/durationpb"
)

type inputs struct {
	filepath   string
	outputPath string
}

var longMsg = `
This command is used for transcribing audio files.
It will iterate through the specified directory of audio files and write the transcript
back either to the same directory or --output directory.  The file name for the transcript
will be the same name as the input audio file, with the extension .txt.

If the server supports transcoding, the file extension (wav, flac, mp3, vox, raw (PCM16SLE)) 
will be used to determine which codec to use.  Use WAV or FLAC for best results.

Usage: transcribe -config sample.config.toml -input /path/to/audio/files -output /path/where/transcripts/will/be/written
`

func createClient(cfg config.Config) (*cubic.Client, error) {
	var client *cubic.Client
	var err error

	dur, err := time.ParseDuration("10s")
	if err != nil {
		return nil, fmt.Errorf("error creating client for server '%s': %v", cfg.Server.Address, err)
	}

	if cfg.Server.Insecure {
		client, err = cubic.NewClient(cfg.Server.Address, cubic.WithInsecure(), cubic.WithConnectTimeout(dur))
	} else {
		client, err = cubic.NewClient(cfg.Server.Address)
	}

	if err != nil {
		return nil, fmt.Errorf("error creating client for server '%s': %v", cfg.Server.Address, err)
	}

	return client, nil
}

func main() {
	logger := log.NewLeveledLogger()
	configFile := flag.String("config", "", "path to config file")
	inputDir := flag.String("input", "", "path to folder containing audio files")
	outputDir := flag.String("output", "", "optional path to folder to which transcript files will be written")
	flag.Usage = func() {
		fmt.Println(longMsg)
		fmt.Println("Flags:")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *configFile == "" {
		fmt.Println("-config is required")
		return
	}
	cfg, err := config.ReadConfigFile(*configFile)
	if err != nil {
		fmt.Println("Error opening config file", err)
		return
	}

	if cfg.Verbose {
		logger.SetFilterLevel(level.Error | level.Info | level.Debug)
	}

	cubicConfig, err := config.CreateCubicConfig(cfg)
	if err != nil {
		fmt.Println("Error parsing config file", err)
		return
	}
	cfg.CubicConfig = cubicConfig
	logger.Info("CubicConfig", cfg.CubicConfig)

	// Set up a cubicsvr client
	client, err := createClient(cfg)
	if err != nil {
		logger.Error("err", err)
		return
	}
	defer client.Close()

	// Load the files and place them in a channel
	files, err := loadFiles(*inputDir, *outputDir, cfg.Extension, logger)
	if err != nil {
		logger.Error("msg", "Error loading files", "err", err)
		return
	}
	fileCount := len(files)
	var numWorkers int
	if fileCount < cfg.NumWorkers {
		numWorkers = fileCount
	} else {
		numWorkers = cfg.NumWorkers
	}
	logger.Info("msg", "Processing files", "server", cfg.Server.Address, "fileCount", fileCount, "numWorkers", numWorkers)
	// Setup channel for communicating between the various goroutines
	fileChannel := make(chan inputs, numWorkers)

	// Start multiple goroutines.  The first pushes to the fileChannel, and the rest
	// each pull from the fileChannel and send requests to cubic server.
	wg := &sync.WaitGroup{}
	wg.Add(numWorkers + 1)
	go feedInputFiles(fileChannel, files, wg, logger)

	logger.Debug("msg", "Starting workers.", "numWorkers", numWorkers)
	for i := 0; i < numWorkers; i++ {
		go transcribeFiles(i, cfg, wg, client, fileChannel, logger)
	}

	wg.Wait() // Wait for all workers to finish
}

// getOutputWriter returns a file writer for the given path
func getOutputWriter(outputPath string) (io.WriteCloser, error) {
	path := fmt.Sprintf("%s.txt", outputPath)

	// Create the file
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("Failed to create output file: %v", err)
	}
	return file, nil
}

func checkDir(dir, desc string) error {
	fi, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("error opening %s dir %s: %v", desc, dir, err)
	}
	if !fi.Mode().IsDir() {
		return fmt.Errorf("%s dir %s is not a directory", desc, dir)
	}
	return nil
}

// loadFiles walks through all the files in inputDir that end in extension and adds them to a list for processing
func loadFiles(inputDir, outputDir, extension string, logger log.Logger) ([]inputs, error) {
	if err := checkDir(inputDir, "input"); err != nil {
		return nil, err
	}
	if outputDir == "" {
		outputDir = inputDir
	} else if err := checkDir(outputDir, "output"); err != nil {
		return nil, err
	}
	files := make([]inputs, 0)
	filepath.Walk(inputDir, func(path string, info os.FileInfo, err error) error {
		// files, outputDir, and extension are available as closures
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.IsDir() || filepath.Ext(path) != extension {
			return nil
		}

		outputPath := filepath.Join(outputDir, filepath.Base(path))
		files = append(files, inputs{
			filepath:   path,
			outputPath: outputPath,
		})
		return nil
	})
	return files, nil
}

func feedInputFiles(fileChannel chan<- inputs, files []inputs, wg *sync.WaitGroup, logger log.Logger) {
	for _, f := range files {
		fileChannel <- f
	}
	logger.Info("msg", "Done feeding audio files.")
	wg.Done()
	close(fileChannel)
}

func transcribeFiles(workerID int, cfg config.Config, wg *sync.WaitGroup, client *cubic.Client,
	fileChannel <-chan inputs, logger log.Logger) {
	logger.Debug("Worker starting", workerID)
	for input := range fileChannel {
		transcribeFile(input, workerID, cfg, client, logger)
	}
	wg.Done()
}

func transcribeFile(input inputs, workerID int, cfg config.Config, client *cubic.Client, logger log.Logger) {
	audio, err := os.Open(input.filepath)
	defer audio.Close()
	if err != nil {
		logger.Error("file", input.filepath, "err", err, "message", "Couldn't open audio file")
		return
	}

	w, err := getOutputWriter(input.outputPath)
	if err != nil {
		logger.Error("file", input.outputPath, "err", err, "message", "Couldn't open output file writer")
	}

	// Counter for segments
	segmentID := 0

	// Send the Streaming Recognize config
	err = client.StreamingRecognize(context.Background(),
		cfg.CubicConfig,
		audio, // The audio file to send
		func(response *cubicpb.RecognitionResponse) { // The callback for results
			logger.Debug("workerID", workerID, "file", input.filepath, "segmentID", segmentID)
			for _, r := range response.Results {
				if !r.IsPartial && len(r.Alternatives) > 0 {
					prefix := ""
					if cfg.Prefix {
						prefix = fmt.Sprintf("[Channel %d - %s]", r.AudioChannel, formatDuration(r.Alternatives[0].GetStartTime()))
					}

					fmt.Fprintln(w, prefix, r.Alternatives[0].Transcript)
				}
			}
			segmentID++
		})

	if err != nil {
		logger.Error("file", input.filepath, "err", simplifyGrpcErrors(cfg, err))
	}
}

// asDuration converts a pbduration.Duration to a time.Duration
// so it is more nicely formatted. Don't worry about overflow since
// it's unlikely that the timestamp in a file would be more than 200 years!
func formatDuration(x *pbduration.Duration) string {
	d := time.Duration(x.GetSeconds()) * time.Second
	d += time.Duration(x.GetNanos()) * time.Nanosecond
	return d.String()
}

// simplifyGrpcErrors converts semi-cryptic gRPC errors into more user-friendly errors.
// Not meant to be production error handling.
func simplifyGrpcErrors(cfg config.Config, err error) error {
	switch {
	case strings.Contains(err.Error(), "transport: Error while dialing dial tcp"):
		return fmt.Errorf("unable to reach server at address '%s'", cfg.Server.Address)

	case strings.Contains(err.Error(), "authentication handshake failed: tls:"):
		return fmt.Errorf("'Insecure = true' required for this connection")

	case strings.Contains(err.Error(), "desc = all SubConns are in TransientFailure, latest connection error: "):
		return fmt.Errorf("'Insecure = true' must not be used for this connection")

	case strings.Contains(err.Error(), "invalid model requested"):
		return fmt.Errorf("invalid ModelID '%s' (%v)", cfg.Server.ModelID, err)

	case strings.Contains(err.Error(), "audio transcoding has stopped"):
		return fmt.Errorf("check file format and channel information")

	default:
		return err // return the grpc error directly
	}
}
