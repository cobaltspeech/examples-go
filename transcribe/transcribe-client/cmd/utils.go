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

import "github.com/cobaltspeech/log/pkg/level"

// getLogLevel reads the configured logging level.
func getLogLevel(v int) level.Level {
	logLevel := level.Error

	logLevel |= level.Info

	if v > 0 {
		logLevel |= level.Debug
	}

	if v > 1 {
		logLevel |= level.Trace
	}

	return logLevel
}
