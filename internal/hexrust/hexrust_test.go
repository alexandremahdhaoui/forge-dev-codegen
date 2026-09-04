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

package hexrust_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/hexrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/cellmanifest"
)

const oneStoreOneOperation = `
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
              $ref: "#/components/schemas/Greeting"
      responses:
        "201":
          description: The created greeting
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Greeting"
components:
  schemas:
    Greeting:
      type: object
      x-store: true
      required: [id, name]
      properties:
        id:
          type: string
        name:
          type: string
`

const twoOperationsAndAPathParam = `
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
    delete:
      operationId: deleteGreeting
      x-controller: greeting
      x-ports: [GreetingStore]
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        "204":
          description: Gone
components:
  schemas:
    CreateGreetingRequest:
      type: object
      required: [name]
      properties:
        name:
          type: string
        tags:
          type: array
          items:
            $ref: "#/components/schemas/Tag"
    Tag:
      type: object
      required: [label]
      properties:
        label:
          type: string
        weight:
          type: number
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
        type:
          type: string
        tags:
          type: array
          items:
            $ref: "#/components/schemas/Tag"
`

func TestOneStoreAndOneOperationEmitOneCrateWorthOfLayers(t *testing.T) {
	files, err := hexrust.Generate([]byte(oneStoreOneOperation), hexrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	wantPaths := []string{
		"src/adapter/mod.rs",
		"src/adapter/zz_generated_greeting_sqlite.rs",
		"src/controller/mod.rs",
		"src/controller/zz_generated_greeting_controller.rs",
		"src/driver/mod.rs",
		"src/driver/zz_generated_http_driver.rs",
		"src/driver/zz_generated_wire.rs",
		"src/lib.rs",
		"src/port/mod.rs",
		"src/port/zz_generated_greeting_store.rs",
		"src/types/mod.rs",
		"src/types/zz_generated_greeting.rs",
		"zz_generated_cell.yaml",
	}

	gotPaths := []string{}
	byPath := map[string]hexrust.File{}

	for _, f := range files {
		gotPaths = append(gotPaths, f.Path)
		byPath[f.Path] = f
	}

	if !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Fatalf("emitted paths\n got %q\nwant %q", gotPaths, wantPaths)
	}

	tests := []struct {
		name string
		path string
		want []string
	}{
		{
			name: "the type is a plain serde struct",
			path: "src/types/zz_generated_greeting.rs",
			want: []string{
				"#[derive(Debug, Clone, PartialEq, serde::Serialize, serde::Deserialize)]",
				"pub struct Greeting {",
				"    pub id: String,",
			},
		},
		{
			name: "the port is a mockable trait with put and get",
			path: "src/port/zz_generated_greeting_store.rs",
			want: []string{
				"#[cfg_attr(test, mockall::automock)]",
				"pub trait GreetingStore: Send + Sync {",
				"    fn put(&self, v: Greeting) -> Result<(), GreetingStoreError>;",
				"    fn get(&self, id: &str) -> Result<Option<Greeting>, GreetingStoreError>;",
			},
		},
		{
			name: "the controller emits the trait, the struct holding boxed ports and new",
			path: "src/controller/zz_generated_greeting_controller.rs",
			want: []string{
				"pub enum GreetingControllerError {",
				"        #[source]",
				"        source: GreetingStoreError,",
				"pub trait GreetingController: Send + Sync {",
				"    fn create_greeting(&self, body: Greeting) -> Result<Greeting, GreetingControllerError>;",
				"pub struct GreetingControllerImpl {",
				"    pub(crate) greeting_store: Arc<dyn GreetingStore + Send + Sync>,",
				"    pub fn new(greeting_store: Arc<dyn GreetingStore + Send + Sync>) -> Self {",
			},
		},
		{
			name: "the controller layer mounts the user impl file and re exports the trait and the struct",
			path: "src/controller/mod.rs",
			want: []string{
				"pub mod zz_generated_greeting_controller;",
				"mod greeting_controller;",
				"pub use zz_generated_greeting_controller::{GreetingController, GreetingControllerError, GreetingControllerImpl};",
			},
		},
		{
			name: "the sqlite adapter takes a config and owns the table and the audit table",
			path: "src/adapter/zz_generated_greeting_sqlite.rs",
			want: []string{
				"pub struct GreetingSqliteStoreConfig {",
				"    pub path: String,",
				"pub struct GreetingSqliteStore {",
				"    pub fn new(config: GreetingSqliteStoreConfig) -> Result<Self, GreetingSqliteError> {",
				"CREATE TABLE IF NOT EXISTS greeting (id TEXT PRIMARY KEY, body TEXT NOT NULL);",
				"CREATE TABLE IF NOT EXISTS audit (at TEXT NOT NULL, table_name TEXT NOT NULL, key TEXT NOT NULL, op TEXT NOT NULL, before TEXT, after TEXT);",
				"impl GreetingStore for GreetingSqliteStore {",
			},
		},
		{
			name: "the driver takes a config, binds, announces and serves",
			path: "src/driver/zz_generated_http_driver.rs",
			want: []string{
				"    pub(crate) greeting_controller: Arc<dyn GreetingController + Send + Sync>,",
				`            .route("/greetings", routing::post(create_greeting))`,
				"        .create_greeting(body.into())",
				"    Ok((StatusCode::CREATED, Json(out.into())))",
				"    pub async fn bind(&mut self) -> Result<(), HttpDriverError> {",
				`        println!("LISTENING {}", self.local_port()?);`,
				"    pub async fn serve(self) -> Result<(), HttpDriverError> {",
			},
		},
		{
			name: "the adapter and the driver layers allow the io lint table",
			path: "src/adapter/mod.rs",
			want: []string{
				"#![allow(clippy::disallowed_methods, clippy::disallowed_types)]",
			},
		},
		{
			name: "lib mounts every layer as a plain module directory",
			path: "src/lib.rs",
			want: []string{
				"pub mod adapter;",
				"pub mod controller;",
				"pub mod driver;",
				"pub mod port;",
				"pub mod types;",
			},
		},
		{
			name: "a layer mod file names the generated file and aliases it to the layer name",
			path: "src/types/mod.rs",
			want: []string{
				"pub mod zz_generated_greeting;",
				"pub use zz_generated_greeting as greeting;",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := byPath[tt.path].Content

			for _, line := range tt.want {
				if !strings.Contains(content, line) {
					t.Errorf("%s lacks %q\n%s", tt.path, line, content)
				}
			}
		})
	}
}

