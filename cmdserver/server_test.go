// Copyright (2021-present) Cobalt Speech and Language, Inc. All rights reserved.

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
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSetCommand(t *testing.T) {
	// Create the server
	svr := NewServer(nil)

	// Add handlers (and expected test data)
	cmd1ExpectedIn := Input{
		SessionID: "random31234",
		ModelID:   "1",
		CommandID: "cmd1",
		Parameters: Params{
			"a": "some data",
			"b": "other data",
		},
	}

	cmd1ExpectedOut := Output{CommandID: "cmd1"}

	svr.SetCommand("cmd1", func(in Input, out *Output) error {
		diff := cmp.Diff(cmd1ExpectedIn.Parameters, in.Parameters)
		if diff != "" {
			t.Error(diff)
		}

		return nil
	})

	cmd5ExpectedIn := Input{CommandID: "cmd5"}
	cmd5ExpectedOut := Output{
		CommandID: "cmd5",
		Parameters: Params{
			"c": "crazy",
			"d": "silly",
		},
	}
	svr.SetCommand("cmd5", func(in Input, out *Output) error {
		if len(in.Parameters) != 0 {
			t.Errorf("got unexpected input params: %+v", in.Parameters)
		}

		*out = cmd5ExpectedOut
		return nil
	})

	// Create the test server and client
	tsvr := httptest.NewServer(&svr)
	defer tsvr.Close()
	client := newTestClient(tsvr)

	if cmd1Out, err := client.send(cmd1ExpectedIn); err != nil {
		t.Error(err)
	} else if diff := cmp.Diff(cmd1ExpectedOut, cmd1Out); diff != "" {
		t.Error(diff)
	}

	if cmd5Out, err := client.send(cmd5ExpectedIn); err != nil {
		t.Error(err)
	} else if diff := cmp.Diff(cmd5ExpectedOut, cmd5Out); diff != "" {
		t.Error(diff)
	}
}

func TestUnknownCmd(t *testing.T) {
	// Create the server
	svr := NewServer(nil)
	svr.SetCommand("junk", func(Input, *Output) error {
		t.Error("called handler when it shouldn't")
		return nil
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
	svr.SetCommand("junk", func(Input, *Output) error {
		t.Error("called handler when it shouldn't")
		return nil
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

func TestHandlerRegistry(t *testing.T) {
	hr := newRegistry()
	hr.setModelCmd("m1", "c1", func(in Input, out *Output) error {
		if in.ModelID != "m1" || in.CommandID != "c1" || in.Parameters["target"] != "m1c1" {
			return fmt.Errorf("wrong input sent to m1c1: %v", in)
		}

		return nil
	})

	hr.setCmd("c1", func(in Input, out *Output) error {
		if in.CommandID != "c1" || in.Parameters["target"] != "c1" {
			return fmt.Errorf("wrong input sent to c1: %v", in)
		}

		return nil
	})

	hr.setCmd("c2", func(in Input, out *Output) error {
		if in.CommandID != "c2" || in.Parameters["target"] != "c2" {
			return fmt.Errorf("wrong input sent to c2: %v", in)
		}

		return nil
	})

	hr.setModel("m1", func(in Input, out *Output) error {
		if in.ModelID != "m1" || in.Parameters["target"] != "m1" {
			return fmt.Errorf("wrong input sent to m1: %v", in)
		}

		return nil
	})

	hr.setModel("m2", func(in Input, out *Output) error {
		if in.ModelID != "m2" || in.Parameters["target"] != "m2" {
			return fmt.Errorf("wrong input sent to m2: %v", in)
		}

		return nil
	})

	// Now that the registry is set up, run tests
	testList := []struct {
		modelID string
		cmdID   string
		target  string
		found   bool
	}{
		{"m1", "c1", "m1c1", true},
		{"x", "c1", "c1", true},
		{"x", "c2", "c2", true},
		{"m1", "y", "m1", true},
		{"m2", "y", "m2", true},
		{"x", "x", "", false},
		{"m1", "c2", "c2", true},
		{"m2", "c1", "c1", true},
		{"m2", "c2", "c2", true},
	}

	for i := range testList {
		test := testList[i]
		name := test.modelID + "-" + test.cmdID
		t.Run(name, func(t *testing.T) {
			in := Input{
				ModelID:    test.modelID,
				CommandID:  test.cmdID,
				Parameters: make(Params),
			}
			in.Parameters.SetString("target", test.target)

			handler, found := hr.findHandler(in)
			if found != test.found {
				t.Fatalf("found flag mismatch - expected: %v, actual: %v",
					test.found, found)
			}

			if !found {
				return
			}

			if err := handler(in, nil); err != nil {
				t.Error(err)
			}
		})
	}
}

type testClient struct {
	client *http.Client
	url    string
}

func newTestClient(svr *httptest.Server) testClient {
	return testClient{
		client: svr.Client(),
		url:    svr.URL,
	}
}

func (tc *testClient) send(in Input) (Output, error) {
	var body bytes.Buffer
	encoder := json.NewEncoder(&body)
	if err := encoder.Encode(&in); err != nil {
		return Output{}, err
	}

	resp, err := tc.client.Post(tc.url, "application/json", &body)
	if err != nil {
		return Output{}, err
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)
	var result Output
	err = decoder.Decode(&result)
	return result, err
}
