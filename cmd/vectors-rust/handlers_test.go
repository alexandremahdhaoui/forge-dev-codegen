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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const smallSpec = `
paths:
  /greetings:
    post:
      operationId: createGreeting
      x-controller: greeting
      x-ports: [GreetingStore]
      requestBody:
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/Greeting"
      responses:
        "201":
          description: created
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Greeting"
components:
  schemas:
    Greeting:
      type: object
      x-store: true
      required: [id]
      properties:
        id:
          type: string
`

const smallVectors = `{
  "cases": [
    {
      "case": "creating_a_greeting_succeeds",
      "operation": "createGreeting",
      "input": { "id": "g1" },
      "controllerReply": { "id": "g1" },
      "expectedStatus": 201,
      "expectedBody": { "id": "g1" }
    }
  ]
}`

func TestTheEngineFillsTheVectorsCellOnly(t *testing.T) {
	generate := NewHandlers().Generate

	if _, err := generate(context.Background(), GenerateInput{Name: "svc", Kind: "hexagonal", OpenapiSpec: smallSpec, Vectors: smallVectors}); err == nil {
		t.Error("the hexagonal kind must be refused")
	}

	if _, err := generate(context.Background(), GenerateInput{Name: "svc", Kind: "vectors", Language: "go", OpenapiSpec: smallSpec, Vectors: smallVectors}); err == nil {
		t.Error("the go language must be refused")
	}

	out, err := generate(context.Background(), GenerateInput{Name: "svc", Kind: "vectors", Language: "rust", OpenapiSpec: smallSpec, Vectors: smallVectors})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	if len(out.Files) != 1 {
		t.Fatalf("want one file, got %d", len(out.Files))
	}

	if out.Files[0].Path != "tests/zz_generated_vectors.rs" {
		t.Fatalf("got path %q", out.Files[0].Path)
	}
}

func TestTheCrateDirComesFromTheLayout(t *testing.T) {
	out, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "vectors", OpenapiSpec: smallSpec, Vectors: smallVectors,
		Layout: map[string]interface{}{"crateDir": "../svc"},
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	if out.Files[0].Path != "../svc/tests/zz_generated_vectors.rs" {
		t.Fatalf("got path %q", out.Files[0].Path)
	}
}

const callProto = `syntax = "proto3";

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

const callVectors = `{
  "cases": [
    {
      "case": "grpc_ping_comes_back",
      "operation": "grpc_Ping",
      "input": { "message": "songe" },
      "controllerReply": { "message": "songe" }
    }
  ]
}`

func TestTheGrpcProtoComesFromTheFileLayoutGrpcProtoNames(t *testing.T) {
	src := t.TempDir()

	if err := os.WriteFile(filepath.Join(src, "hello.v1.proto"), []byte(callProto), 0o644); err != nil {
		t.Fatalf("writing the proto: %v", err)
	}

	out, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "songe-hello", Kind: "vectors", OpenapiSpec: smallSpec, Vectors: callVectors, SrcDir: src,
		Layout: map[string]interface{}{"grpcProto": "hello.v1.proto", "grpcCell": "grpc"},
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	if !strings.Contains(out.Files[0].Content, "async fn grpc_ping_comes_back() {") {
		t.Fatalf("the grpc vector never reached the file\n%s", out.Files[0].Content)
	}
}

func TestAGrpcProtoPathThatNamesNoFileIsRefusedByName(t *testing.T) {
	_, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "songe-hello", Kind: "vectors", OpenapiSpec: smallSpec, Vectors: callVectors, SrcDir: t.TempDir(),
		Layout: map[string]interface{}{"grpcProto": "missing.proto"},
	})

	want := "reading the document layout.grpcProto names, missing.proto"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("generating reported %v, want %q", err, want)
	}
}

func TestTheRngPortComesFromTheLayout(t *testing.T) {
	_, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "songe-hello", Kind: "vectors", OpenapiSpec: smallSpec, Vectors: smallVectors,
		Layout: map[string]interface{}{"rng": map[string]interface{}{"trait": "Rng", "module": "udp::port::rng"}},
	})

	want := "reading layout.rng: it names the rng port, so it needs trait, module, method and returns"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("generating reported %v, want %q", err, want)
	}
}

func TestAnRngLayoutThatIsNotAMapIsRefused(t *testing.T) {
	_, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "songe-hello", Kind: "vectors", OpenapiSpec: smallSpec, Vectors: smallVectors,
		Layout: map[string]interface{}{"rng": "TickCounter"},
	})

	want := "reading layout.rng: it names the rng port with trait, module, method and returns"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("generating reported %v, want %q", err, want)
	}
}
