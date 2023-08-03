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

func buildVoicebioVerifyCmd() *cobra.Command {
	var output string

	var cmd = &cobra.Command{
		Use:   "verify <audio file> --voiceprint <voiceprint>",
		Short: "Verify user with an audio file and a voiceprint.",
		Args:  addGlobalFlagsCheck(cobra.ExactArgs(1)),
		Run: runClientFunc(func(ctx context.Context, c *client.Client, args []string) error {

			logger := log.NewLeveledLogger()
			// args[0] is the audio file
			err := verify(ctx, logger, c, args[0], output)
			if err != nil {
				return err
			}

			return nil
		}),
	}

	cmd.Flags().StringVar(&output, "output-json", "", "Path to output containing results of verification. If not specified, writes formatted hypothesis to STDOUT.")
	cmd.Flags().StringVar(&verifyConfigStr, "verification-config", "{}", "Json string to configure verification.")
	cmd.Flags().StringVar(&voiceprint, "voiceprint", "", "Path to json file where the voiceprint is stored.")
	cmd.MarkFlagRequired("voiceprint")

	return cmd
}

func verify(ctx context.Context, logger log.Logger, c *client.Client, wavFn, outputFn string) error {
	var err error

	audio, err := os.Open(wavFn)
	if err != nil {
		return fmt.Errorf("cannot open audio file %w", err)
	}
	defer audio.Close()

	// read the enrollConfig from the config string
	var verifyConfig voicebiopb.VerificationConfig
	var verifyVoiceprint voicebiopb.Voiceprint

	decoder := json.NewDecoder(strings.NewReader(verifyConfigStr))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&verifyConfig)
	if err != nil {
		return fmt.Errorf("error decoding the verification config %w", err)
	}

	// read the voiceprint file and unmarshal into Voiceprint object
	vp, err := os.ReadFile(voiceprint)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return err
	}
	verifyVoiceprint.Data = string(vp)

	verifyConfig.Voiceprint = &verifyVoiceprint

	// use the default (first available) model if model is not specified
	if verifyConfig.ModelId == "" {
		v, err := c.ListModels(ctx)
		if err != nil {
			return fmt.Errorf("error while getting the list of models: %w", err)
		}

		verifyConfig.ModelId = v[0].Id
	}

	resp, err := c.StreamingVerify(ctx, verifyConfig, audio) //nolint:govet // it is okay to copy

	if err != nil {
		return fmt.Errorf("error during verification %s %w", wavFn, simplifyGrpcErrors(err))
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
