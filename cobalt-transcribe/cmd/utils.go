// Copyright (2023 -- present) Cobalt Speech and Language, Inc.

package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/cobaltspeech/examples-go/cobalt-transcribe/internal/client"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/pelletier/go-toml/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// addGlobalFlagsCheck adds validation for the global flag values and returns an error if something
// is wrong.
func addGlobalFlagsCheck(toWrap cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		err := toWrap(cmd, args)
		if err != nil {
			return err
		}

		if cConf.Server.Address == "" {
			return fmt.Errorf("'server' is empty")
		}

		return nil
	}
}

func addGlobalFlags(flags *pflag.FlagSet) {
	// read configuration variables
	flags.StringVar(&confFn, "config", "", "configuration file (.toml) for cubic server")
	flags.StringVar(&cConf.Server.Address, "server", "127.0.0.1:2729", "cubicsvr GRPC server address.")
	flags.DurationVar(&cConf.Server.IdleTimeout, "timeout",
		5000*time.Millisecond, "timeout to wait for (milliseconds)") //nolint: gomnd // 5 seconds is a fine default
}

// runFunc returns a function that serves as a Run function for a cobra command.
func runFunc(f func(args []string) error) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		if err := f(args); err != nil {
			fmt.Fprintf(os.Stderr, "fatal error: %v\n", err)
			os.Exit(1)
		}
	}
}

// runClientFunc creates a logger and client before calling f on the args. The logger has its level
// set by the verbosity from addGlobalFlags, and the client has the server URL set with the
// ServerURL from addGlobalFlags.
func runClientFunc(f func(*client.Client, []string) error) func(*cobra.Command, []string) {
	return runFunc(func(args []string) error {
		// read config file if specified
		if confFn != "" {
			inF, err := os.Open(confFn)
			if err != nil {
				fmt.Println("cannot open config file: %w", err)
				os.Exit(1)
			}

			if err := toml.NewDecoder(inF).Decode(&cConf); err != nil {
				fmt.Println("cannot decode config file: %w", err)
				os.Exit(1)
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), cConf.Server.IdleTimeout)
		defer cancel()

		conn, err := grpc.DialContext(ctx, cConf.Server.Address, grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(), grpc.WithReturnConnectionError(), grpc.FailOnNonTempDialError(true))
		if err != nil {
			log.Fatal("unable to create a client: ", err)
		}

		c, err := client.NewClient(conn)
		if err != nil {
			log.Fatal(err)
		}
		defer c.Close()

		return f(c, args)
	})
}
