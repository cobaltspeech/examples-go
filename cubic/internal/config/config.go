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

	"github.com/BurntSushi/toml"
)

// ServerConfig contains the Diatheke server settings
type ServerConfig struct {
	Address  string
	Insecure bool
	ModelID  string
}

// Config contains the application configuration
type Config struct {
	Channels          []uint32
	NumWorkers        int
	TimestampInterval int
	Server            ServerConfig
	LogFilePath       string
	Verbose           bool
	Extension         string
	IdleTimeout       int64
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
	return config, nil
}
