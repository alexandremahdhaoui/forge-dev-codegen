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

func TestAHandFileThatExistsIsNeverAnsweredAgain(t *testing.T) {
	srcDir := t.TempDir()
	hand := filepath.Join(srcDir, "core", "src", "hand", "greeting_controller.rs")

	if err := os.MkdirAll(filepath.Dir(hand), 0o755); err != nil {
		t.Fatalf("making the hand dir: %v", err)
	}

	if err := os.WriteFile(hand, []byte("the author's body"), 0o644); err != nil {
		t.Fatalf("writing the hand file: %v", err)
	}

	out, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "hexagonal", OpenapiSpec: smallSpec, SrcDir: srcDir,
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	for _, f := range out.Files {
		if strings.Contains(f.Path, "/hand/") {
			t.Errorf("%s was answered although it exists", f.Path)
		}
	}
}

func TestTheSurfaceMountsEverySecondEnginesCell(t *testing.T) {
	out, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "hexagonal", OpenapiSpec: smallSpec,
		Surface: map[string]interface{}{"cells": []interface{}{"grpc"}},
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	byPath := map[string]string{}
	for _, f := range out.Files {
		byPath[f.Path] = f.Content
	}

	for _, path := range []string{"core/src/lib.rs", "app/src/lib.rs"} {
		if !strings.Contains(byPath[path], "pub mod grpc;") {
			t.Errorf("%s does not mount the cell", path)
		}
	}
}

func TestASurfaceThatMalformsTheCellsListIsRefused(t *testing.T) {
	_, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "hexagonal", OpenapiSpec: smallSpec,
		Surface: map[string]interface{}{"cells": "grpc"},
	})
	if err == nil || !strings.Contains(err.Error(), "it is a list of module directory names under src") {
		t.Fatalf("want an error refusing the cells shape, got %v", err)
	}
}

func TestTheOutputRootsComeFromTheTopLevelOrTheSurface(t *testing.T) {
	out, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "hexagonal", OpenapiSpec: smallSpec,
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
		Name: "svc", Kind: "hexagonal", OpenapiSpec: smallSpec, CoreDir: "c", AppDir: "a",
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
