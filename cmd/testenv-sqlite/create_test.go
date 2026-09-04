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
	"testing"

	"github.com/alexandremahdhaoui/forge/pkg/engineframework"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/testenvsqlite"
)

const helloSpec = `
components:
  schemas:
    Greeting:
      type: object
      x-store: true
      required: [id, name, count]
`

const helloVectors = `{"cases":[{"case":"c","operation":"createGreeting","controllerReply":{"id":"g1","name":"Songe","count":0}}]}`

func writeProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	if err := os.WriteFile(filepath.Join(root, "hello.yaml"), []byte(helloSpec), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, "cases.json"), []byte(helloVectors), 0o644); err != nil {
		t.Fatal(err)
	}

	return root
}

func TestTheSpecParsesFromAForgeYamlSpecBlockAndRefusesAMissingStoresList(t *testing.T) {
	spec, err := FromMap(map[string]any{"specPath": "a.yaml", "stores": []any{"Greeting"}, "seed": "cases.json", "keep": true})
	if err != nil {
		t.Fatal(err)
	}

	if spec.SpecPath != "a.yaml" || len(spec.Stores) != 1 || spec.Seed != "cases.json" || !spec.Keep {
		t.Errorf("spec: %+v", spec)
	}

	if out := ValidateMap(map[string]any{"specPath": "a.yaml"}); out.Valid {
		t.Error("a spec without stores must be invalid")
	}

	if out := ValidateMap(map[string]any{"stores": []any{"Greeting"}}); out.Valid {
		t.Error("a spec without specPath must be invalid")
	}
}

func TestCreateWritesOneSeededDatabasePerStoreAndExportsItsPath(t *testing.T) {
	writer, err := testenvsqlite.DetectWriter()
	if err != nil {
		t.Skip(err)
	}

	root := writeProject(t)
	tmpDir := t.TempDir()

	input := engineframework.CreateInput{TestID: "t1", Stage: "integration", TmpDir: tmpDir, RootDir: root}
	spec := &Spec{SpecPath: "hello.yaml", Stores: []string{"Greeting"}, Seed: "cases.json"}

	artifact, err := createWith(context.Background(), input, spec, writer)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(tmpDir, "greeting.db")

	if artifact.Files["sqlite.greeting"] != "greeting.db" {
		t.Errorf("files: %v", artifact.Files)
	}

	if artifact.Env["SONGE_STORE_GREETING_PATH"] != path {
		t.Errorf("env: %v", artifact.Env)
	}

	if artifact.Metadata["testenv-sqlite.greeting.rows"] != "1" || artifact.Metadata["testenv-sqlite.writer"] != writer.Name {
		t.Errorf("metadata: %v", artifact.Metadata)
	}

	if len(artifact.ManagedResources) != 1 || artifact.ManagedResources[0] != path {
		t.Errorf("managed: %v", artifact.ManagedResources)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("database file: %v", err)
	}
}

func TestAStoreThatNamesItsPathEnvIsExportedUnderThatNameInsteadOfTheDefault(t *testing.T) {
	writer, err := testenvsqlite.DetectWriter()
	if err != nil {
		t.Skip(err)
	}

	root := writeProject(t)
	tmpDir := t.TempDir()

	input := engineframework.CreateInput{TestID: "t1", Stage: "integration", TmpDir: tmpDir, RootDir: root}
	spec := &Spec{
		SpecPath: "hello.yaml",
		Stores:   []string{"Greeting"},
		Seed:     "cases.json",
		PathEnv:  map[string]string{"Greeting": "SONGE_HELLO_NODE_GREETING_STORE_SQLITE_PATH"},
	}

	artifact, err := createWith(context.Background(), input, spec, writer)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(tmpDir, "greeting.db")

	if artifact.Env["SONGE_HELLO_NODE_GREETING_STORE_SQLITE_PATH"] != path {
		t.Errorf("env: %v", artifact.Env)
	}

	if _, taken := artifact.Env["SONGE_STORE_GREETING_PATH"]; taken {
		t.Errorf("the default name is still exported alongside the named one: %v", artifact.Env)
	}
}

