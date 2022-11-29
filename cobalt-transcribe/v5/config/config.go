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

package config

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/cobaltspeech/examples-go/pkg/audio"
	"github.com/cobaltspeech/sdk-cubic/grpc/go-cubic/cubicpb"
)

// ServerConfig contains the Diatheke server settings
type ServerConfig struct {
	Address     string
	Insecure    bool
	ModelID     string
	GRPCTimeout int
}

// Config contains the application configuration
type Config struct {
	Channels    []uint32
	NumWorkers  int
	Prefix      bool
	Server      ServerConfig
	LogFilePath string
	Verbose     bool
	Extension   string
	CubicConfig *cubicpb.RecognitionConfig
	Recording   audio.Config
}

// ReadConfigFile attempts to load the given config file
func ReadConfigFile(filename string) (Config, error) {
	var config Config

	_, err := toml.DecodeFile(filename, &config)
	if err != nil {
		return config, err
	}

	if config.Server.Address == "" {
		return config, fmt.Errorf("missing server address")
	}

	if config.NumWorkers < 1 {
		return config, fmt.Errorf("NumWorkers must be greater than 0")
	}

	if config.Server.GRPCTimeout < 1 {
		// If timeout not specified, set to default
		config.Server.GRPCTimeout = 2
	}
	return config, nil
}

// CreateCubicConfig checks the value of cfg.Extension and populates
// the RecognitionConfig struct if there was no error.
// Note: there are many more options available to control the
// Cubic output. This example illustrates a simple case.  Please see
// https://cobaltspeech.github.io/sdk-cubic/protobuf/autogen-doc-cubic-proto/#message-recognitionconfig
// for description of other available options.
func CreateCubicConfig(cfg Config) (*cubicpb.RecognitionConfig, error) {
	var audioEncoding cubicpb.RecognitionConfig_Encoding
	ext := strings.ToLower(cfg.Extension)
	switch ext {
	case ".wav":
		audioEncoding = cubicpb.RecognitionConfig_WAV
	case ".flac":
		audioEncoding = cubicpb.RecognitionConfig_FLAC
	case ".mp3":
		audioEncoding = cubicpb.RecognitionConfig_MP3
	case ".vox":
		audioEncoding = cubicpb.RecognitionConfig_ULAW8000
	case ".raw":
		audioEncoding = cubicpb.RecognitionConfig_RAW_LINEAR16
	default:
		return nil, fmt.Errorf("unsupported file extension %s", ext)
	}

	return &cubicpb.RecognitionConfig{
		ModelId:                cfg.Server.ModelID,
		AudioEncoding:          audioEncoding,
		AudioChannels:          cfg.Channels,
		EnableConfusionNetwork: true,
		EnableRawTranscript:    true,
		EnableWordTimeOffsets:  true,
	}, nil
}
