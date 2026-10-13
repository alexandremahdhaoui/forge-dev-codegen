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
      x-store:
        key: id
        lookups: []
        adapters: [sqlite]
      required: [id]
      properties:
        id:
          type: string
`

const noCases = `{"cases":[]}`

const greetingCases = `{"cases":[
  {"case":"create_a_greeting","operation":"createGreeting","controllerReply":{"id":"g\"1"}},
  {"case":"create_a_refused_greeting","operation":"createGreeting","controllerReply":null}
]}`

func seeded(t *testing.T, cases string) GenerateInput {
	t.Helper()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "cell"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "cases.json"), []byte(cases), 0o644); err != nil {
		t.Fatal(err)
	}

	return GenerateInput{
		Name:        "svc",
		Kind:        "rest",
		OpenapiSpec: smallSpec,
		SrcDir:      filepath.Join(dir, "cell"),
		Layout:      map[string]interface{}{"vectors": "../cases.json"},
	}
}

func sqliteAdapter(t *testing.T, out *GenerateOutput) string {
	t.Helper()

	for _, f := range out.Files {
		if f.Path == "adapter/zz_generated_greeting_sqlite.rs" {
			return f.Content
		}
	}

	t.Fatal("no greeting sqlite adapter was answered")

	return ""
}

func TestASqliteStoreInACellWhoseLayoutNamesNoVectorsIsRefusedNamingTheCellAndTheKey(t *testing.T) {
	input := seeded(t, noCases)
	delete(input.Layout, "vectors")

	_, err := NewHandlers().Generate(context.Background(), input)
	if err == nil {
		t.Fatal("a sqlite store with no layout.vectors must be refused")
	}

	for _, name := range []string{`rest cell "rest"`, "layout.vectors", "Greeting"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("the refusal names no %s: %v", name, err)
		}
	}
}

func TestALayoutVectorsFileThatCannotBeReadIsRefusedNamingThePath(t *testing.T) {
	input := seeded(t, noCases)
	input.Layout["vectors"] = "../missing.json"

	_, err := NewHandlers().Generate(context.Background(), input)
	if err == nil || !strings.Contains(err.Error(), "../missing.json") {
		t.Fatalf("an unreadable layout.vectors must be refused naming its path: %v", err)
	}
}

func TestARestCellRefusesACreateCaseAnsweringARowOfNoDeclaredStoreNamingTheCaseTheOperationAndTheStores(t *testing.T) {
	input := seeded(t, `{"cases":[{"case":"stray","operation":"createOther","controllerReply":{"other":"x"}}]}`)

	_, err := NewHandlers().Generate(context.Background(), input)
	if err == nil {
		t.Fatal("a create case matching no store must be refused")
	}

	for _, name := range []string{`"stray"`, "createOther", "the stores are Greeting"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("the refusal names no %s: %v", name, err)
		}
	}
}

func TestTheSqliteAdapterBakesEveryCreateCaseRowAsASeedConstantAndSkipsAnErrorCase(t *testing.T) {
	out, err := NewHandlers().Generate(context.Background(), seeded(t, greetingCases))
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	adapter := sqliteAdapter(t, out)

	want := "const SEEDS: &[(&str, &str)] = &[\n    (\"g\\\"1\", \"{\\\"id\\\":\\\"g\\\\\\\"1\\\"}\"),\n];"
	if !strings.Contains(adapter, want) {
		t.Fatalf("the adapter carries no seed constant %s\n%s", want, adapter)
	}

	if !strings.Contains(adapter, `"INSERT INTO \"greeting\" (\"id\", body) VALUES (?1, ?2)"`) {
		t.Errorf("the seed insert is not a plain INSERT that lets sqlite raise on a duplicate\n%s", adapter)
	}

	if !strings.Contains(adapter, "'greeting', ?1, 'seed', NULL, ?2)") {
		t.Errorf("the seed writes no seed audit row\n%s", adapter)
	}
}

func TestAStoreWithNoCreateCaseCarriesAnEmptySeedConstantAndTheSameSeedCode(t *testing.T) {
	out, err := NewHandlers().Generate(context.Background(), seeded(t, noCases))
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	adapter := sqliteAdapter(t, out)

	for _, want := range []string{"const SEEDS: &[(&str, &str)] = &[\n];", "seed(&mut connection, &path)?;", "for &(key, body) in SEEDS {"} {
		if !strings.Contains(adapter, want) {
			t.Errorf("the adapter carries no %s\n%s", want, adapter)
		}
	}
}

func TestTheEngineFillsTheRestRustCellOnly(t *testing.T) {
	generate := NewHandlers().Generate

	if _, err := generate(context.Background(), GenerateInput{Name: "svc", Kind: "hexagonal", OpenapiSpec: smallSpec}); err == nil {
		t.Error("the hexagonal kind must be refused")
	}

	if _, err := generate(context.Background(), GenerateInput{Name: "svc", Kind: "rest", Language: "go", OpenapiSpec: smallSpec}); err == nil {
		t.Error("the go language must be refused")
	}

	input := seeded(t, noCases)
	input.Language = "rust"

	out, err := generate(context.Background(), input)
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	if len(out.Files) == 0 {
		t.Fatal("no files answered")
	}
}

func TestTheAnswerDeclaresThatItCarriesACellManifest(t *testing.T) {
	out, err := NewHandlers().Generate(context.Background(), seeded(t, noCases))
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
	out, err := NewHandlers().Generate(context.Background(), seeded(t, noCases))
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
	input := seeded(t, noCases)
	input.Layout["cell"] = "http"

	out, err := NewHandlers().Generate(context.Background(), input)
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	for _, f := range out.Files {
		if f.Path != "zz_generated_cell.yaml" {
			continue
		}

		if !strings.Contains(f.Content, "cell: http") {
			t.Fatalf("the manifest does not name the cell\n%s", f.Content)
		}

		return
	}

	t.Fatal("no cell manifest was answered")
}
