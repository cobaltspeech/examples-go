// Copyright (2019 -- present) Cobalt Speech and Language, Inc.

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

	"github.com/cobaltspeech/examples-go/transcribe/transcribe-client/internal/client"
	transcribepb "github.com/cobaltspeech/go-genproto/cobaltspeech/transcribe/v5"
	"github.com/cobaltspeech/log"

	"github.com/spf13/cobra"
)

func buildTransribeCmd() *cobra.Command {
	var outputFn string

	var cmd = &cobra.Command{
		Use:   "recognize <audio file>",
		Short: "Transcribe an audio file.",
		Args:  addGlobalFlagsCheck(cobra.ExactArgs(1)),
		Run: runClientFunc(func(ctx context.Context, c *client.Client, args []string) error {
			logger := log.NewLeveledLogger()

			// args[0] is the audio file
			err := transcribe(ctx, logger, c, args[0], outputFn)
			if err != nil {
				return err
			}

			return nil
		}),
	}

	cmd.Flags().StringVar(&outputFn, "output-json", "", "Path to output json file. If not specified, writes formatted hypothesis to STDOUT.")
	cmd.Flags().StringVar(&recoConfigStr, "recognition-config", "{}", "Json string to configure recognition. "+
		"See https://pkg.go.dev/github.com/cobaltspeech/go-genproto/cobaltspeech/transcribe/v5#RecognitionConfig for more details.")

	return cmd
}

func transcribe(ctx context.Context, logger log.Logger, c *client.Client, wavFn, outputFn string) error {
	var err error

	audio, err := os.Open(wavFn)
	if err != nil {
		return fmt.Errorf("cannot open audio file %w", err)
	}
	defer audio.Close()

	var resp []transcribepb.StreamingRecognizeResponse

	// The callback for results
	callBackFunc := func(response *transcribepb.StreamingRecognizeResponse) {
		if response.Error != nil {
			logger.Info("msg", response.Error.Message)
			return
		}

		if !response.Result.IsPartial && len(response.Result.Alternatives) > 0 {
			resp = append(resp, *response) //nolint:govet // it is okay to copy
		}
	}

	// read the recoConf from the config string
	var recoConfig transcribepb.RecognitionConfig

	decoder := json.NewDecoder(strings.NewReader(recoConfigStr))
	decoder.DisallowUnknownFields()

	if err = decoder.Decode(&recoConfig); err != nil {
		return fmt.Errorf("error decoding the recognition config %w", err)
	}

	// use the default (first available) model if model is not specified
	if recoConfig.ModelId == "" {
		v, err := c.ListModels(ctx)
		if err != nil {
			return fmt.Errorf("error while getting the list of models: %w", err)
		}

		recoConfig.ModelId = v[0].Id
	}

	err = c.StreamingRecognize(ctx, recoConfig, audio, callBackFunc) //nolint:govet // it is okay to copy

	if err != nil {
		return fmt.Errorf("error during recognition %s %w", wavFn, simplifyGrpcErrors(err))
	}

	err = writeTranscript(outputFn, resp)

	if err != nil {
		logger.Error("error", "error writing transcript", "msg", err)
		os.Exit(1)
	}

	return nil
}

func writeTranscript(path string, response []transcribepb.StreamingRecognizeResponse) error {
	var enc *json.Encoder

	if path == "" {
		for i := 0; i < len(response); i++ {
			fmt.Fprintln(os.Stdout, response[i].Result.Alternatives[0].TranscriptFormatted)
		}

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