func TestAStoreThatNamesNoPathEnvKeepsTheDefaultName(t *testing.T) {
	writer, err := testenvsqlite.DetectWriter()
	if err != nil {
		t.Skip(err)
	}

	root := writeProject(t)
	tmpDir := t.TempDir()

	input := engineframework.CreateInput{TestID: "t1", Stage: "integration", TmpDir: tmpDir, RootDir: root}
	spec := &Spec{
		SpecPath: "hello.yaml",
		Stores:   []string{"Greeting"},
		Seed:     "cases.json",
		PathEnv:  map[string]string{"Other": "SOMETHING_ELSE"},
	}

	artifact, err := createWith(context.Background(), input, spec, writer)
	if err != nil {
		t.Fatal(err)
	}

	if artifact.Env["SONGE_STORE_GREETING_PATH"] != filepath.Join(tmpDir, "greeting.db") {
		t.Errorf("env: %v", artifact.Env)
	}
}

func TestCreateWithKeepLeavesTheDatabaseOutOfTheManagedResources(t *testing.T) {
	writer, err := testenvsqlite.DetectWriter()
	if err != nil {
		t.Skip(err)
	}

	root := writeProject(t)

	input := engineframework.CreateInput{TestID: "t1", Stage: "integration", TmpDir: t.TempDir(), RootDir: root}
	spec := &Spec{SpecPath: filepath.Join(root, "hello.yaml"), Stores: []string{"Greeting"}, Keep: true}

	artifact, err := createWith(context.Background(), input, spec, writer)
	if err != nil {
		t.Fatal(err)
	}

	if len(artifact.ManagedResources) != 0 || artifact.Metadata["testenv-sqlite.greeting.rows"] != "0" {
		t.Errorf("artifact: %+v", artifact)
	}
}

func TestCreateReportsAMissingDocumentAMissingSeedAndAnUnknownStore(t *testing.T) {
	writer, err := testenvsqlite.DetectWriter()
	if err != nil {
		t.Skip(err)
	}

	root := writeProject(t)
	input := engineframework.CreateInput{TestID: "t1", Stage: "s", TmpDir: t.TempDir(), RootDir: root}

	if _, err := createWith(context.Background(), input, &Spec{SpecPath: "none.yaml", Stores: []string{"Greeting"}}, writer); err == nil {
		t.Error("a missing document must be reported")
	}

	if _, err := createWith(context.Background(), input, &Spec{SpecPath: "hello.yaml", Stores: []string{"Greeting"}, Seed: "none.json"}, writer); err == nil {
		t.Error("a missing seed must be reported")
	}

	if _, err := createWith(context.Background(), input, &Spec{SpecPath: "hello.yaml", Stores: []string{"Nope"}}, writer); err == nil {
		t.Error("an unknown store must be reported")
	}
}

func TestCreateReportsAWriterThatFails(t *testing.T) {
	root := writeProject(t)
	input := engineframework.CreateInput{TestID: "t1", Stage: "s", TmpDir: filepath.Join(t.TempDir(), "missing-dir"), RootDir: root}

	writer, err := testenvsqlite.DetectWriter()
	if err != nil {
		t.Skip(err)
	}

	if _, err := createWith(context.Background(), input, &Spec{SpecPath: "hello.yaml", Stores: []string{"Greeting"}}, writer); err == nil {
		t.Error("a database that cannot be written must be reported")
	}
}

func TestDeleteIsANoOp(t *testing.T) {
	if err := Delete(context.Background(), engineframework.DeleteInput{TestID: "t1"}, nil); err != nil {
		t.Error(err)
	}
}

func TestResolveKeepsAbsolutePathsAndJoinsRelativeOnesToTheRoot(t *testing.T) {
	if got := resolve("/root", "/abs/x"); got != "/abs/x" {
		t.Errorf("absolute: %q", got)
	}

	if got := resolve("/root", "rel/x"); got != "/root/rel/x" {
		t.Errorf("relative: %q", got)
	}

	if got := resolve("", "rel/x"); got != "rel/x" {
		t.Errorf("no root: %q", got)
	}
}
