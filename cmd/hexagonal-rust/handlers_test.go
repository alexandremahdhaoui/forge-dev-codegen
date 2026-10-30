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
      x-store:
        key: id
        lookups: []
        adapters: [sqlite]
      required: [id]
      properties:
        id:
          type: string
`

const smallWiring = `binary: svc-node
ports:
  GreetingStore:
    default: sqlite
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

func generateWithHelpers(t *testing.T, helpers interface{}) (*GenerateOutput, error) {
	t.Helper()

	return NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "hexagonal", SrcDir: standUpTheRestCell(t), WiringSpec: smallWiring,
		Layout: map[string]interface{}{"cells": []interface{}{"rest"}, "helpers": helpers},
	})
}

func helperRefusal(t *testing.T, helpers interface{}, want string) {
	t.Helper()

	_, err := generateWithHelpers(t, helpers)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("want a refusal naming %q, got %v", want, err)
	}
}

func TestADeclaredHelperIsMountedInTheRootControllerLayer(t *testing.T) {
	out, err := generateWithHelpers(t, []interface{}{"presence", "stamps"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	byPath := map[string]string{}
	for _, f := range out.Files {
		byPath[f.Path] = f.Content
	}

	controller := byPath["src/controller/mod.rs"]

	if !strings.Contains(controller, "\npub mod presence;\npub mod stamps;\n") {
		t.Errorf("src/controller/mod.rs never mounts the two helpers in declared order\n%s", controller)
	}

	for _, layer := range []string{"adapter", "driver", "port", "types"} {
		if strings.Contains(byPath["src/"+layer+"/mod.rs"], "presence") {
			t.Errorf("src/%s/mod.rs mounts a controller helper\n%s", layer, byPath["src/"+layer+"/mod.rs"])
		}
	}
}

func TestAHelpersValueThatIsNotAListOfModuleNamesIsRefused(t *testing.T) {
	helperRefusal(t, "presence", "reading helpers: it is a list of module names under src/controller, not presence")
	helperRefusal(t, []interface{}{7}, "reading helpers: it is a list of module names under src/controller, not [7]")
}

func TestAHelperNameRustCannotSpellAsAModuleIsRefused(t *testing.T) {
	helperRefusal(t, []interface{}{"Presence"}, `mounting helper "Presence": it is not a name Rust can spell as a module, use lowercase letters, digits and underscores and start with a letter`)
}

func TestAHelperListedTwiceIsRefused(t *testing.T) {
	helperRefusal(t, []interface{}{"presence", "presence"}, `mounting helper "presence": it is listed twice`)
}

func TestAHelperCarryingTheGeneratedPrefixIsRefusedBecauseThePrefixBelongsToGeneratedFiles(t *testing.T) {
	helperRefusal(t, []interface{}{"zz_generated_presence"}, `mounting helper "zz_generated_presence": the zz_generated prefix belongs to generated files, name the helper for what it shares`)
}

func TestAModelWithNoWiringDocumentIsRefused(t *testing.T) {
	_, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "hexagonal",
	})
	if err == nil || !strings.Contains(err.Error(), "the cell names no wiring file") {
		t.Fatalf("want an error naming the missing wiring, got %v", err)
	}
}
