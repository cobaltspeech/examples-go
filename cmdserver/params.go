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
	"fmt"
	"strconv"
)

// Params is an alias for a map[string]string that
// includes some convenience functions for converting
// to other types. To create a new Params object, use
// the go standard make function (e.g., `make(Params)`).
// Note that the convenience functions are not safe
// to use concurrently (just as it is not safe to
// access a regular map in Go concurrently).
type Params map[string]string

// AsString returns the parameter value for the given key
// as a string. Returns an error if the key was not found.
// Note that the underlying map may be used directly to
// get the parameter and check it's existence without the
// error (e.g., `val, ok := p[key]`).
func (p Params) AsString(key string) (string, error) {
	val, found := p[key]
	if !found {
		return val, fmt.Errorf("missing key %q", key)
	}

	return val, nil
}

// SetString sets the parameter value for the given key.
// This is no different than setting it directly on the
// object (e.g., `p[key] = val`).
func (p Params) SetString(key, val string) {
	p[key] = val
}

// AsInt returns the parameter value for the given key
// as an int. Returns an error if the key was not found
// or there was a problem during conversion.
func (p Params) AsInt(key string) (int, error) {
	if val, err := p.AsString(key); err != nil {
		return 0, err
	} else {
		return strconv.Atoi(val)
	}
}

// SetInt converts the given int to a string and stores
// it in the parameter map.
func (p Params) SetInt(key string, val int) {
	p[key] = strconv.Itoa(val)
}

// AsFloat32 returns the parameter value for the given key
// as a float32. Returns an error if the key was not found
// or there was problem during conversion.
func (p Params) AsFloat32(key string) (float32, error) {
	if val, err := p.AsString(key); err != nil {
		return 0.0, err
	} else {
		f, err := strconv.ParseFloat(val, 32)
		return float32(f), err
	}
}

// SetFloat32 converts the given float32 to a string and stores
// it in the parameter map. This uses the 'g' formatting style
// for the conversion, with a precision of 4. For different
// formatting, it is recommended to use strconv.FormatFloat().
func (p Params) SetFloat32(key string, val float32) {
	p[key] = strconv.FormatFloat(float64(val), 'g', 4, 32)
}

// AsFloat64 returns the parameter value for the given key
// as a float64. Returns an error if the key was not found
// or there was a problem during conversion.
func (p Params) AsFloat64(key string) (float64, error) {
	if val, err := p.AsString(key); err != nil {
		return 0.0, err
	} else {
		return strconv.ParseFloat(val, 64)
	}
}

// SetFloat64 converts the given float64 to a string and stores
// it in the parameter map. This uses the 'g' formatting style
// for the conversion, with a precision of 4. For different
// formatting, it is recommended to use strconv.FormatFloat().
func (p Params) SetFloat64(key string, val float64) {
	p[key] = strconv.FormatFloat(val, 'g', 4, 64)
}

// AsBool returns the parameter value for the given key
// as a bool. Returns an error if the key was not found
// or there was a problem during conversion.
func (p Params) AsBool(key string) (bool, error) {
	if val, err := p.AsString(key); err != nil {
		return false, err
	} else {
		return strconv.ParseBool(val)
	}
}

// SetBool converts the given bool to a string and stores it
// in the parameter map.
func (p Params) SetBool(key string, val bool) {
	p[key] = strconv.FormatBool(val)
}
