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

	if out.Files[0].Path != "app/tests/zz_generated_vectors.rs" {
		t.Fatalf("got path %q", out.Files[0].Path)
	}
}

func TestTheAppDirComesFromTheTopLevelOrTheSurface(t *testing.T) {
	out, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "vectors", OpenapiSpec: smallSpec, Vectors: smallVectors,
		Surface: map[string]interface{}{"appDir": "../svc-app"},
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	if out.Files[0].Path != "../svc-app/tests/zz_generated_vectors.rs" {
		t.Fatalf("got path %q", out.Files[0].Path)
	}

	out, err = NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "vectors", OpenapiSpec: smallSpec, Vectors: smallVectors, AppDir: "a",
		Surface: map[string]interface{}{"appDir": "../svc-app"},
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	if out.Files[0].Path != "a/tests/zz_generated_vectors.rs" {
		t.Fatalf("got path %q", out.Files[0].Path)
	}
}
