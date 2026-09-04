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

func TestTheLayoutCellNamesTheModuleTheCrateMounts(t *testing.T) {
	out, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "grpc", ProtoSpec: smallProto,
		Layout: map[string]interface{}{"cell": "wire"},
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	for _, f := range out.Files {
		if f.Path != "zz_generated_build.rs" {
			continue
		}

		if !strings.Contains(f.Content, "src/wire/proto") {
			t.Fatalf("the build script does not read the cell name\n%s", f.Content)
		}
	}
}

func TestEveryAnsweredPathStaysInsideTheCellDirectory(t *testing.T) {
	out, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "grpc", ProtoSpec: smallProto,
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	for _, f := range out.Files {
		if strings.HasPrefix(f.Path, "..") || strings.HasPrefix(f.Path, "/") {
			t.Errorf("%s reaches above the cell directory", f.Path)
		}
	}
}

func TestTheCellHoldsOneCrateWorthOfLayersAndNoHandDirectory(t *testing.T) {
	out, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "grpc", ProtoSpec: smallProto,
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	got := map[string]bool{}
	for _, f := range out.Files {
		got[f.Path] = true
	}

	for _, path := range []string{
		"adapter/zz_generated_hello_grpc_client.rs",
		"controller/zz_generated_hello_controller.rs",
		"driver/zz_generated_hello_grpc_driver.rs",
		"port/zz_generated_hello_client.rs",
		"types/zz_generated_hello_messages.rs",
		"zz_generated_build.rs",
		"zz_generated_cell.yaml",
	} {
		if !got[path] {
			t.Errorf("the cell did not answer %s", path)
		}
	}

	for _, f := range out.Files {
		if strings.Contains(f.Path, "hand") {
			t.Errorf("%s belongs to a hand directory, which is gone", f.Path)
		}
	}
}

func TestTheAnswerDeclaresThatItCarriesACellManifest(t *testing.T) {
	out, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "grpc", ProtoSpec: smallProto,
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	if !out.Manifest {
		t.Error("the answer does not declare a manifest")
	}
}
