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
	"log"
	"math"
	"strings"

	"github.com/cobaltspeech/examples-go/diatheke/internal/audio"
	"github.com/cobaltspeech/examples-go/diatheke/internal/config"
	"github.com/cobaltspeech/sdk-cubic/grpc/go-cubic"
	"github.com/cobaltspeech/sdk-cubic/grpc/go-cubic/cubicpb"
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
		log.Fatalf("error reading config file: %v", err)
		return
	}

	// Create the Wake-word cubicsvr client.  This client is an ASR
	// model that is only focused on identifying the wake word in a
	// long running recognizer, and unblocking once the wake word is
	// detected.
	// Creating client without TLS. Remove cubic.WithInsecure() if using TLS
	wwOpts := make([]cubic.Option, 0)
	if appCfg.Server.Insecure {
		// NOTE: Secure connections are recommended for production
		wwOpts = append(wwOpts, cubic.WithInsecure())
	}
	wwClient, err := cubic.NewClient(appCfg.WakeWordServer.Address, wwOpts...)
	if err != nil {
		log.Fatal(err)
	}
	defer wwClient.Close()

	// Use the first wake word model available
	modelResp, err := wwClient.ListModels(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	model := modelResp.Models[0]
	sampleRateBytes := model.Attributes.SampleRate * 2 // 2 bytes per sample

	cfg := &cubicpb.RecognitionConfig{
		ModelId:               model.Id,
		EnableRawTranscript:   true,
		EnableWordTimeOffsets: true,
	}

	// Creader a recorder that will read audio from the microphone.
	recorder := audio.NewRecorder(appCfg.Recording)
	if err = recorder.Start(); err != nil {
		log.Fatalf("Recorder Error!!!!")
		return
	}

	// Wrap the recorder in a "StoppableReader" that will allow the reader's Read()
	// method to return EOF on the first Read() after abortFunc is called (to force an
	// exit from the wake word cubicsvr after the wake word is recognized), but later
	// Read() calls will be successful.  This StoppableReader will also allow audio
	// to be re-wound so when the Diatheke server reads from the same stream it can
	// start reading right at the start of the wake word.
	wwBufferSize := int(float32(sampleRateBytes) * appCfg.WakeWordServer.AudioBufferSec)
	stoppableReader := audio.NewStoppableReader(recorder.Output(), wwBufferSize)

	// Create a new diatheke client
	opts := make([]diatheke.Option, 0)
	if appCfg.Server.Insecure {
		// NOTE: Secure connections are recommended for production
		opts = append(opts, diatheke.WithInsecure())
	}
	diathekeClient, err := diatheke.NewClient(appCfg.Server.Address, opts...)
	if err != nil {
		log.Fatalf("error creating diathekeClient: %v\n", err)
		return
	}
	defer diathekeClient.Close()

	// Print the server version info
	bctx := context.Background()
	ver, err := diathekeClient.Version(bctx)
	if err != nil {
		log.Fatalf("error getting server version: %v\n", err)
		return
	}
	log.Printf("Server Versions\n")
	log.Printf("  Diatheke: %v\n", ver.Diatheke)
	log.Printf("  Chosun (NLU): %v\n", ver.Chosun)
	log.Printf("  Cubic (ASR): %v\n", ver.Cubic)
	log.Printf("  Luna (TTS): %v\n", ver.Luna)

	// Print the list of available models
	modelList, err := diathekeClient.ListModels(bctx)
	if err != nil {
		log.Fatalf("error getting model list: %v\n", err)
		return
	}
	log.Printf("Available Models:\n")
	for _, mdl := range modelList.Models {
		log.Printf("  ID: %v\n", mdl.Id)
		log.Printf("    Name: %v\n", mdl.Name)
		log.Printf("    Language: %v\n", mdl.Language)
		log.Printf("    ASR Sample Rate: %v\n", mdl.AsrSampleRate)
		log.Printf("    TTS Sample Rate: %v\n\n", mdl.TtsSampleRate)
	}

	// Create a session using the model specified in the config file.
	session, err := diathekeClient.CreateSession(bctx, appCfg.Server.ModelID)
	if err != nil {
		log.Fatalf("CreateSession error: %v\n", err)
		return
	}

	// Begin processing actions
	for {
		// Run diatheke
		session, err = processActions(wwClient, cfg, appCfg.WakeWordServer.WakePhrases,
			appCfg.WakeWordServer.MinWakePhraseConfidence, int(sampleRateBytes),
			diathekeClient, session, stoppableReader)
		if err != nil {
			log.Fatalf("error processing actions: %v\n", err)
			break
		}
	}

	// Clean up the session
	if err = diathekeClient.DeleteSession(bctx, session.Token); err != nil {
		log.Fatalf("error deleting session: %v\n", err)
	}
}

