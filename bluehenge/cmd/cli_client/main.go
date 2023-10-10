// Copyright (2020) Cobalt Speech and Language Inc.

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
	"os"

	helper "github.com/cobalspeech/examples-go/bluehenge/internal"
	bluehengepb "github.com/cobaltspeech/go-genproto/cobaltspeech/bluehenge/v1"
	"github.com/cobaltspeech/log"
	"github.com/cobaltspeech/log/pkg/level"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// Read the config file
	bhgAdd := flag.String("add", "", "Bluehenge address")
	// TODO(gsegovia2018): This will be used for gremlin functions
	_ = flag.String("gremlinAdd", "", "Gremlin server address")
	flag.Parse()

	if *bhgAdd == "" {
		flag.Usage()
		os.Exit(1)
	}

	logger := log.NewLeveledLogger()
	logger.SetFilterLevel(level.Error | level.Info | level.Debug)

	conn, err := grpc.Dial(*bhgAdd, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("msg", "unable to dial server", "error", err)
	}
	defer conn.Close()

	client := bluehengepb.NewBluehengeServiceClient(conn)
	ctx := context.Background()
	helper.Version(client, ctx)

	helper.RunSession(client, ctx)

}
