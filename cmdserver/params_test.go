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
	"testing"

	"github.com/google/go-cmp/cmp"
)

const (
	foo = "foo"
	bar = "bar"
	baz = "baz"
)

func TestParamsString(t *testing.T) {
	expected := map[string]string{
		foo: "this is fooey",
		bar: "a llama walks into a",
		baz: "what does this even mean",
	}

	p := make(Params)
	p.SetString(foo, expected[foo])
	p.SetString(bar, expected[bar])
	p.SetString(baz, expected[baz])

	if diff := cmp.Diff(map[string]string(p), expected); diff != "" {
		t.Errorf(diff)
	}

	for key, exp := range expected {
		if val, err := p.AsString(key); err != nil {
			t.Error(err)
		} else if val != exp {
			t.Errorf("incorrect val for %q - expected: %q, actual: %q",
				key, exp, val)
		}
	}
}

func TestParamsInt(t *testing.T) {
	expected := "17"
	expectedInt := 17

	p := make(Params)
	p.SetInt(foo, expectedInt)

	if p[foo] != expected {
		t.Errorf("incorrect value - expected: %v, actual: %v", expected, p[foo])
	}

	if val, err := p.AsInt(foo); err != nil {
		t.Error(err)
	} else if val != expectedInt {
		t.Errorf("incorrect int - expected: %v, actual: %v", expectedInt, val)
	}
}

func TestParamsFloat32(t *testing.T) {
	expected := "1.372"
	expectedFloat := float32(1.372)

	p := make(Params)
	p.SetFloat32(foo, expectedFloat)

	if p[foo] != expected {
		t.Errorf("incorrect value - expected: %v, actual: %v", expected, p[foo])
	}

	if val, err := p.AsFloat32(foo); err != nil {
		t.Error(err)
	} else if val != expectedFloat {
		t.Errorf("incorrect float32 - expected: %v, actual: %v", expectedFloat, val)
	}
}

func TestParamsFloat64(t *testing.T) {
	expected := "0.3215"
	expectedFloat := float64(0.3215)

	p := make(Params)
	p.SetFloat64(foo, expectedFloat)

	if p[foo] != expected {
		t.Errorf("incorrect value - expected: %v, actual: %v", expected, p[foo])
	}

	if val, err := p.AsFloat64(foo); err != nil {
		t.Error(err)
	} else if val != expectedFloat {
		t.Errorf("incorrect float64 - expected: %v, actual: %v", expectedFloat, val)
	}
}

func TestParamsBool(t *testing.T) {
	expected := "true"
	expectedBool := true

	p := make(Params)
	p.SetBool(foo, expectedBool)

	if p[foo] != expected {
		t.Errorf("incorrect value - expected: %v, actual: %v", expected, p[foo])
	}

	if val, err := p.AsBool(foo); err != nil {
		t.Error(err)
	} else if val != expectedBool {
		t.Errorf("incorrect bool - expected: %v, actual: %v", expectedBool, val)
	}
}