// processActions executes the actions for the given session
// and returns an updated session.
func processActions(wwClient *cubic.Client, wwCfg *cubicpb.RecognitionConfig,
	wwPhrases []string, wwMinConf float64, wwBytesPerSec int,
	diathekeClient *diatheke.Client, session *diathekepb.SessionOutput,
	reader *audio.StoppableReader) (*diathekepb.SessionOutput, error) {
	// Iterate through each action in the list and determine its type.
	for _, action := range session.ActionList {
		if inputAction := action.GetInput(); inputAction != nil {
			// The WaitForUserAction will involve a session update.
			log.Println(".....wait for input")
			return waitForInput(reader, wwClient, wwCfg, wwPhrases, wwMinConf,
				wwBytesPerSec, diathekeClient, session, inputAction)
		} else if reply := action.GetReply(); reply != nil {
			log.Println(".....GetReply")
			// Replies do not require a session update.
			err := handleReply(diathekeClient, reply)
			if err != nil {
				return nil, err
			}
		} else if cmd := action.GetCommand(); cmd != nil {
			log.Println(".....GetCommand")
			// The CommandAction will involve a session update.
			return handleCommand(diathekeClient, session, cmd)
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
	reader *audio.StoppableReader,
	wwClient *cubic.Client,
	wwCfg *cubicpb.RecognitionConfig,
	wwPhrases []string,
	wwMinConf float64,
	wwBytesPerSec int,
	diathekeClient *diatheke.Client,
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
		log.Printf("(Immediate input required) ")
	}

	// Wait for a wake word if one is required.
	// Whether a wake word is required at the current time depends on the diatheke model.
	// Some diatheke models will never expect or wait for a wake word.
	if inputAction.RequiresWakeWord {
		// This action requires the wake-word to be spoken before
		// any other audio will be processed. Use a wake-word detector
		// and wait for it to trigger.
		log.Printf("(Wakeword required) ")

		// Define a callback function to check if the wake word was present in the endpointed audio.
		// This example expects the wake phrase only be one token in length (but multi-word wake
		// phrasess could be supported with this handler if the wake word model used a multi-word
		// token for the wake phrase)
		var wakeWordStartTimeSec float64
		resultHandler := func(resp *cubicpb.RecognitionResponse) {
			for _, result := range resp.Results {
				if !result.IsPartial {
					transcript := result.Alternatives[0].Transcript
					confidence := result.Alternatives[0].Confidence
					for _, wakePhrase := range wwPhrases {
						if strings.HasSuffix(transcript, wakePhrase) && confidence >= wwMinConf {
							// Find the index of the first word in the wake phrase in the alternatives list.
							wakePhraseFirstWord := strings.Split(wakePhrase, " ")[0]
							wakePhraseStartIndex := -1
							for i, wordInfo := range result.Alternatives[0].Words {
								if wakePhraseFirstWord == wordInfo.Word {
									wakePhraseStartIndex = i
									break
								}
							}

							wakeWordStartTimeSec = float64(result.Alternatives[wakePhraseStartIndex].StartTime.Seconds) +
								float64(result.Alternatives[wakePhraseStartIndex].StartTime.Nanos)/1000000000.0

							reader.Stop()
							break
						}
					}
				}
			}
		}

		// Run the wake-word recognizer, this will block until a wake word is found.
		// The time that the wake word stars will be set in "wakeWordStartTimeSec".
		// The start time of the wake word will be set in "wakeWordStartTimeSec"
		log.Println("Waiting for wake word...")
		err := wwClient.StreamingRecognize(context.Background(), wwCfg, reader, resultHandler)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Wake word found")

		// Rewind the recorder to the start of the wake word.
		// The start of the rewound stream is now considered to be time=0.0
		err = reader.Rewind(int(math.Round(wakeWordStartTimeSec*float64(wwBytesPerSec))), true)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Create an ASR stream
	stream, err := diathekeClient.NewSessionASRStream(context.Background(), session.Token)
	if err != nil {
		return nil, err
	}

	log.Printf("Recording...\n")

	// Record until we get a result
	result, err := diatheke.ReadASRAudio(stream, reader, 8192)
	if err != nil {
		return nil, err
	}

	log.Printf("  ASRResult: %v\n\n", result)

	// Reset the historical buffer in the reader since it is no longer needed.
	// Also, reading from a StoppableBuffer that has been Rewound() twice without a
	// Reset() call between the rewinds is not supported.
	reader.Reset()

	// Update the session with the result
	return diathekeClient.ProcessASRResult(context.Background(), session.Token, result)
}

// handleReply uses TTS to play back the reply as speech.
func handleReply(client *diatheke.Client, reply *diathekepb.ReplyAction) error {
	log.Printf("  TTS Reply: %v\n\n", reply)

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
		log.Println("Error writing audio to TTS (skipping TTS)")
		return nil
	}

	// Stop the player
	return player.Stop()
}

// handleCommand executes the specified command.
func handleCommand(
	client *diatheke.Client,
	session *diathekepb.SessionOutput,
	cmd *diathekepb.CommandAction,
) (*diathekepb.SessionOutput, error) {
	log.Printf("  Command:\n")
	log.Printf("    ID: %v\n", cmd.Id)
	log.Printf("    Input params: %v\n\n", cmd.InputParameters)

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
