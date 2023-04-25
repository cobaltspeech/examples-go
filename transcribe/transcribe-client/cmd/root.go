// Copyright (2023 -- present) Cobalt Speech and Language, Inc.

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

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// configuration struct to hold global flags
var (
	serverAddress string // address is the GRPC address of Transcribe server.
	isInsecure    bool   // isInsecure is a flag specify insecure connection to the server.
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "transcribe-client",
	Short: "transcribe-client is a command line interface for interacting with a running instance of transcribe-server.",
	Long:  `transcribe-client is a command line interface for interacting with a running instance of transcribe-server.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(buildTransribeCmd())
	rootCmd.AddCommand(listModelsCmd)

	// Add the global flags.
	rootCmd.PersistentFlags().StringVarP(&serverAddress, "server", "s", "127.0.0.1:2727", "Transcribe-server GRPC address.")
	rootCmd.PersistentFlags().BoolVar(&isInsecure, "insecure", false,
		"If flag provided, TLS will not be used when establishing a connection to the server")
}
