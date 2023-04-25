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
	var (
		cfgStr  string
		outPath string
	)

	cmd := &cobra.Command{
		Use:   "recognize <AUDIO_FILE>",
		Short: "Transcribe an audio file.",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) < 1 {
				cmd.PrintErr(cmd.UsageString())

				return
			}

			// TODO: add verbosity
			logger := log.NewLeveledLogger()

			c, err := client.NewClient(serverAddress, client.WithLogger(logger), client.WithInsecure())
			if err != nil {
				cmd.PrintErrf("error: failed to create a client: %v\n", err)

				return
			}

			defer c.Close()

			// args[0] is the audio file
			if err := transcribe(context.Background(), logger, c, cfgStr, args[0], outPath); err != nil {
				cmd.PrintErrf("error: %v\n", err)

				return
			}
		},
	}

	cmd.Flags().StringVar(&outPath, "output-json", "", "Path to output json file. If not specified, writes formatted hypothesis to STDOUT.")
	cmd.Flags().StringVar(&cfgStr, "recognition-config", "{}", "Json string to configure recognition. "+
		"See https://pkg.go.dev/github.com/cobaltspeech/go-genproto/cobaltspeech/transcribe/v5#RecognitionConfig for more details.")

	return cmd
}

func transcribe(ctx context.Context, logger log.Logger, c *client.Client,
	cfgStr, audioPath, outPath string) error {
	// read the recognition config from the config string
	cfg, err := parseRecognitionConfig(cfgStr)
	if err != nil {
		return fmt.Errorf("failed to parse recognition config: %w", err)
	}

	if cfg.ModelId == "" {
		// model is not specified, use the default (first available) model
		if cfg.ModelId, err = getDefaultModelID(ctx, c); err != nil {
			return fmt.Errorf("failed to get default model ID: %w", err)
		}
	}

	audio, err := os.Open(audioPath)
	if err != nil {
		return fmt.Errorf("failed to open audio file (%s): %w", audioPath, err)
	}

	defer audio.Close()

	var responses []*transcribepb.StreamingRecognizeResponse

	// The callback for results
	callBackFunc := func(resp *transcribepb.StreamingRecognizeResponse) {
		if resp == nil {
			return
		}

		if resp.Error != nil {
			logger.Error("msg", "recognition error", "error", resp.Error)
		}

		if !resp.Result.IsPartial && len(resp.Result.Alternatives) > 0 {
			logger.Trace("chan", resp.Result.AudioChannel, "transcript", resp.Result.Alternatives[0].TranscriptFormatted)

			if outPath == "" {
				fmt.Println(resp.Result.Alternatives[0].TranscriptFormatted)
			} else {
				responses = append(responses, resp)
			}
		}
	}

	if err = c.StreamingRecognize(ctx, cfg, audio, callBackFunc); err != nil {
		return fmt.Errorf("failed to transcribe: %w", err)
	}

	if err = writeResponses(outPath, responses); err != nil {
		return fmt.Errorf("failed to write out transcript: %w", err)
	}

	return nil
}

func parseRecognitionConfig(s string) (*transcribepb.RecognitionConfig, error) {
	var cfg transcribepb.RecognitionConfig

	decoder := json.NewDecoder(strings.NewReader(s))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to decode recognition config: %w", err)
	}

	return &cfg, nil
}

func getDefaultModelID(ctx context.Context, c *client.Client) (string, error) {
	v, err := c.ListModels(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list models: %w", err)
	}

	return v[0].Id, nil
}

func writeResponses(path string, responses []*transcribepb.StreamingRecognizeResponse) error {
	if path == "" {
		// empty path to write, do nothing.
		return nil
	}

	outF, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create output file (path=%s): %w", path, err)
	}

	defer outF.Close()

	enc := json.NewEncoder(outF)
	enc.SetIndent("", "  ")

	if err := enc.Encode(responses); err != nil {
		return fmt.Errorf("failed to encode responses JSON: %w", err)
	}

	return nil
}
