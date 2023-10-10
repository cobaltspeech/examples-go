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
	"os"

	voicebiopb "github.com/cobaltspeech/go-genproto/cobaltspeech/voicebio/v1"

	"github.com/cobaltspeech/examples-go/voicebio/internal/client"
	"github.com/cobaltspeech/log"
	"github.com/spf13/cobra"
)

func buildVoicebioEnrollCmd() *cobra.Command {
	var output string

	var cmd = &cobra.Command{
		Use:   "enroll <audio file>",
		Short: "Enroll an user through an audio file.",
		Args:  addGlobalFlagsCheck(cobra.ExactArgs(1)),
		Run: runClientFunc(func(ctx context.Context, c *client.Client, args []string) error {

			logger := log.NewLeveledLogger()

			// args[0] is the audio file
			err := enroll(ctx, logger, c, args[0], output)
			if err != nil {
				return err
			}

			return nil
		}),
	}

	cmd.Flags().StringVar(&output, "output", "", "Path to output file. If not specified, writes formatted hypothesis to STDOUT and voiceprint.bin.")
	cmd.Flags().StringVar(&modelId, "model-id", "", "String to configure enrollment model id.")
	cmd.Flags().StringVar(&voiceprint, "voiceprint", "", "Path to json file where the voiceprint is stored.")

	return cmd
}

func enroll(ctx context.Context, logger log.Logger, c *client.Client, wavFn, outputFn string) error {
	var err error

	audio, err := os.Open(wavFn)
	if err != nil {
		return fmt.Errorf("cannot open audio file %w", err)
	}
	defer audio.Close()

	var enrollConfig voicebiopb.EnrollmentConfig
	// use the default (first available) model if model is not specified
	if modelId == "" {
		v, err := c.ListModels(ctx)
		if err != nil {
			return fmt.Errorf("error while getting the list of models: %w", err)
		}

		enrollConfig.ModelId = v[0].Id
	} else {
		enrollConfig.ModelId = modelId
	}

	if voiceprint != "" {
		var enrollVoiceprint voicebiopb.Voiceprint
		// read the voiceprint file and unmarshal into Voiceprint object
		vp, err := os.ReadFile(voiceprint)
		if err != nil {
			fmt.Println("Error reading file:", err)
			return err
		}
		enrollVoiceprint.Data = string(vp)

		enrollConfig.PreviousVoiceprint = &enrollVoiceprint
	}

	resp, err := c.StreamingEnroll(ctx, enrollConfig, audio) //nolint:govet // it is okay to copy
	if err != nil {
		return fmt.Errorf("error during enrollment %s %w", wavFn, simplifyGrpcErrors(err))
	}

	// Check if we have enough audio for verification/identification
	if resp.EnrollmentStatus.EnrollmentComplete != true {
		fmt.Printf("This voiceprint is not ready for verification/identification, needs extra %v seconds of audio. \n", resp.EnrollmentStatus.AdditionalAudioRequiredSeconds)
	}

	if outputFn == "" {
		fmt.Fprintln(os.Stdout, resp)
		err = os.WriteFile("voiceprint.bin", []byte(resp.Voiceprint.Data), 0666)
		if err != nil {
			fmt.Println("Error writing JSON data to file:", err)
		}
	} else {
		err = os.WriteFile(outputFn, []byte(resp.Voiceprint.Data), 0666)
		if err != nil {
			fmt.Println("Error writing JSON data to file:", err)
		}
	}

	return nil
}
