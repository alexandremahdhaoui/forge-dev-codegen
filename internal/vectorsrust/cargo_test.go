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

package vectorsrust_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/hexrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/vectorsrust"
)

const cargoWorkspaceManifest = `[workspace]
resolver = "2"
members = ["core", "app"]
`

const cargoCoreManifest = `[package]
name = "songe-hello-core"
version = "0.1.0"
edition = "2021"

[dependencies]
serde = { version = "1", features = ["derive"] }
serde_json = "1"
thiserror = "2"

[dev-dependencies]
mockall = "0.15"
`

const cargoAppManifest = `[package]
name = "songe-hello-app"
version = "0.1.0"
edition = "2021"

[dependencies]
songe-hello-core = { path = "../core" }
anyhow = "1"
axum = "0.8"
rusqlite = { version = "0.40", features = ["bundled"] }
serde = { version = "1", features = ["derive"] }
serde_json = "1"
thiserror = "2"
tokio = { version = "1", features = ["full"] }

[dev-dependencies]
mockall = "0.15"
tower = "0.5"
http-body-util = "0.1"
`

const cargoSpec = `
openapi: 3.1.0
info:
  title: Hello API
  version: 1.0.0
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
              $ref: "#/components/schemas/CreateGreetingRequest"
      responses:
        "201":
          description: The created greeting
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Greeting"
  /greetings/{id}:
    get:
      operationId: getGreeting
      x-controller: greeting
      x-ports: [GreetingStore]
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        "200":
          description: The greeting
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Greeting"
components:
  schemas:
    CreateGreetingRequest:
      type: object
      required: [name]
      properties:
        name:
          type: string
    Greeting:
      type: object
      x-store: true
      required: [id, name, count]
      properties:
        id:
          type: string
        name:
          type: string
        count:
          type: integer
`

const cargoVectors = `{
  "cases": [
    {
      "case": "creating_a_greeting_succeeds",
      "operation": "createGreeting",
      "input": { "name": "Songe" },
      "controllerReply": { "id": "g1", "name": "Songe", "count": 0 },
      "expectedStatus": 201,
      "expectedBody": { "id": "g1", "name": "Songe", "count": 0 }
    },
    {
      "case": "creating_a_greeting_with_an_empty_name_is_refused",
      "operation": "createGreeting",
      "input": { "name": "" },
      "expectedStatus": 400,
      "expectedErrorSubstring": "name"
    },
    {
      "case": "getting_a_known_id_returns_it",
      "operation": "getGreeting",
      "input": { "id": "g1" },
      "controllerReply": { "id": "g1", "name": "Songe", "count": 0 },
      "expectedStatus": 200,
      "expectedBody": { "id": "g1", "name": "Songe", "count": 0 }
    },
    {
      "case": "getting_an_unknown_id_answers_not_found",
      "operation": "getGreeting",
      "input": { "id": "missing" },
      "expectedStatus": 404,
      "expectedErrorSubstring": "not found"
    }
  ]
}`

func TestTheGeneratedVectorsPassAgainstTheGeneratedDriverAndAMockedController(t *testing.T) {
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Skip("cargo is not on PATH")
	}

	hexFiles, err := hexrust.Generate([]byte(cargoSpec), hexrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating the hexagonal skeleton: %v", err)
	}

	vectorFiles, err := vectorsrust.Generate([]byte(cargoSpec), []byte(cargoVectors), vectorsrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating the vectors: %v", err)
	}

	root := t.TempDir()

	write := func(rel, content string) {
		t.Helper()

		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("making %s: %v", filepath.Dir(p), err)
		}

		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", p, err)
		}
	}

	write("Cargo.toml", cargoWorkspaceManifest)
	write("core/Cargo.toml", cargoCoreManifest)
	write("app/Cargo.toml", cargoAppManifest)

	for _, f := range hexFiles {
		write(f.Path, f.Content)
	}

	for _, f := range vectorFiles {
		write(f.Path, f.Content)
	}

	cmd := exec.Command(cargo, "test", "--workspace")
	cmd.Dir = root

	out, err := cmd.CombinedOutput()
	if err != nil {
		lower := strings.ToLower(string(out))
		if strings.Contains(lower, "could not resolve host") ||
			strings.Contains(lower, "failed to get") ||
			strings.Contains(lower, "network") ||
			strings.Contains(lower, "spurious network error") ||
			strings.Contains(lower, "403 forbidden") {
			t.Skipf("cargo test needs network access to crates.io, which this run did not have: %v\n%s", err, out)
		}

		t.Fatalf("cargo test: %v\n%s", err, out)
	}
}