func TestTheCellManifestNamesEveryDriverAdapterControllerAndPort(t *testing.T) {
	files, err := hexrust.Generate([]byte(oneStoreOneOperation), hexrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	var body string

	for _, f := range files {
		if f.Path == cellmanifest.FileName {
			body = f.Content
		}
	}

	if body == "" {
		t.Fatal("no cell manifest was emitted")
	}

	m, err := cellmanifest.Parse([]byte(body))
	if err != nil {
		t.Fatalf("parsing the manifest: %v\n%s", err, body)
	}

	if m.Cell != "rest" {
		t.Errorf("cell = %q, want rest", m.Cell)
	}

	if len(m.Provides.Drivers) != 1 || m.Provides.Drivers[0].Name != "rest" {
		t.Fatalf("drivers = %+v", m.Provides.Drivers)
	}

	driver := m.Provides.Drivers[0]
	if driver.Type != "HttpDriver" || driver.Module != "driver::http_driver" {
		t.Errorf("driver = %+v", driver)
	}

	if !reflect.DeepEqual(driver.Requires, []string{"GreetingController"}) {
		t.Errorf("driver requires = %+v", driver.Requires)
	}

	if driver.Config["addr"].Type != cellmanifest.FieldTypeString {
		t.Errorf("driver config = %+v", driver.Config)
	}

	if len(m.Provides.Adapters) != 1 || m.Provides.Adapters[0].Name != "sqlite" {
		t.Fatalf("adapters = %+v", m.Provides.Adapters)
	}

	adapter := m.Provides.Adapters[0]
	if adapter.Implements != "GreetingStore" || adapter.Type != "GreetingSqliteStore" {
		t.Errorf("adapter = %+v", adapter)
	}

	if adapter.Config["path"].Default != ":memory:" {
		t.Errorf("adapter config = %+v", adapter.Config)
	}

	if len(m.Provides.Controllers) != 1 {
		t.Fatalf("controllers = %+v", m.Provides.Controllers)
	}

	controller := m.Provides.Controllers[0]
	if controller.Trait != "GreetingController" || controller.Impl != "GreetingControllerImpl" || controller.Module != "controller" {
		t.Errorf("controller = %+v", controller)
	}

	if !reflect.DeepEqual(controller.Ports, []string{"GreetingStore"}) {
		t.Errorf("controller ports = %+v", controller.Ports)
	}

	if len(m.Provides.Ports) != 1 || m.Provides.Ports[0].Trait != "GreetingStore" {
		t.Errorf("ports = %+v", m.Provides.Ports)
	}
}

func TestTwoStoresNameTheirOwnSqliteAdapterSoOneMergeNeverCollides(t *testing.T) {
	doc := strings.Replace(twoOperationsAndAPathParam, `    Tag:
      type: object
      required: [label]`, `    Tag:
      type: object
      x-store: true
      required: [id, label]`, 1)
	doc = strings.Replace(doc, `      properties:
        label:
          type: string`, `      properties:
        id:
          type: string
        label:
          type: string`, 1)

	files, err := hexrust.Generate([]byte(doc), hexrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	for _, f := range files {
		if f.Path != cellmanifest.FileName {
			continue
		}

		m, err := cellmanifest.Parse([]byte(f.Content))
		if err != nil {
			t.Fatalf("parsing the manifest: %v\n%s", err, f.Content)
		}

		names := []string{}
		for _, a := range m.Provides.Adapters {
			names = append(names, a.Name)
		}

		if !reflect.DeepEqual(names, []string{"greeting_sqlite", "tag_sqlite"}) {
			t.Fatalf("adapter names = %+v", names)
		}

		return
	}

	t.Fatal("no cell manifest was emitted")
}

func TestTheSpecSchemaBelongsToForgeDevAndNeverBecomesAType(t *testing.T) {
	doc := oneStoreOneOperation + `    Spec:
      type: object
      properties:
        note:
          type: string
`

	files, err := hexrust.Generate([]byte(doc), hexrust.Options{Service: "svc"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	for _, f := range files {
		if strings.HasSuffix(f.Path, "zz_generated_spec.rs") {
			t.Errorf("%s was emitted for the forge-dev Spec schema", f.Path)
		}
	}
}

func TestEveryEmittedRustFileCarriesTheGeneratedHeader(t *testing.T) {
	files, err := hexrust.Generate([]byte(oneStoreOneOperation), hexrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	const header = "// Code generated by hexagonal-rust (forge-dev-codegen). DO NOT EDIT.\n"

	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".rs") {
			continue
		}

		if !strings.HasPrefix(f.Content, header) {
			t.Errorf("%s does not open with the generated header", f.Path)
		}
	}
}

func TestNoEmittedFileIsAHandFileOrAWriteOnceFile(t *testing.T) {
	files, err := hexrust.Generate([]byte(oneStoreOneOperation), hexrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	for _, f := range files {
		if strings.Contains(f.Path, "hand") {
			t.Errorf("%s belongs to a hand directory, which is gone", f.Path)
		}
	}
}

func TestLibRsMountsEveryCellAsAPlainModuleDirectoryUnderSrc(t *testing.T) {
	files, err := hexrust.Generate([]byte(oneStoreOneOperation), hexrust.Options{
		Service: "songe-hello",
		Cells:   []string{"udp", "grpc"},
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	for _, f := range files {
		if f.Path != "src/lib.rs" {
			continue
		}

		for _, want := range []string{"pub mod grpc;", "pub mod udp;"} {
			if !strings.Contains(f.Content, want) {
				t.Errorf("src/lib.rs lacks %q\n%s", want, f.Content)
			}
		}

		if strings.Contains(f.Content, `#[path = "grpc`) {
			t.Errorf("src/lib.rs mounts a cell with a path attribute\n%s", f.Content)
		}

		return
	}

	t.Fatal("no src/lib.rs was emitted")
}

func TestACellNameRustCannotSpellIsRefused(t *testing.T) {
	tests := []struct {
		name  string
		cells []string
		want  string
	}{
		{
			name:  "a capital letter is not a module name",
			cells: []string{"Grpc"},
			want:  "is not a name Rust can spell as a module",
		},
		{
			name:  "a dash is not a module name",
			cells: []string{"grpc-cell"},
			want:  "is not a name Rust can spell as a module",
		},
		{
			name:  "a keyword is not a module name",
			cells: []string{"mod"},
			want:  "is not a name Rust can spell as a module",
		},
		{
			name:  "a layer the crate root already owns cannot be a cell",
			cells: []string{"driver"},
			want:  "the crate root already owns that module",
		},
		{
			name:  "the config module cannot be a cell",
			cells: []string{"config"},
			want:  "the crate root already owns that module",
		},
		{
			name:  "the same cell twice is a mistake",
			cells: []string{"grpc", "grpc"},
			want:  "it is listed twice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := hexrust.Generate([]byte(oneStoreOneOperation), hexrust.Options{
				Service: "songe-hello",
				Cells:   tt.cells,
			})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want an error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestTheLayoutCarriesTheCellsList(t *testing.T) {
	got, err := hexrust.CellsFromLayout(map[string]interface{}{
		"cells": []interface{}{"grpc", "udp"},
	})
	if err != nil {
		t.Fatalf("reading the layout: %v", err)
	}

	want := []string{"grpc", "udp"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cells\n got %+v\nwant %+v", got, want)
	}
}

func TestALayoutThatMalformsTheCellsListIsRefused(t *testing.T) {
	tests := []struct {
		name   string
		layout map[string]interface{}
		want   string
	}{
		{
			name:   "a list is required",
			layout: map[string]interface{}{"cells": "grpc"},
			want:   "it is a list of module directory names under src",
		},
		{
			name:   "an entry is a name",
			layout: map[string]interface{}{"cells": []interface{}{map[string]interface{}{"name": "grpc"}}},
			want:   "it is a name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := hexrust.CellsFromLayout(tt.layout); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want an error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestNoGeneratedFileCarriesAPathAttribute(t *testing.T) {
	files, err := hexrust.Generate([]byte(twoOperationsAndAPathParam), hexrust.Options{
		Service: "songe-hello",
		Cells:   []string{"grpc"},
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	for _, f := range files {
		if strings.Contains(f.Content, "#[path") {
			t.Errorf("%s mounts a module with a path attribute\n%s", f.Path, f.Content)
		}
	}
}

func TestALayoutWithNoCellsMountsNothingExtra(t *testing.T) {
	got, err := hexrust.CellsFromLayout(map[string]interface{}{})
	if err != nil {
		t.Fatalf("reading the layout: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("want no cells, got %+v", got)
	}
}

func TestASpecThatBreaksTheMappingIsRefused(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want string
	}{
		{
			name: "an operation without x-controller has no home",
			doc: `
paths:
  /a:
    get:
      operationId: a
      responses:
        "200":
          description: ok
components:
  schemas: {}
`,
			want: "x-controller is required",
		},
		{
			name: "a port must name an x-store schema",
			doc: `
paths:
  /a:
    get:
      operationId: a
      x-controller: a
      x-ports: [NopeStore]
      responses:
        "200":
          description: ok
components:
  schemas: {}
`,
			want: `x-ports names "NopeStore"`,
		},
		{
			name: "a store needs a string id",
			doc: `
paths:
  /a:
    get:
      operationId: a
      x-controller: a
      responses:
        "200":
          description: ok
components:
  schemas:
    Thing:
      type: object
      x-store: true
      properties:
        name:
          type: string
`,
			want: "required string property named id",
		},
		{
			name: "a path parameter must be declared",
			doc: `
paths:
  /a/{id}:
    get:
      operationId: a
      x-controller: a
      responses:
        "200":
          description: ok
components:
  schemas: {}
`,
			want: `path parameter "id" is not declared`,
		},
		{
			name: "a 2xx response is required",
			doc: `
paths:
  /a:
    get:
      operationId: a
      x-controller: a
      responses:
        "500":
          description: boom
components:
  schemas: {}
`,
			want: "a 2xx response is required",
		},
		{
			name: "the service name is required",
			doc:  oneStoreOneOperation,
			want: "the service name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := hexrust.Options{Service: "svc"}
			if tt.want == "the service name is required" {
				opts.Service = ""
			}

			_, err := hexrust.Generate([]byte(tt.doc), opts)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want an error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestANameRustOrSQLCannotSpellIsRefusedBeforeItReachesTheOutput(t *testing.T) {
	withParam := func(name string) string {
		doc := strings.ReplaceAll(twoOperationsAndAPathParam, "{id}", "{"+name+"}")

		return strings.ReplaceAll(doc, "- name: id\n", "- name: "+name+"\n")
	}

	tests := []struct {
		name string
		doc  string
		opts hexrust.Options
		want string
	}{
		{
			name: "a service name with a quote never reaches main or the crate name",
			doc:  oneStoreOneOperation,
			opts: hexrust.Options{Service: `svc"; DROP TABLE greeting; --`},
			want: `reading service "svc\"; DROP TABLE greeting; --"`,
		},
		{
			name: "a schema name with a semicolon cannot be a struct or a table",
			doc:  strings.ReplaceAll(oneStoreOneOperation, "Greeting", "Greet;ing"),
			opts: hexrust.Options{Service: "svc"},
			want: `reading schema "Greet;ing"`,
		},
		{
			name: "a schema name starting with a digit cannot be a struct",
			doc:  strings.ReplaceAll(oneStoreOneOperation, "Greeting", "1Greeting"),
			opts: hexrust.Options{Service: "svc"},
			want: `reading schema "1Greeting"`,
		},
		{
			name: "a property name with a quote cannot be a field",
			doc:  strings.Replace(oneStoreOneOperation, "        name:\n", "        \"na'me\":\n", 1),
			opts: hexrust.Options{Service: "svc"},
			want: `reading property "na'me"`,
		},
		{
			name: "an operationId with a semicolon cannot be a method",
			doc:  strings.Replace(oneStoreOneOperation, "createGreeting", "create;Greeting", 1),
			opts: hexrust.Options{Service: "svc"},
			want: `reading operationId "create;Greeting"`,
		},
		{
			name: "an x-controller with a slash cannot be a module",
			doc:  strings.Replace(oneStoreOneOperation, "x-controller: greeting", "x-controller: greet/ing", 1),
			opts: hexrust.Options{Service: "svc"},
			want: `reading x-controller "greet/ing"`,
		},
		{
			name: "a path parameter with a parenthesis cannot be an extractor binding",
			doc:  withParam("i(d)"),
			opts: hexrust.Options{Service: "svc"},
			want: `reading path parameter "i(d)"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := hexrust.Generate([]byte(tt.doc), tt.opts)
			if err == nil || !strings.Contains(err.Error(), tt.want) || !strings.Contains(err.Error(), "Rust or SQL can spell") {
				t.Fatalf("want an error naming %s, got %v", tt.want, err)
			}
		})
	}
}

func TestTheDriverAnswersTheStatusTheSpecDeclaresForAnInvalidRequest(t *testing.T) {
	with422 := strings.Replace(oneStoreOneOperation, "      responses:\n", "      responses:\n        \"422\":\n          description: refused\n", 1)
	with400 := strings.Replace(oneStoreOneOperation, "      responses:\n", "      responses:\n        \"400\":\n          description: refused\n", 1)
	withBoth := strings.Replace(with422, "      responses:\n", "      responses:\n        \"400\":\n          description: refused\n", 1)

	tests := []struct {
		name string
		doc  string
		want string
	}{
		{"a declared 422 answers 422", with422, "reject_greeting(error, StatusCode::UNPROCESSABLE_ENTITY)"},
		{"a declared 400 answers 400", with400, "reject_greeting(error, StatusCode::BAD_REQUEST)"},
		{"422 wins when both are declared", withBoth, "reject_greeting(error, StatusCode::UNPROCESSABLE_ENTITY)"},
		{"no declared 4xx falls back to 400", oneStoreOneOperation, "reject_greeting(error, StatusCode::BAD_REQUEST)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := hexrust.Generate([]byte(tt.doc), hexrust.Options{Service: "svc"})
			if err != nil {
				t.Fatalf("generating: %v", err)
			}

			for _, f := range files {
				if strings.HasSuffix(f.Path, "zz_generated_http_driver.rs") && !strings.Contains(f.Content, tt.want) {
					t.Errorf("the driver lacks %q\n%s", tt.want, f.Content)
				}
			}
		})
	}
}

const crateManifest = `[package]
name = "songe-hello"
version = "0.1.0"
edition = "2021"

[dependencies]
anyhow = "1"
axum = "0.8"
rusqlite = { version = "0.40", features = ["bundled"] }
serde = { version = "1", features = ["derive"] }
serde_json = "1"
thiserror = "2"
tokio = { version = "1", features = ["full"] }

[dev-dependencies]
mockall = "0.15"
`

const greetingControllerImpl = `use crate::controller::{GreetingController, GreetingControllerError, GreetingControllerImpl};
use crate::port::greeting_store::GreetingStore;
use crate::types::create_greeting_request::CreateGreetingRequest;
use crate::types::greeting::Greeting;

impl GreetingController for GreetingControllerImpl {
    fn create_greeting(
        &self,
        body: CreateGreetingRequest,
    ) -> Result<Greeting, GreetingControllerError> {
        let greeting = Greeting {
            count: 0,
            id: body.name.clone(),
            name: body.name,
            tags: body.tags,
            r#type: None,
        };

        self.greeting_store
            .put(greeting.clone())
            .map_err(|source| GreetingControllerError::GreetingStore {
                id: greeting.id.clone(),
                source,
            })?;

        Ok(greeting)
    }

    fn delete_greeting(&self, id: &str) -> Result<(), GreetingControllerError> {
        let _ = id;

        Ok(())
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

func writeUnder(t *testing.T, root string) func(rel, content string) {
	t.Helper()

	return func(rel, content string) {
		t.Helper()

		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("making %s: %v", filepath.Dir(p), err)
		}

		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", p, err)
		}
	}
}

func TestTheGeneratedCrateCompilesOnceTheUserWritesTheControllerImpl(t *testing.T) {
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Skip("cargo is not on PATH")
	}

	files, err := hexrust.Generate([]byte(twoOperationsAndAPathParam), hexrust.Options{
		Service: "songe-hello",
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	root := t.TempDir()
	write := writeUnder(t, root)

	write("Cargo.toml", crateManifest)

	for _, f := range files {
		if strings.HasSuffix(f.Path, ".rs") {
			write(f.Path, f.Content)
		}
	}

	write("src/controller/greeting_controller.rs", greetingControllerImpl)

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
