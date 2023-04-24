// Copyright (2021 -- present) Cobalt Speech and Language, Inc.

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

package cmdserver

// Input contains the command input data as received from
// Diatheke.
type Input struct {
	// The Diatheke model ID where the command is defined.
	ModelID string `json:"modelID"`

	// The ID of the command to execute.
	CommandID string `json:"id"`

	// The unique ID of the session trying to execute the command.
	SessionID string `json:"sessionID"`

	// Defined parameters (possibly empty).
	Parameters Params `json:"inputParameters"`

	// Application specific, user-defined data. Implementers
	// may use this field to store arbitrary data for a session.
	// Implementers are responsible for passing this data to the
	// command Output, and are free to modify it however they
	// want (or clear it entirely).
	Metadata string `json:"metadata"`
}

// Output contains the command data to send back to Diatheke.
type Output struct {
	// The ID of the command that was executed
	CommandID string `json:"id"`

	// Parameters that Diatheke expects to be returned (possibly
	// empty).
	Parameters Params `json:"outParameters,omitempty"`

	// Application specific, user-defined data to associate
	// with the session that executed this command.
	Metadata string `json:"metadata,omitempty"`

	// An error message to indicate to Diatheke that something
	// went wrong during command execution. Most implementers
	// won't need to set this field directly as it set by the
	// server when an error is returned from the handler.
	Error string `json:"error,omitempty"`
}

// Handler is a function that takes command input and sets
// the command output that is expected by a Diatheke command.
type Handler func(in Input, out *Output) error
