// Copyright 2024 Alexandre Mahdhaoui
//
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
	"strings"
	"testing"
)

const smallProto = `
syntax = "proto3";

package songe.hello.v1;

service Hello {
  rpc Ping(PingRequest) returns (PingReply);
}

message PingRequest {
  string message = 1;
}

message PingReply {
  string message = 1;
}
`

func TestTheEngineFillsTheGrpcRustCellOnly(t *testing.T) {
	generate := NewHandlers().Generate

	if _, err := generate(context.Background(), GenerateInput{Name: "svc", Kind: "rest-api", ProtoSpec: smallProto}); err == nil {
		t.Error("the rest-api kind must be refused")
	}

	if _, err := generate(context.Background(), GenerateInput{Name: "svc", Kind: "grpc", Language: "go", ProtoSpec: smallProto}); err == nil {
		t.Error("the go language must be refused")
	}

	out, err := generate(context.Background(), GenerateInput{Name: "svc", Kind: "grpc", Language: "rust", ProtoSpec: smallProto})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	if len(out.Files) == 0 {
		t.Fatal("no files answered")
	}
}

func TestTheOutputRootsComeFromTheTopLevelOrTheSurface(t *testing.T) {
	out, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "grpc", ProtoSpec: smallProto,
		Surface: map[string]interface{}{"coreDir": "../svc-core", "appDir": "../svc-app"},
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	for _, f := range out.Files {
		if !strings.HasPrefix(f.Path, "../svc-core/") && !strings.HasPrefix(f.Path, "../svc-app/") {
			t.Errorf("%s ignores the surface roots", f.Path)
		}
	}

	out, err = NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "grpc", ProtoSpec: smallProto, CoreDir: "c", AppDir: "a",
		Surface: map[string]interface{}{"coreDir": "../svc-core"},
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	for _, f := range out.Files {
		if !strings.HasPrefix(f.Path, "c/") && !strings.HasPrefix(f.Path, "a/") {
			t.Errorf("%s ignores the top level roots", f.Path)
		}
	}
}
