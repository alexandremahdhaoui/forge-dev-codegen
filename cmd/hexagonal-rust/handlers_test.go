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

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/restrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/cellmanifest"
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

const smallWiring = `binary: svc-node
ports:
  GreetingStore:
    default: sqlite
    adapters:
      sqlite: {}
drivers:
  rest: { enabled: true }
`

func standUpTheRestCell(t *testing.T) string {
	t.Helper()

	files, err := restrust.Generate([]byte(smallSpec), restrust.Options{Service: "svc"})
	if err != nil {
		t.Fatalf("generating the rest cell: %v", err)
	}

	root := t.TempDir()
	dir := filepath.Join(root, "src", "rest")

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("making %s: %v", dir, err)
	}

	if err := os.WriteFile(filepath.Join(dir, "forge-dev.yaml"), []byte("name: svc\nkind: rest\n"), 0o644); err != nil {
		t.Fatalf("writing the cell config: %v", err)
	}

	for _, f := range files {
		if f.Path != cellmanifest.FileName {
			continue
		}

		if err := os.WriteFile(filepath.Join(dir, f.Path), []byte(f.Content), 0o644); err != nil {
			t.Fatalf("writing the cell manifest: %v", err)
		}
	}

	return root
}

func TestTheEngineFillsTheHexagonalRustCellOnly(t *testing.T) {
	generate := NewHandlers().Generate
	root := standUpTheRestCell(t)

	input := GenerateInput{
		Name: "svc", Kind: "hexagonal", Language: "rust",
		SrcDir: root, WiringSpec: smallWiring,
		Layout: map[string]interface{}{"cells": []interface{}{"rest"}},
	}

	cli := input
	cli.Kind = "cli"

	if _, err := generate(context.Background(), cli); err == nil {
		t.Error("the cli kind must be refused")
	}

	golang := input
	golang.Language = "go"

	if _, err := generate(context.Background(), golang); err == nil {
		t.Error("the go language must be refused")
	}

	out, err := generate(context.Background(), input)
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	if len(out.Files) == 0 {
		t.Fatal("no files answered")
	}
}

func TestTheSkeletonAnswersNoCellManifestOfItsOwn(t *testing.T) {
	root := standUpTheRestCell(t)

	out, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "hexagonal", SrcDir: root, WiringSpec: smallWiring,
		Layout: map[string]interface{}{"cells": []interface{}{"rest"}},
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	for _, f := range out.Files {
		if f.Path == cellmanifest.FileName {
			t.Fatal("the skeleton reads manifests, it never writes one")
		}
	}
}

func TestTheLayoutMountsEverySecondEnginesCell(t *testing.T) {
	root := standUpTheRestCell(t)

	out, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "hexagonal", SrcDir: root, WiringSpec: smallWiring,
		Layout: map[string]interface{}{"cells": []interface{}{"rest"}},
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	for _, f := range out.Files {
		if f.Path != "src/lib.rs" {
			continue
		}

		if !strings.Contains(f.Content, "pub mod rest;") {
			t.Errorf("src/lib.rs does not mount the cell\n%s", f.Content)
		}

		return
	}

	t.Fatal("no crate root was answered")
}

func TestALayoutThatMalformsTheCellsListIsRefused(t *testing.T) {
	_, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "hexagonal", WiringSpec: smallWiring,
		Layout: map[string]interface{}{"cells": "rest"},
	})
	if err == nil || !strings.Contains(err.Error(), "it is a list of module directory names under src") {
		t.Fatalf("want an error refusing the cells shape, got %v", err)
	}
}

func TestAModelWithNoWiringDocumentIsRefused(t *testing.T) {
	_, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "hexagonal",
	})
	if err == nil || !strings.Contains(err.Error(), "the cell names no wiring file") {
		t.Fatalf("want an error naming the missing wiring, got %v", err)
	}
}
