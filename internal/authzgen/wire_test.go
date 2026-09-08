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

package authzgen_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/authzgen"
)

type wireModel struct {
	SchemaVersion   string                   `json:"schema_version"`
	TypeDefinitions []wireType               `json:"type_definitions"`
	Conditions      map[string]wireCondition `json:"conditions"`
}

type wireType struct {
	Type     string       `json:"type"`
	Metadata wireMetadata `json:"metadata"`
}

type wireMetadata struct {
	Module    string                  `json:"module"`
	Relations map[string]wireRelation `json:"relations"`
}

type wireRelation struct {
	Module string `json:"module"`
}

type wireCondition struct {
	Metadata wireMetadata `json:"metadata"`
}

func decodeWire(t *testing.T, name, content string) wireModel {
	t.Helper()

	var read wireModel

	if err := json.Unmarshal([]byte(content), &read); err != nil {
		t.Fatalf("reading the wire form of %s: %v", name, err)
	}

	return read
}

func generatedFiles(t *testing.T, module string) map[string]string {
	t.Helper()

	files, err := authzgen.Generate([]byte(readDemoFile(t, module, "authz.yaml")))
	if err != nil {
		t.Fatalf("generating %s: %v", module, err)
	}

	byPath := map[string]string{}
	for _, f := range files {
		byPath[f.Path] = f.Content
	}

	return byPath
}

func demoWire(t *testing.T, module string) wireModel {
	t.Helper()

	path := "zz_generated_" + module + ".model.json"

	content, written := generatedFiles(t, module)[path]
	if !written {
		t.Fatalf("the module %s never wrote %s", module, path)
	}

	return decodeWire(t, module, content)
}

func TestEveryDemoModuleWritesItsWireFormBesideItsModuleFileAndItsTests(t *testing.T) {
	for _, module := range []string{"session", "chat", "play"} {
		t.Run(module, func(t *testing.T) {
			files := generatedFiles(t, module)

			for _, path := range []string{
				"zz_generated_" + module + ".fga",
				"zz_generated_" + module + ".fga.yaml",
				"zz_generated_" + module + ".model.json",
				"zz_generated_authz.json",
			} {
				if _, written := files[path]; !written {
					t.Errorf("the module %s never wrote %s", module, path)
				}
			}

			if len(files) != 4 {
				t.Fatalf("the module %s wrote %d files", module, len(files))
			}
		})
	}
}

func TestTheWireFormOfEveryDemoModuleIsTheCommittedGoldenFile(t *testing.T) {
	for _, module := range []string{"session", "chat", "play"} {
		t.Run(module, func(t *testing.T) {
			path := "zz_generated_" + module + ".model.json"

			if got, want := generatedFiles(t, module)[path], readDemoFile(t, module, path); got != want {
				t.Errorf("%s/%s drifted from the committed golden file\n got:\n%s\nwant:\n%s", module, path, got, want)
			}
		})
	}
}

func TestTheWireFormTagsEveryTypeWithTheModuleThatDeclaredIt(t *testing.T) {
	for _, module := range []string{"session", "chat", "play"} {
		t.Run(module, func(t *testing.T) {
			model := demoWire(t, module)

			if len(model.TypeDefinitions) == 0 {
				t.Fatalf("the wire form of %s carries no type", module)
			}

			for _, one := range model.TypeDefinitions {
				if one.Metadata.Module != module {
					t.Errorf("type %q carries the module %q", one.Type, one.Metadata.Module)
				}
			}
		})
	}
}

func TestTheWireFormTagsEveryConditionWithTheModuleThatDeclaredIt(t *testing.T) {
	model := demoWire(t, "play")

	if len(model.Conditions) == 0 {
		t.Fatal("the wire form of play carries no condition")
	}

	for name, one := range model.Conditions {
		if one.Metadata.Module != "play" {
			t.Errorf("condition %q carries the module %q", name, one.Metadata.Module)
		}
	}
}

func TestTheWireFormTagsARelationOnlyWhenItExtendsATypeOfAnotherModule(t *testing.T) {
	byType := map[string]wireType{}
	for _, one := range demoWire(t, "play").TypeDefinitions {
		byType[one.Type] = one
	}

	for name, relation := range byType["monster"].Metadata.Relations {
		if relation.Module != "" {
			t.Errorf("the own relation monster %q carries the module %q, the type already carries it", name, relation.Module)
		}
	}

	extended := byType["session"].Metadata.Relations
	if len(extended) == 0 {
		t.Fatal("the wire form of play extends session with no relation")
	}

	for name, relation := range extended {
		if relation.Module != "play" {
			t.Errorf("the extending relation session %q carries the module %q", name, relation.Module)
		}
	}
}

func TestTheWireFormCarriesTheSchemaVersionThatAModuleFileCannotHold(t *testing.T) {
	for _, module := range []string{"session", "chat", "play"} {
		t.Run(module, func(t *testing.T) {
			if got := demoWire(t, module).SchemaVersion; got != "1.2" {
				t.Errorf("the wire form of %s carries the schema version %q", module, got)
			}
		})
	}
}

