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

package restrust_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/restrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/cellmanifest"
)

const helloSpec = `
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

func generate(t *testing.T, opts restrust.Options) map[string]restrust.File {
	t.Helper()

	files, err := restrust.Generate([]byte(helloSpec), opts)
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	byPath := map[string]restrust.File{}
	for _, f := range files {
		byPath[f.Path] = f
	}

	return byPath
}

func TestTheCellEmitsFiveLayersAModFileAndAManifest(t *testing.T) {
	files, err := restrust.Generate([]byte(helloSpec), restrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	got := []string{}
	for _, f := range files {
		got = append(got, f.Path)
	}

	want := []string{
		"adapter/mod.rs",
		"adapter/zz_generated_greeting_sqlite.rs",
		"controller/mod.rs",
		"controller/zz_generated_greeting_controller.rs",
		"driver/mod.rs",
		"driver/zz_generated_http_driver.rs",
		"driver/zz_generated_wire.rs",
		"mod.rs",
		"port/mod.rs",
		"port/zz_generated_greeting_store.rs",
		"types/mod.rs",
		"types/zz_generated_create_greeting_request.rs",
		"types/zz_generated_greeting.rs",
		"zz_generated_cell.yaml",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("emitted paths\n got %q\nwant %q", got, want)
	}
}

func TestEveryModulePathInTheCellClimbsThroughTheCellName(t *testing.T) {
	files := generate(t, restrust.Options{Service: "songe-hello"})

	for _, want := range []string{
		"use crate::rest::types::greeting::Greeting;",
	} {
		if !strings.Contains(files["port/zz_generated_greeting_store.rs"].Content, want) {
			t.Errorf("the port lacks %q\n%s", want, files["port/zz_generated_greeting_store.rs"].Content)
		}
	}

	if !strings.Contains(files["controller/zz_generated_greeting_controller.rs"].Content, "use crate::rest::port::greeting_store::{GreetingStore, GreetingStoreError};") {
		t.Errorf("the controller lacks the cell scoped port import\n%s", files["controller/zz_generated_greeting_controller.rs"].Content)
	}

	if !strings.Contains(files["driver/zz_generated_http_driver.rs"].Content, "use crate::rest::controller::{GreetingController, GreetingControllerError};") {
		t.Errorf("the driver lacks the cell scoped controller import\n%s", files["driver/zz_generated_http_driver.rs"].Content)
	}
}

func TestANamedCellRenamesEveryModulePath(t *testing.T) {
	files := generate(t, restrust.Options{Service: "songe-hello", Cell: "http"})

	if !strings.Contains(files["controller/zz_generated_greeting_controller.rs"].Content, "use crate::http::port::greeting_store::") {
		t.Fatalf("the controller ignored the named cell\n%s", files["controller/zz_generated_greeting_controller.rs"].Content)
	}
}

func TestTheRootModePutsEveryFileUnderSrcAndMountsNoCell(t *testing.T) {
	files, err := restrust.Generate([]byte(helloSpec), restrust.Options{Service: "songe-hello", Root: true})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	for _, f := range files {
		if f.Path == "mod.rs" {
			t.Error("the root mode emitted a cell mod file")
		}

		if strings.HasSuffix(f.Path, ".rs") && !strings.HasPrefix(f.Path, "src/") {
			t.Errorf("%s is outside src", f.Path)
		}

		if strings.Contains(f.Content, "crate::rest::") {
			t.Errorf("%s reaches through a cell name the root mode has none of", f.Path)
		}
	}
}

func TestTheCellManifestNamesTheCellAndItsModules(t *testing.T) {
	files := generate(t, restrust.Options{Service: "songe-hello"})

	m, err := cellmanifest.Parse([]byte(files[cellmanifest.FileName].Content))
	if err != nil {
		t.Fatalf("parsing the manifest: %v", err)
	}

	if m.Cell != "rest" || m.Generator != restrust.Generator {
		t.Errorf("cell = %q, generator = %q", m.Cell, m.Generator)
	}

	if len(m.Provides.Drivers) != 1 || m.Provides.Drivers[0].Module != "rest::driver::http_driver" {
		t.Errorf("drivers = %+v", m.Provides.Drivers)
	}

	if len(m.Provides.Adapters) != 1 || m.Provides.Adapters[0].Module != "rest::adapter::greeting_sqlite" {
		t.Errorf("adapters = %+v", m.Provides.Adapters)
	}

	if len(m.Provides.Controllers) != 1 || m.Provides.Controllers[0].Module != "rest::controller" {
		t.Errorf("controllers = %+v", m.Provides.Controllers)
	}

	if len(m.Provides.Ports) != 1 || m.Provides.Ports[0].Module != "rest::port::greeting_store" {
		t.Errorf("ports = %+v", m.Provides.Ports)
	}
}

func TestACellNameRustCannotSpellIsRefused(t *testing.T) {
	for _, cell := range []string{"Rest", "rest-cell", "mod", "1rest"} {
		if _, err := restrust.Generate([]byte(helloSpec), restrust.Options{Service: "songe-hello", Cell: cell}); err == nil {
			t.Errorf("cell %q was accepted", cell)
		}
	}
}

func TestNamesFollowRustCasing(t *testing.T) {
	tests := []struct {
		in     string
		snake  string
		pascal string
		upper  string
	}{
		{"createGreeting", "create_greeting", "CreateGreeting", "CREATE_GREETING"},
		{"GreetingStore", "greeting_store", "GreetingStore", "GREETING_STORE"},
		{"player-session", "player_session", "PlayerSession", "PLAYER_SESSION"},
		{"songe-hello", "songe_hello", "SongeHello", "SONGE_HELLO"},
		{"HTTPServer", "httpserver", "HTTPServer", "HTTPSERVER"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := restrust.Snake(tt.in); got != tt.snake {
				t.Errorf("Snake(%q) = %q, want %q", tt.in, got, tt.snake)
			}

			if got := restrust.Pascal(tt.in); got != tt.pascal {
				t.Errorf("Pascal(%q) = %q, want %q", tt.in, got, tt.pascal)
			}

			if got := restrust.Upper(tt.in); got != tt.upper {
				t.Errorf("Upper(%q) = %q, want %q", tt.in, got, tt.upper)
			}
		})
	}
}

const cellCrateManifest = `[package]
name = "songe-hello"
version = "0.1.0"
edition = "2021"

