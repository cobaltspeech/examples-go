// Copyright (2021) Cobalt Speech and Language Inc.

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

package main

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/cobaltspeech/examples-go/diatheke/internal/audio"
	"github.com/cobaltspeech/examples-go/diatheke/internal/config"
	"github.com/cobaltspeech/sdk-diatheke/grpc/go-diatheke/v2"
	"github.com/cobaltspeech/sdk-diatheke/grpc/go-diatheke/v2/diathekepb"
)

// Contains application settings as defined in the config file.
var appCfg config.Config

func main() {
	// Read the config file
	configFile := flag.String("config", "config.toml", "Path to the config file")
	flag.Parse()
	err := loadConfig(*configFile)
	if err != nil {
		fmt.Printf("error reading config file: %v", err)
		return
	}

	// Create a new client
	opts := make([]diatheke.Option, 0)
	if appCfg.Server.Insecure {
		// NOTE: Secure connections are recommended for production
		opts = append(opts, diatheke.WithInsecure())
	}

	client, err := diatheke.NewClient(appCfg.Server.Address, opts...)
	if err != nil {
		fmt.Printf("error creating client: %v\n", err)
		return
	}
	defer client.Close()

	// Print the server version info
	bctx := context.Background()
	ver, err := client.Version(bctx)
	if err != nil {
		fmt.Printf("error getting server version: %v\n", err)
		return
	}
	fmt.Printf("Server Versions\n")
	fmt.Printf("  Diatheke: %v\n", ver.Diatheke)
	fmt.Printf("  Chosun (NLU): %v\n", ver.Chosun)
	fmt.Printf("  Cubic (ASR): %v\n", ver.Cubic)
	fmt.Printf("  Luna (TTS): %v\n", ver.Luna)

	// Print the list of available models
	modelList, err := client.ListModels(bctx)
	if err != nil {
		fmt.Printf("error getting model list: %v\n", err)
		return
	}
	fmt.Printf("Available Models:\n")
	for _, mdl := range modelList.Models {
		fmt.Printf("  ID: %v\n", mdl.Id)
		fmt.Printf("    Name: %v\n", mdl.Name)
		fmt.Printf("    Language: %v\n", mdl.Language)
		fmt.Printf("    ASR Sample Rate: %v\n", mdl.AsrSampleRate)
		fmt.Printf("    TTS Sample Rate: %v\n\n", mdl.TtsSampleRate)
	}

	// Create a session using the model specified in the config file.
	session, err := client.CreateSession(bctx, appCfg.Server.ModelID)
	if err != nil {
		fmt.Printf("CreateSession error: %v\n", err)
		return
	}

	// Begin processing actions
	for {
		session, err = processActions(client, session)
		if err != nil {
			fmt.Printf("error processing actions: %v\n", err)
			break
		} else if session == nil {
			fmt.Printf("got nil session back")
			break
		}
	}

	// Clean up the session
	if err = client.DeleteSession(bctx, session.Token); err != nil {
		fmt.Printf("error deleting session: %v\n", err)
	}
}

// processActions executes the actions for the given session
// and returns an updated session.
func processActions(client *diatheke.Client, session *diathekepb.SessionOutput,
) (*diathekepb.SessionOutput, error) {
	// Iterate through each action in the list and determine its type.
	for _, action := range session.ActionList {
		if inputAction := action.GetInput(); inputAction != nil {
			// The WaitForUserAction will involve a session update.
			return waitForInput(client, session, inputAction)
		} else if reply := action.GetReply(); reply != nil {
			// Replies do not require a session update.
			err := handleReply(client, reply)
			if err != nil {
				return nil, err
			}
		} else if cmd := action.GetCommand(); cmd != nil {
			// The CommandAction will involve a session update.
			return handleCommand(client, session, cmd)
		} else if scribe := action.GetTranscribe(); scribe != nil {
			// Transcribe actions do not require a session update.
			err := handleTranscribe(client, scribe)
			if err != nil {
				return nil, err
			}
		} else if action.Action != nil {
			return nil, fmt.Errorf("received unknown action type %T", action.Action)
		}
	}

	return nil, fmt.Errorf("action list ended without session update")
}

