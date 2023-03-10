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
	"context"
	"fmt"

	"github.com/cobaltspeech/examples-go/cobalt-transcribe/internal/client"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Fetch version of cubicsvr.",
	Args:  addGlobalFlagsCheck(cobra.NoArgs),
	Run: runClientFunc(func(c *client.Client, args []string) error {
		err := version(c)

		return err
	}),
}

func version(c *client.Client) error {
	v, err := c.CobaltVersions(context.Background())
	if err != nil {
		return fmt.Errorf("error while getting version: %w", err)
	}

	fmt.Printf("Cubic engine version is %s\n", v)

	return nil
}
