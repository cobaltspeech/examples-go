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
	"context"
	"fmt"

	"github.com/cobaltspeech/examples-go/transcribe/transcribe-client/internal/client"

	"github.com/spf13/cobra"
)

var listModelsCmd = &cobra.Command{
	Use:   "list",
	Short: "List models available in Transcribe server.",
	Long:  "List out the information about the models Transcribe server can access.",
	Run: func(cmd *cobra.Command, args []string) {
		var opts []client.Option

		if isInsecure {
			opts = append(opts, client.WithInsecure())
		}

		c, err := client.NewClient(serverAddress, opts...)
		if err != nil {
			cmd.PrintErrf("error: failed to create a client: %v\n", err)

			return
		}

		defer c.Close()

		if err := listModels(context.Background(), c); err != nil {
			cmd.PrintErrf("error: %v\n", err)

			return
		}
	},
}

func listModels(ctx context.Context, c *client.Client) error {
	models, err := c.ListModels(ctx)
	if err != nil {
		return fmt.Errorf("failed to list models: %w", err)
	}

	fmt.Printf("Available Models:\n")

	for _, mdl := range models {
		fmt.Printf("    ID: %q\n", mdl.Id)
		fmt.Printf("    Name: %q\n", mdl.Name)
		fmt.Printf("    Attributes: %v\n\n", mdl.Attributes)
	}

	return nil
}
