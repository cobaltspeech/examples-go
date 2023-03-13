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
	"encoding/json"
	"fmt"
	"os"

	cubicpb "github.com/cobaltspeech/go-genproto/cobaltspeech/cubic/v5"

	"github.com/cobaltspeech/examples-go/cobalt-transcribe/internal/client"
	"github.com/cobaltspeech/log"
	"github.com/spf13/cobra"
)

func buildTransribeCmd() *cobra.Command {
	var (
		wavFn    string
		outputFn string
	)

	var cmd = &cobra.Command{
		Use:   "transcribe",
		Short: "Transcribe wav file.",
		Args:  addGlobalFlagsCheck(cobra.NoArgs),
		Run: runClientFunc(func(c *client.Client, args []string) error {

			logger := log.NewLeveledLogger()

			err := transcribe(c, wavFn, outputFn, logger)
			if err != nil {
				return err
			}

			return nil
		}),
	}

	cmd.Flags().StringVar(&wavFn, "input", "", "Path to input audio file (required)")
	cmd.Flags().StringVar(&outputFn, "output", "", "Path to output json file")
	cmd.Flags().StringVar(&cConf.Server.ModelID, "model", "1", "Model ID")

	err := cmd.MarkFlagRequired("input")
	if err != nil {
		fmt.Println("cannot read the commandline arguments: ", err)
		os.Exit(1)
	}

	return cmd
}

func transcribe(c *client.Client, wavFn, outputFn string, logger log.Logger) error {
	var err error

	audio, err := os.Open(wavFn)
	if err != nil {
		return fmt.Errorf("cannot open audio file %w", err)
	}
	defer audio.Close()

	err = c.StreamingRecognize(context.Background(),
		cubicpb.RecognitionConfig{ModelId: cConf.Server.ModelID},
		audio,
		func(response *cubicpb.StreamingRecognizeResponse) { // The callback for results
			if !response.Result.IsPartial && len(response.Result.Alternatives) > 0 {
				err = writeTranscript(outputFn, *response)

				if err != nil {
					logger.Error("error", "error writing transcript", "msg", err)
					os.Exit(1)
				}
			}
		})

	if err != nil {
		return fmt.Errorf("error during recognition %s %w", wavFn, simplifyGrpcErrors(err))
	}

	return err
}

func writeTranscript(path string, response cubicpb.StreamingRecognizeResponse) error {
	var enc *json.Encoder

	if path == "" {
		fmt.Fprintln(os.Stdout, response.Result.Alternatives[0].TranscriptFormatted)
		return nil
	} else {
		outF, err := os.Create(path)
		if err != nil {
			return err
		}
		defer outF.Close()

		enc = json.NewEncoder(outF)
	}

	enc.SetIndent("", "  ")

	if err := enc.Encode(response); err != nil {
		return err
	}

	return nil
}