// waitForInput creates an ASR stream and records audio from the user.
// The audio is sent to Diatheke until an ASR result is returned, which
// is used to return an updated session.
func waitForInput(
	client *diatheke.Client,
	session *diathekepb.SessionOutput,
	inputAction *diathekepb.WaitForUserAction,
) (*diathekepb.SessionOutput, error) {
	// The given input action has a couple of flags to help the app
	// decide when to begin recording audio.
	if inputAction.Immediate {
		// This action is likely waiting for user input in response to
		// a question Diatheke asked, in which case the user should
		// reply immediately. If this flag is false, the app may wait
		// as long as it wants before processing user input (such as
		// waiting for a wake-word below).
		fmt.Printf("(Immediate input required)")
	}

	if inputAction.RequiresWakeWord {
		// This action requires the wake-word to be spoken before
		// any other audio will be processed. Use a wake-word detector
		// and wait for it to trigger.
		fmt.Printf("(Wakeword required)")
	}

	// Create an ASR stream
	stream, err := client.NewSessionASRStream(context.Background(), session.Token)
	if err != nil {
		return nil, err
	}

	// Create something to handle recording audio
	recorder := audio.NewRecorder(appCfg.Recording)
	if err = recorder.Start(); err != nil {
		return nil, err
	}

	fmt.Printf("Recording...\n")

	// Record until we get a result
	result, err := diatheke.ReadASRAudio(stream, recorder.Output(), 8192)
	recorder.Stop()
	if err != nil {
		return nil, err
	}

	fmt.Printf("  ASRResult: %v\n\n", result)

	// Update the session with the result
	return client.ProcessASRResult(context.Background(), session.Token, result)
}

// handleReply uses TTS to play back the reply as speech.
func handleReply(client *diatheke.Client, reply *diathekepb.ReplyAction) error {
	fmt.Printf("  Reply: %v\n\n", reply)

	// Create the TTS stream
	stream, err := client.NewTTSStream(context.Background(), reply)
	if err != nil {
		return err
	}

	// Create something to handle audio playback
	player := audio.NewPlayer(appCfg.Playback)

	// Start the player
	if err = player.Start(); err != nil {
		return err
	}

	// Play the entire reply uninterrupted
	if err = diatheke.WriteTTSAudio(stream, player.Input()); err != nil {
		return err
	}

	// Stop the player
	return player.Stop()
}

// handleTranscribe uses ASR to record a transcription from the user.
func handleTranscribe(client *diatheke.Client, scribe *diathekepb.TranscribeAction) error {
	// Create the transcription stream
	stream, err := client.NewTranscribeStream(context.Background(), scribe)
	if err != nil {
		return err
	}

	// Create something to handle recording audio
	recorder := audio.NewRecorder(appCfg.Recording)
	if err = recorder.Start(); err != nil {
		return err
	}

	fmt.Printf("Recording...\n")

	var finalTranscription strings.Builder
	handler := func(result *diathekepb.TranscribeResult) {
		// Print the result on the same line (overwrite current
		// contents). Note that this assumes that stdout is going
		// to a terminal.
		fmt.Printf("\r%s (confidence: %v)", result.Text, result.Confidence)

		if result.IsPartial {
			return
		}

		// As this is the final result (non-partial), go to
		// the next line in preparation for the next result.
		fmt.Printf("\n")

		// Accumulate all non-partial transcriptions here.
		finalTranscription.WriteString(result.Text)
	}

	err = diatheke.ReadTranscribeAudio(stream, recorder.Output(), 8192, handler)
	if err != nil {
		return err
	}

	fmt.Printf("  Transcription: %v\n\n", finalTranscription.String())
	return nil
}

// handleCommand executes the specified command.
func handleCommand(
	client *diatheke.Client,
	session *diathekepb.SessionOutput,
	cmd *diathekepb.CommandAction,
) (*diathekepb.SessionOutput, error) {
	fmt.Printf("  Command:\n")
	fmt.Printf("    ID: %v\n", cmd.Id)
	fmt.Printf("    Input params: %v\n\n", cmd.InputParameters)

	// Update the session with the command result
	result := diathekepb.CommandResult{
		Id: cmd.Id,
	}

	session, err := client.ProcessCommandResult(context.Background(), session.Token, &result)
	if err != nil {
		err = fmt.Errorf("ProcessCommandResult error: %v", err)
	}
	return session, err
}

// loadConfig reads the specified config file at application startup.
func loadConfig(filepath string) error {
	var err error
	appCfg, err = config.ReadConfigFile(filepath)
	if err != nil {
		return err
	}

	// Verify that we have the required fields for this demo.
	// The following are required for this demo
	if appCfg.Server.ModelID == "" {
		return fmt.Errorf("missing Diatheke ModelID in the config file")
	}

	if appCfg.Playback.Application == "" {
		return fmt.Errorf("missing Playback application in the config file")
	}

	if appCfg.Recording.Application == "" {
		return fmt.Errorf("missing Recording application in the config file")
	}

	return nil
}
