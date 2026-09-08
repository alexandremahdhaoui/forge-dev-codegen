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

const smallSpec = `module: session
types:
  - name: account
  - name: character
    relations:
      - name: owner
        subjects: [account]
`

func TestTheEngineFillsTheAuthzCellAndTheCombineCellOnly(t *testing.T) {
	generate := NewHandlers().Generate

	_, err := generate(context.Background(), GenerateInput{Name: "svc", Kind: "hexagonal", WiringSpec: smallSpec})
	if err == nil {
		t.Fatal("the hexagonal kind must be refused")
	}

	if !strings.Contains(err.Error(), "authz-gen fills the authz and combine cells only") {
		t.Fatalf("the refusal %q never named both kinds", err)
	}

	out, err := generate(context.Background(), GenerateInput{Name: "svc", Kind: "authz", WiringSpec: smallSpec})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	if len(out.Files) != 3 {
		t.Fatalf("want three files, got %d", len(out.Files))
	}
}

func TestACombineCellWithoutACombineDocumentIsRefusedNamingTheFix(t *testing.T) {
	_, err := NewHandlers().Generate(context.Background(), GenerateInput{Name: "spec", Kind: "combine"})
	if err == nil {
		t.Fatal("an empty combine spec was accepted")
	}

	for _, want := range []string{`emitting the combined model of "spec"`, "combine.yaml", "wiring.specPath"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the refusal %q never named %q", err, want)
		}
	}
}

func TestACombineDocumentThatDoesNotReadIsRefusedWithTheCellNamed(t *testing.T) {
	_, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "spec", Kind: "combine", WiringSpec: "modules: session\n",
	})
	if err == nil {
		t.Fatal("a module list that is a string was accepted")
	}

	for _, want := range []string{`emitting the combined model of "spec"`, "combine.yaml modules: expected a list"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the refusal %q never named %q", err, want)
		}
	}
}

func TestACombineCellReadsItsModulesRelativeToTheCellDirectory(t *testing.T) {
	out, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name:       "demo-authz-combined",
		Kind:       "combine",
		SrcDir:     "../../demo/authz-gen/combined",
		WiringSpec: "modules:\n  - ../session/zz_generated_authz.json\n",
	})
	if err != nil {
		t.Fatalf("combining: %v", err)
	}

	if len(out.Files) != 3 {
		t.Fatalf("want three files, got %d", len(out.Files))
	}
}

func TestEveryAnsweredPathIsZzGeneratedAndStaysInsideTheCellDirectory(t *testing.T) {
	out, err := NewHandlers().Generate(context.Background(), GenerateInput{Name: "svc", Kind: "authz", WiringSpec: smallSpec})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	for _, f := range out.Files {
		if !strings.HasPrefix(f.Path, "zz_generated") {
			t.Errorf("%s is not named zz_generated", f.Path)
		}
	}
}

func TestAModelWithoutAnAuthzDocumentIsRefusedNamingTheFix(t *testing.T) {
	_, err := NewHandlers().Generate(context.Background(), GenerateInput{Name: "svc", Kind: "authz"})
	if err == nil {
		t.Fatal("an empty wiring spec was accepted")
	}

	if !strings.Contains(err.Error(), "wiring.specPath") {
		t.Fatalf("the refusal %q never named wiring.specPath", err)
	}
}

func TestADocumentThatDoesNotValidateIsRefusedWithTheCellNamed(t *testing.T) {
	_, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "authz", WiringSpec: "module: Session\n",
	})
	if err == nil {
		t.Fatal("an upper case module name was accepted")
	}

	for _, want := range []string{`emitting the authz module of "svc"`, `module "Session" is not a lower case identifier`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the refusal %q never named %q", err, want)
		}
	}
}
