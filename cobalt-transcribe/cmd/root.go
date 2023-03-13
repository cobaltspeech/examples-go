// Copyright (2019) Cobalt Speech and Language Inc.

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
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type clientConf struct {
	Server Server
}

type Server struct {
	Address     string        `toml:"Address"`     // URL of the server as specified by the command line arguments.
	ModelID     string        `toml:"ModelID"`     // ID of the ASR model
	Insecure    bool          `toml:"Insecure"`    // Whether to use insecure connection
	IdleTimeout time.Duration `toml:"IdleTimeout"` // Time to wait for a response from the server.
}

// configuration struct to hold global flags
var (
	cConf  clientConf
	confFn string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "transcribe",
	Short: "transcribe is a command line interface for interacting with a running instance of cubicsvr",
	Long:  `transcribe is a command line interface for interacting with a running instance of cubicsvr`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(buildTransribeCmd())
	// TODO (cenk): add modelscmd
	// rootCmd.AddCommand(modelsCmd)

	// Add the global flags.
	addGlobalFlags(rootCmd.PersistentFlags())
}

// simplifyGrpcErrors converts semi-cryptic gRPC errors into more user-friendly errors.
func simplifyGrpcErrors(err error) error {
	// TODO create more robust/consistent ways of checking for each error.  This is a little too ad-hoc.
	switch {
	case strings.Contains(err.Error(), "transport: Error while dialing dial tcp"):
		return fmt.Errorf("unable to reach server at the address '%s'", cConf.Server.Address)

	case strings.Contains(err.Error(), "authentication handshake failed: tls:"):
		return fmt.Errorf(" '--insecure' required for this connection")

	case strings.Contains(err.Error(), "desc = all SubConns are in TransientFailure, latest connection error: "):
		return fmt.Errorf(" '--insecure' must not be used for this connection")

	default:
		return fmt.Errorf(err.Error()) // return the grpc error directly
	}

}
