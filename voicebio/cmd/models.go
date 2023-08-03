// Copyright (2023 -- present) Cobalt Speech and Language Inc.

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

	"github.com/cobaltspeech/examples-go/voicebio/internal/client"
	"github.com/spf13/cobra"
)

var listModelsCmd = &cobra.Command{
	Use:   "listmodels",
	Short: "List models available in voicebio server.",
	Args:  addGlobalFlagsCheck(cobra.NoArgs),
	Run: runClientFunc(func(ctx context.Context, c *client.Client, args []string) error {

		err := listModels(ctx, c)

		return err
	}),
}

func listModels(ctx context.Context, c *client.Client) error {

	v, err := c.ListModels(ctx)
	if err != nil {
		return fmt.Errorf("error while getting the list of models: %w", err)
	}

	fmt.Printf("%s\n", v)

	return nil
}
