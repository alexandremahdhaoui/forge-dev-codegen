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

const smallSpec = `controller: board
width: 12
height: 6
tickMs: 100
`

func TestTheEngineFillsTheTuiRustCellOnly(t *testing.T) {
	generate := NewHandlers().Generate

	if _, err := generate(context.Background(), GenerateInput{Name: "svc", Kind: "hexagonal", WiringSpec: smallSpec}); err == nil {
		t.Error("the hexagonal kind must be refused")
	}

	if _, err := generate(context.Background(), GenerateInput{Name: "svc", Kind: "tui", Language: "go", WiringSpec: smallSpec}); err == nil {
		t.Error("the go language must be refused")
	}

	out, err := generate(context.Background(), GenerateInput{Name: "svc", Kind: "tui", Language: "rust", WiringSpec: smallSpec})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	if len(out.Files) == 0 {
		t.Fatal("no files answered")
	}
}

func TestTheAnswerDeclaresThatItCarriesACellManifest(t *testing.T) {
	out, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "tui", WiringSpec: smallSpec,
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

func TestEveryAnsweredPathStaysInsideTheCellDirectory(t *testing.T) {
	out, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "tui", WiringSpec: smallSpec,
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

func TestTheLayoutCellNamesTheModuleTheCrateMounts(t *testing.T) {
	out, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "tui", WiringSpec: smallSpec,
		Layout: map[string]interface{}{"cell": "screen"},
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	for _, f := range out.Files {
		if f.Path != "zz_generated_cell.yaml" {
			continue
		}

		if !strings.Contains(f.Content, "cell: screen") {
			t.Fatalf("the manifest does not name the cell\n%s", f.Content)
		}

		return
	}

	t.Fatal("no cell manifest was answered")
}

func TestASpecThatDoesNotParseIsRefusedWithTheServiceNamed(t *testing.T) {
	_, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "tui", WiringSpec: "controller: board\nwidth: 0\nheight: 6\ntickMs: 100\n",
	})
	if err == nil {
		t.Fatal("a zero width was accepted")
	}

	for _, want := range []string{`emitting the tui cell of "svc"`, "width 0 is outside 1 to 200"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the refusal %q never named %q", err, want)
		}
	}
}
