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
	"encoding/json"
	"fmt"
	"os"
	"strings"

	voicebiopb "github.com/cobaltspeech/go-genproto/cobaltspeech/voicebio/v1"

	"github.com/cobaltspeech/examples-go/voicebio/internal/client"
	"github.com/cobaltspeech/log"
	"github.com/spf13/cobra"
)

func buildVoicebioIdentifyCmd() *cobra.Command {
	var output string

	var cmd = &cobra.Command{
		Use:   "identify <audio file> --voiceprintlist='<voiceprint_1,voiceprint_2,...>'",
		Short: "Identify an user with audio file compared to voiceprint list.",
		Args:  addGlobalFlagsCheck(cobra.ExactArgs(1)),
		Run: runClientFunc(func(ctx context.Context, c *client.Client, args []string) error {

			logger := log.NewLeveledLogger()

			// // args[0] is the audio file
			err := identify(ctx, logger, c, args[0], output)
			if err != nil {
				return err
			}

			return nil
		}),
	}

	cmd.Flags().StringVar(&output, "output-json", "", "Path to output json containing results of verification. If not specified, writes formatted hypothesis to STDOUT.")
	cmd.Flags().StringVar(&identifyConfigStr, "identification-config", "{}", "Json string to configure identification.")
	cmd.Flags().StringSliceVar(&voiceprintlist, "voiceprintlist", []string{}, "Paths to json files where the voiceprints are stored.")
	cmd.MarkFlagRequired("voiceprintlist")

	return cmd
}

func identify(ctx context.Context, logger log.Logger, c *client.Client, wavFn, outputFn string) error {
	var err error

	audio, err := os.Open(wavFn)
	if err != nil {
		return fmt.Errorf("cannot open audio file %w", err)
	}
	defer audio.Close()

	// read the enrollConfig from the config string
	var identifyConfig voicebiopb.IdentificationConfig
	var listVoiceprints []*voicebiopb.Voiceprint

	decoder := json.NewDecoder(strings.NewReader(identifyConfigStr))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&identifyConfig)
	if err != nil {
		return fmt.Errorf("error decoding the identification config %w", err)
	}

	// read the voiceprint list file and unmarshal into []Voiceprint object
	for _, vp := range voiceprintlist {
		vpData, err := os.ReadFile(vp)
		if err != nil {
			fmt.Println("Error reading file:", err)
			return err
		}
		listVoiceprints = append(listVoiceprints, &voicebiopb.Voiceprint{Data: string(vpData)})
	}

	identifyConfig.Voiceprints = listVoiceprints

	// use the default (first available) model if model is not specified
	if identifyConfig.ModelId == "" {
		v, err := c.ListModels(ctx)
		if err != nil {
			return fmt.Errorf("error while getting the list of models: %w", err)
		}

		identifyConfig.ModelId = v[0].Id
	}

	resp, err := c.StreamingIdentify(ctx, identifyConfig, audio) //nolint:govet // it is okay to copy

	if err != nil {
		return fmt.Errorf("error during identification %s %w", wavFn, simplifyGrpcErrors(err))
	}

	if outputFn == "" {
		fmt.Fprintln(os.Stdout, resp)
	} else {
		var enc *json.Encoder

		outF, err := os.Create(outputFn)
		if err != nil {
			return err
		}
		defer outF.Close()

		enc = json.NewEncoder(outF)
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}

	return nil
}
