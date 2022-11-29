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
	"strings"

	"github.com/cobaltspeech/examples-go/cobalt-transcribe/internal/config"

	"github.com/cobaltspeech/examples-go/pkg/audio"
	"github.com/cobaltspeech/log"
	"github.com/cobaltspeech/log/pkg/level"
	cubic "github.com/cobaltspeech/sdk-cubic/grpc/go-cubic"
	"github.com/cobaltspeech/sdk-cubic/grpc/go-cubic/cubicpb"
)

type fileRef struct {
	audioPath  string
	outputPath string
}

var longMsg = `
This command is used for transcribing input from the microphone

Usage: transcribe -config sample.config.toml -input /path/to/audio/files -output /path/where/transcripts/will/be/written
`

func main() {
	logger := log.NewLeveledLogger()
	configFile := flag.String("config", "", "path to config file")
	output := flag.String("output", "", "filepath to which transcript file will be appended")
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
		fmt.Printf("Error in config file %s: %v\n", *configFile, err)
		return
	}

	// By default, Error and Info messages are logged
	if cfg.Verbose {
		logger.SetFilterLevel(level.Error | level.Info | level.Debug)
	}

	cubicConfig, err := config.CreateCubicConfig(cfg)
	if err != nil {
		fmt.Printf("Error in config file %s: %v\n", *configFile, err)
		return
	}
	cfg.CubicConfig = cubicConfig
	logger.Info("CubicConfig", cfg.CubicConfig)

	if *output == "" {
		fmt.Printf("Please specify -output\n")
		return
	}
	w, err := getOutputWriter(*output)
	if err != nil {
		logger.Error("msg", "error getting output writer", "err", err)
		return
	}

	// Set up a cubicsvr client
	client, err := createClient(cfg)
	if err != nil {
		logger.Error("err", err)
		return
	}
	defer client.Close()
	modelList, err := client.ListModels(context.Background())
	if err != nil {
		logger.Error("msg", "error listing models", "err", err)
		return
	}
	fmt.Printf("Available Models:\n")
	for _, mdl := range modelList.Models {
		fmt.Printf("  ID: %v\n", mdl.Id)
		fmt.Printf("    Name: %v\n", mdl.Name)
	}

	// Create something to handle recording audio
	recorder := audio.NewRecorder(cfg.Recording)
	if err = recorder.Start(); err != nil {
		fmt.Println(err)
		return
	}
	defer recorder.Stop()
	fmt.Println("recording...")
	for {
		// Run until Ctrl-C

		err = client.StreamingRecognize(context.Background(),
			cfg.CubicConfig,
			recorder.Output(), // The output stream
			func(response *cubicpb.RecognitionResponse) { // The callback for results
				for _, r := range response.Results {
					// Note: The Results object includes a lot of detail about the ASR output.
					// For simplicity, this example just uses a few of the available properties.
					// See https://cobaltspeech.github.io/sdk-cubic/protobuf/autogen-doc-cubic-proto/#message-recognitionalternative
					// for a description of what other information is available.
					if len(r.Alternatives) > 0 {
						line := r.Alternatives[0].Transcript
						fmt.Printf("\r%s", line)
						if !r.IsPartial {
							fmt.Println()
							fmt.Fprintln(w, line)
						}
					}
				}
			})
	}

}

// createClient instantiates the Client from the Cubic SDK to communicate with the server
// specified in the config file
func createClient(cfg config.Config) (*cubic.Client, error) {
	var client *cubic.Client
	var err error

	if cfg.Server.Insecure {
		client, err = cubic.NewClient(cfg.Server.Address, cubic.WithInsecure())
	} else {
		client, err = cubic.NewClient(cfg.Server.Address)
	}

	if err != nil {
		return nil, simplifyGrpcErrors(cfg, err)
	}

	return client, nil
}

// getOutputWriter returns a file writer for the given path
func getOutputWriter(outputPath string) (io.WriteCloser, error) {
	// Create the file
	file, err := os.Create(outputPath)
	if err != nil {
		return nil, fmt.Errorf("Failed to create output file: %v", err)
	}
	return file, nil
}

// checkDir validates that the specified directory path exists and is a directory
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

// simplifyGrpcErrors converts semi-cryptic gRPC errors into more user-friendly errors.
// Not meant to be production error handling.
func simplifyGrpcErrors(cfg config.Config, err error) error {
	switch {
	case strings.Contains(err.Error(), "context deadline exceeded"):
		return fmt.Errorf("timeout trying to reach server at '%s'", cfg.Server.Address)

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
