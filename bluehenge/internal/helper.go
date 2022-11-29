package internal

import (
	"bufio"
	"context"
	"fmt"
	"os"

	bluehengepb "github.com/cobaltspeech/go-genproto/cobaltspeech/bluehenge/v1"
	diathekepb "github.com/cobaltspeech/go-genproto/cobaltspeech/diatheke/v3"
)

func Version(client bluehengepb.BluehengeServiceClient, ctx context.Context) {
	versionResponse, err := client.Version(ctx, &bluehengepb.VersionRequest{})
	if err != nil {
		fmt.Printf("Not able to get the version from bluehenge server %v", err)
		return
	}
	fmt.Printf("Server Versions\n")
	fmt.Printf("  Bluehenge: %v\n", versionResponse.Bluehenge)
	fmt.Printf("  Diatheke: %v\n", versionResponse.DiathekeVersionResponse.Diatheke)
	fmt.Printf("  Chosun (NLU): %v\n", versionResponse.DiathekeVersionResponse.Chosun)
	fmt.Printf("  Cubic (ASR): %v\n", versionResponse.DiathekeVersionResponse.Cubic)
	fmt.Printf("  Luna (TTS): %v\n\n", versionResponse.DiathekeVersionResponse.Luna)
}

func ListModels(client bluehengepb.BluehengeServiceClient, ctx context.Context) diathekepb.ModelInfo {
	listModelsResp, err := client.ListModels(ctx, &bluehengepb.ListModelsRequest{})
	if err != nil {
		fmt.Printf("Not able to get the version from bluehenge server %v", err)
		return diathekepb.ModelInfo{}
	}
	fmt.Printf("Available Models:\n")
	for _, mdl := range listModelsResp.DiathekeListModelsResponse.Models {
		fmt.Printf("  ID: %v\n", mdl.Id)
		fmt.Printf("    Name: %v\n", mdl.Name)
		fmt.Printf("    Language: %v\n", mdl.Language)
		fmt.Printf("    ASR Sample Rate: %v\n", mdl.AsrSampleRate)
		fmt.Printf("    TTS Sample Rate: %v\n\n", mdl.TtsSampleRate)
	}
	return diathekepb.ModelInfo(*listModelsResp.DiathekeListModelsResponse.Models[0])
}

func RunSession(client bluehengepb.BluehengeServiceClient, ctx context.Context) {
	firstModel := ListModels(client, ctx)

	// Create a session using the specified model ID.
	createSessionReq := &bluehengepb.CreateSessionRequest{
		DiathekeCreateSessionRequest: &diathekepb.CreateSessionRequest{
			ModelId: firstModel.Id,
		},
	}

	session, err := client.CreateSession(ctx, createSessionReq)
	if err != nil {
		fmt.Printf("CreateSession error: %v\n", err)
		return
	}
	sessionOutput := session.DiathekeCreateSessionResponse.SessionOutput

	// Being processing actions
	for {
		sessionOutput, err = processActions(client, sessionOutput)
		if err != nil {
			fmt.Printf("error processing actions: %v\n", err)
			break
		} else if session == nil {
			fmt.Printf("got nil session back")
			break
		}
	}

	// Clean up the session.
	_, err = client.DeleteSession(ctx, &bluehengepb.DeleteSessionRequest{
		DiathekeDeleteSessionRequest: &diathekepb.DeleteSessionRequest{
			TokenData: sessionOutput.Token,
		},
	})
	if err != nil {
		fmt.Printf("error deleting session: %v\n", err)
	}
	fmt.Println("Session deleted")
}

