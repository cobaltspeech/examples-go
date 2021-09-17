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

package cmdserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestServerEndpoint(t *testing.T) {
	// Create the server
	svr := NewServer(nil)

	// Add handlers (and expected test data)
	cmd1ExpectedIn := cmdRequest{
		ID: "cmd1",
		InputParameters: map[string]string{
			"a": "some data",
			"b": "other data",
		},
	}

	cmd1ExpectedOut := cmdResponse{ID: "cmd1"}

	svr.SetHandler("cmd1", func(input Params) (Params, error) {
		diff := cmp.Diff(
			cmd1ExpectedIn.InputParameters,
			map[string]string(input),
		)
		if diff != "" {
			t.Error(diff)
		}

		return nil, nil
	})

	cmd5ExpectedIn := cmdRequest{ID: "cmd5"}
	cmd5ExpectedOut := cmdResponse{
		ID: "cmd5",
		OutParameters: map[string]string{
			"c": "crazy",
			"d": "silly",
		},
	}
	svr.SetHandler("cmd5", func(input Params) (Params, error) {
		if len(input) != 0 {
			t.Errorf("got unexpected input params: %+v", input)
		}

		return cmd5ExpectedOut.OutParameters, nil
	})

	// Create the test server
	tsvr := httptest.NewServer(&svr)
	defer tsvr.Close()

	client := tsvr.Client()
	send := func(cmd cmdRequest) (cmdResponse, error) {
		var body bytes.Buffer
		encoder := json.NewEncoder(&body)
		if sErr := encoder.Encode(&cmd); sErr != nil {
			return cmdResponse{}, sErr
		}

		resp, sErr := client.Post(tsvr.URL, "application/json", &body)
		if sErr != nil {
			return cmdResponse{}, sErr
		}
		defer resp.Body.Close()

		decoder := json.NewDecoder(resp.Body)
		var result cmdResponse
		sErr = decoder.Decode(&result)
		return result, sErr
	}

	if cmd1Out, err := send(cmd1ExpectedIn); err != nil {
		t.Error(err)
	} else if diff := cmp.Diff(cmd1ExpectedOut, cmd1Out); diff != "" {
		t.Error(diff)
	}

	if cmd5Out, err := send(cmd5ExpectedIn); err != nil {
		t.Error(err)
	} else if diff := cmp.Diff(cmd5ExpectedOut, cmd5Out); diff != "" {
		t.Error(diff)
	}
}

func TestUnknownCmd(t *testing.T) {
	// Create the server
	svr := NewServer(nil)
	svr.SetHandler("junk", func(Params) (Params, error) {
		t.Error("called handler when it shouldn't")
		return nil, nil
	})

	// Create the test server
	tsvr := httptest.NewServer(&svr)
	defer tsvr.Close()

	client := tsvr.Client()
	var body bytes.Buffer
	if _, err := body.WriteString(`{"id": "not_junk"}`); err != nil {
		t.Fatal(err)
	}

	resp, err := client.Post(tsvr.URL, "application/json", &body)
	if err != nil {
		t.Error(err)
	} else if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("did not get status InternalServerError in response")
	}
}

func TestBadRequest(t *testing.T) {
	// Create the server
	svr := NewServer(nil)
	svr.SetHandler("junk", func(Params) (Params, error) {
		t.Error("called handler when it shouldn't")
		return nil, nil
	})

	// Create the test server
	tsvr := httptest.NewServer(&svr)
	defer tsvr.Close()

	client := tsvr.Client()
	var body bytes.Buffer
	if _, err := body.WriteString(`{"bad_request":"hahaha"`); err != nil {
		t.Fatal(err)
	}

	resp, err := client.Post(tsvr.URL, "application/json", &body)
	if err != nil {
		t.Error(err)
	} else if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("did not get status BadRequest in response")
	}
}