func TestTheManifestNamesTheWireFileBesideTheModuleFileAndTheTests(t *testing.T) {
	var read struct {
		Model string `json:"model"`
		Tests string `json:"tests"`
		Wire  string `json:"wire"`
	}

	if err := json.Unmarshal([]byte(generatedFiles(t, "play")["zz_generated_authz.json"]), &read); err != nil {
		t.Fatalf("reading the manifest of play: %v", err)
	}

	if read.Wire != "zz_generated_play.model.json" {
		t.Fatalf("the manifest of play names the wire file %q", read.Wire)
	}
}

func combinedDemoWire(t *testing.T) wireModel {
	t.Helper()

	cell := filepath.Join(demoDir, "combined")

	spec, err := os.ReadFile(filepath.Join(cell, "combine.yaml"))
	if err != nil {
		t.Fatalf("reading the demo combine.yaml: %v", err)
	}

	modules, err := authzgen.ModulesFromSpec(spec)
	if err != nil {
		t.Fatalf("reading the demo combine.yaml: %v", err)
	}

	files, err := authzgen.Combine(authzgen.CombineOptions{
		Name:    "demo-authz-combined",
		SrcDir:  cell,
		Modules: modules,
	})
	if err != nil {
		t.Fatalf("combining the demo: %v", err)
	}

	for _, f := range files {
		if f.Path == "zz_generated_fga.model.json" {
			return decodeWire(t, "the combination", f.Content)
		}
	}

	t.Fatal("the combination never wrote zz_generated_fga.model.json")

	return wireModel{}
}

func TestTheCombinedWireFormHoldsEveryTypeOfEveryModuleExactlyOnce(t *testing.T) {
	seen := map[string]string{}

	for _, one := range combinedDemoWire(t).TypeDefinitions {
		if other, twice := seen[one.Type]; twice {
			t.Fatalf("type %q is written by %q and by %q", one.Type, other, one.Metadata.Module)
		}

		seen[one.Type] = one.Metadata.Module
	}

	for _, want := range []struct{ name, module string }{
		{"account", "session"},
		{"character", "session"},
		{"session", "session"},
		{"channel", "chat"},
		{"monster", "play"},
	} {
		if got := seen[want.name]; got != want.module {
			t.Errorf("type %q carries the module %q", want.name, got)
		}
	}
}

func TestTheCombinedWireFormFoldsAnExtensionIntoTheExtendedTypeAndKeepsTheExtendingModuleOnTheRelation(t *testing.T) {
	byType := map[string]wireType{}
	for _, one := range combinedDemoWire(t).TypeDefinitions {
		byType[one.Type] = one
	}

	relations := byType["session"].Metadata.Relations

	for name, want := range map[string]string{
		"host":       "",
		"member":     "",
		"join":       "",
		"leave":      "",
		"turn_owner": "play",
		"move":       "play",
		"end_turn":   "play",
	} {
		relation, held := relations[name]
		if !held {
			t.Errorf("the combined session type never holds the relation %q", name)

			continue
		}

		if relation.Module != want {
			t.Errorf("the combined session relation %q carries the module %q", name, relation.Module)
		}
	}
}

func TestTheCombinedWireFormCarriesEveryConditionOfEveryModule(t *testing.T) {
	conditions := combinedDemoWire(t).Conditions

	if got := conditions["monster_alive"].Metadata.Module; got != "play" {
		t.Fatalf("the combined condition monster_alive carries the module %q", got)
	}
}

func TestTwoModulesDeclaringOneTypeAreRefusedNamingTheDuplicate(t *testing.T) {
	root := twoCells(t)

	writeModuleCell(t, root, "market", `module: market
types:
  - name: character
    relations:
      - name: seller
        subjects: [character]
vectors:
  - name: a character sells
    tuples:
      - user: character:alice
        relation: seller
        object: character:alice
    checks:
      - user: character:alice
        relation: seller
        object: character:alice
        expected: true
`)

	refusedCombination(t,
		root,
		[]string{
			"../session/zz_generated_authz.json",
			"../chat/zz_generated_authz.json",
			"../market/zz_generated_authz.json",
		},
		"composing the type definitions",
		"duplicate type definition character",
	)
}

func TestAConditionExpressionThatBreaksTheModuleFileIsRefusedNamingTheModule(t *testing.T) {
	_, err := authzgen.Generate([]byte(`module: play
types:
  - name: character
  - name: monster
    relations:
      - name: alive
        subjects: ["character:*"]
        condition: monster_alive
conditions:
  - name: monster_alive
    parameters:
      - name: hp
        type: int
    expression: "}"
vectors:
  - name: a monster is alive
    tuples:
      - user: "character:*"
        relation: alive
        object: monster:boar
        condition:
          name: monster_alive
    checks:
      - user: character:alice
        relation: alive
        object: monster:boar
        context:
          hp: 3
        expected: true
`))
	if err == nil {
		t.Fatal("the module was accepted")
	}

	for _, want := range []string{"the wire form of module", "transforming the module file of"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the refusal %q never named %q", err, want)
		}
	}
}
