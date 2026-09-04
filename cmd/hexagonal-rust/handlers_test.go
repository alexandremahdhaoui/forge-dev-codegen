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

func TestTheEngineFillsTheHexagonalRustCellOnly(t *testing.T) {
	generate := NewHandlers().Generate

	if _, err := generate(context.Background(), GenerateInput{Name: "svc", Kind: "cli", OpenapiSpec: smallSpec}); err == nil {
		t.Error("the cli kind must be refused")
	}

	if _, err := generate(context.Background(), GenerateInput{Name: "svc", Kind: "hexagonal", Language: "go", OpenapiSpec: smallSpec}); err == nil {
		t.Error("the go language must be refused")
	}

	out, err := generate(context.Background(), GenerateInput{Name: "svc", Kind: "hexagonal", Language: "rust", OpenapiSpec: smallSpec})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	if len(out.Files) == 0 {
		t.Fatal("no files answered")
	}
}

func TestTheAnswerDeclaresThatItCarriesACellManifest(t *testing.T) {
	out, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "hexagonal", OpenapiSpec: smallSpec,
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	if !out.Manifest {
		t.Error("the answer does not declare a manifest")
	}

	for _, f := range out.Files {
		if f.Path == "zz_generated_cell.yaml" {
			return
		}
	}

	t.Fatal("no cell manifest was answered")
}

func TestTheLayoutMountsEverySecondEnginesCell(t *testing.T) {
	out, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "hexagonal", OpenapiSpec: smallSpec,
		Layout: map[string]interface{}{"cells": []interface{}{"grpc"}},
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	for _, f := range out.Files {
		if f.Path != "src/lib.rs" {
			continue
		}

		if !strings.Contains(f.Content, "pub mod grpc;") {
			t.Errorf("src/lib.rs does not mount the cell\n%s", f.Content)
		}

		return
	}

	t.Fatal("no crate root was answered")
}

func TestALayoutThatMalformsTheCellsListIsRefused(t *testing.T) {
	_, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "hexagonal", OpenapiSpec: smallSpec,
		Layout: map[string]interface{}{"cells": "grpc"},
	})
	if err == nil || !strings.Contains(err.Error(), "it is a list of module directory names under src") {
		t.Fatalf("want an error refusing the cells shape, got %v", err)
	}
}

func TestTheLayoutNamesTheCellTheManifestDeclares(t *testing.T) {
	out, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "hexagonal", OpenapiSpec: smallSpec,
		Layout: map[string]interface{}{"cell": "http"},
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	for _, f := range out.Files {
		if f.Path != "zz_generated_cell.yaml" {
			continue
		}

		if !strings.Contains(f.Content, "cell: http") {
			t.Errorf("the manifest does not name the cell\n%s", f.Content)
		}

		return
	}

	t.Fatal("no cell manifest was answered")
}
