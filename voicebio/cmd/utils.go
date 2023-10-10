// Copyright (2023 -- present) Cobalt Speech and Language, Inc.

package cmd

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cobaltspeech/examples-go/voicebio/internal/client"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

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

		return nil
	}
}

func addGlobalFlags(flags *pflag.FlagSet) {
	// read configuration variables
	flags.StringVar(&serverAddress, "server", "127.0.0.1:2727", "address of the voicebio GRPC server.")
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
func runClientFunc(f func(context.Context, *client.Client, []string) error) func(*cobra.Command, []string) {
	return runFunc(func(args []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		conn, err := grpc.DialContext(ctx, serverAddress, grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(), grpc.WithReturnConnectionError(), grpc.FailOnNonTempDialError(true))
		if err != nil {
			log.Fatal("unable to create a client connection: ", err)
		}

		c, err := client.NewClient(conn)
		if err != nil {
			log.Fatal("unable to create a client: ", err)
		}
		defer c.Close()

		return f(ctx, c, args)
	})
}