// processActions executes the actions for the given session
// and returns an updated session.
func processActions(client bluehengepb.BluehengeServiceClient, sessionOut *diathekepb.SessionOutput,
) (*diathekepb.SessionOutput, error) {
	// Iterate through each action in the list and determine its type.
	fmt.Println("Session ActionList", sessionOut.ActionList)
	for _, action := range sessionOut.ActionList {
		switch action.Action.(type) {
		case *diathekepb.ActionData_Input:
			fmt.Printf("Input: %v", action.GetInput())
			// The WaitForUserAction will involve a session update.
			return waitForInput(client, sessionOut, action.GetInput())
		case *diathekepb.ActionData_Reply:
			// Replies do not require a session update.
			handleReply(client, action.GetReply())
		case *diathekepb.ActionData_Command:
			// The CommandAction will involve a session update.
			return handleCommand(client, sessionOut, action.GetCommand())
		case *diathekepb.ActionData_Transcribe:
			err := handleTranscribe(action.GetTranscribe())
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("received unknown action type %T", action.Action)
		}
	}

	return nil, fmt.Errorf("action list ended without session update")
}

// waitForInput prompts the user for text input, then updates the
// session based on the user-supplied text.
func waitForInput(
	client bluehengepb.BluehengeServiceClient,
	session *diathekepb.SessionOutput,
	inputAction *diathekepb.WaitForUserAction,
) (*diathekepb.SessionOutput, error) {
	// Display a prompt
	fmt.Printf("\n\nBluehenge> ")

	// Wait for user input on stdin
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	text := scanner.Text()

	// Update the session with the text
	session, err := processText(context.Background(), client, session.Token, text)
	if err != nil {
		err = fmt.Errorf("ProcessText error: %v", err)
	}
	return session, err
}

// ProcessText sends the given text to Diatheke and returns an updated
// session token.
func processText(
	ctx context.Context, client bluehengepb.BluehengeServiceClient, token *diathekepb.TokenData, text string,
) (*diathekepb.SessionOutput, error) {
	req := &bluehengepb.UpdateSessionRequest{
		DiathekeUpdateSessionRequest: &diathekepb.UpdateSessionRequest{
			SessionInput: &diathekepb.SessionInput{
				Token: token,
				Input: &diathekepb.SessionInput_Text{
					Text: &diathekepb.TextInput{
						Text: text,
					},
				},
			},
		},
	}

	resp, err := client.UpdateSession(ctx, req)
	if err != nil {
		err = fmt.Errorf("processText error: %v", err)
	}

	return resp.DiathekeUpdateSessionResponse.SessionOutput, err
}

// handleReply prints the given reply text to stdout.
func handleReply(client bluehengepb.BluehengeServiceClient, reply *diathekepb.ReplyAction) {
	fmt.Printf("  Reply: %v\n", reply.Text)
}

// handleCommand executes the task specified by the given command
// and returns an updated session based on the command result.
func handleCommand(
	client bluehengepb.BluehengeServiceClient,
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
	sess, err := processCommandResult(context.Background(), client, session.Token, &result)
	if err != nil {
		err = fmt.Errorf("ProcessCommandResult error: %v", err)
	}
	return sess.DiathekeUpdateSessionResponse.SessionOutput, err
}

// ProcessCommandResult sends the given command result to Diatheke and
// returns an updated session token. This function should be called in
// response to a command action Diatheke sent previously.
func processCommandResult(
	ctx context.Context,
	client bluehengepb.BluehengeServiceClient,
	token *diathekepb.TokenData,
	result *diathekepb.CommandResult,
) (*bluehengepb.UpdateSessionResponse, error) {
	req := bluehengepb.UpdateSessionRequest{
		DiathekeUpdateSessionRequest: &diathekepb.UpdateSessionRequest{
			SessionInput: &diathekepb.SessionInput{
				Token: token,
				Input: &diathekepb.SessionInput_Cmd{
					Cmd: result,
				},
			},
		},
	}

	return client.UpdateSession(ctx, &req)
}

// handleTranscribe uses ASR to record a transcription from the user.
func handleTranscribe(scribe *diathekepb.TranscribeAction) error {
	fmt.Printf("  Transcribe: %+v\n", scribe)
	return nil
}