[dependencies]
axum = "0.8"
rusqlite = { version = "0.40", features = ["bundled"] }
serde = { version = "1", features = ["derive"] }
serde_json = "1"
thiserror = "2"
tokio = { version = "1", features = ["full"] }

[dev-dependencies]
mockall = "0.15"
`

const cellCrateLib = `pub mod rest;
`

const cellGreetingControllerImpl = `use crate::rest::controller::{
    GreetingController, GreetingControllerError, GreetingControllerImpl,
};
use crate::rest::port::greeting_store::GreetingStore;
use crate::rest::types::create_greeting_request::CreateGreetingRequest;
use crate::rest::types::greeting::Greeting;

impl GreetingController for GreetingControllerImpl {
    fn create_greeting(
        &self,
        body: CreateGreetingRequest,
    ) -> Result<Greeting, GreetingControllerError> {
        let greeting = Greeting {
            count: 0,
            id: body.name.clone(),
            name: body.name,
        };

        self.greeting_store
            .put(greeting.clone())
            .map_err(|source| GreetingControllerError::GreetingStore {
                id: greeting.id.clone(),
                source,
            })?;

        Ok(greeting)
    }

    fn get_greeting(&self, id: &str) -> Result<Greeting, GreetingControllerError> {
        self.greeting_store
            .get(id)
            .map_err(|source| GreetingControllerError::GreetingStore {
                id: id.to_string(),
                source,
            })?
            .ok_or(GreetingControllerError::NotFound { id: id.to_string() })
    }
}
`

func TestTheGeneratedCellCompilesOnceTheUserWritesTheControllerImpl(t *testing.T) {
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Skip("cargo is not on PATH")
	}

	files, err := restrust.Generate([]byte(helloSpec), restrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating: %v", err)
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

	write("Cargo.toml", cellCrateManifest)
	write("src/lib.rs", cellCrateLib)

	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".rs") {
			continue
		}

		write(filepath.Join("src", "rest", f.Path), f.Content)
	}

	write("src/rest/controller/greeting_controller.rs", cellGreetingControllerImpl)

	cmd := exec.Command(cargo, "check", "--workspace", "--all-targets")
	cmd.Dir = root

	out, err := cmd.CombinedOutput()
	if err == nil {
		return
	}

	lower := strings.ToLower(string(out))
	if strings.Contains(lower, "could not resolve host") ||
		strings.Contains(lower, "failed to get") ||
		strings.Contains(lower, "spurious network error") {
		t.Skipf("cargo check needs network access to crates.io, which this run did not have: %v\n%s", err, out)
	}

	t.Fatalf("cargo check: %v\n%s", err, out)
}
