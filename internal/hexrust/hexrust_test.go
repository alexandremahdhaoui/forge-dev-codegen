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

func TestOneStoreAndOneOperationEmitTheWholeSkeleton(t *testing.T) {
	files, err := hexrust.Generate([]byte(oneStoreOneOperation), hexrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	wantPaths := []string{
		"app/src/adapter/zz_generated_greeting_sqlite.rs",
		"app/src/adapter/zz_generated_mod.rs",
		"app/src/bin/songe-hello-server.rs",
		"app/src/driver/zz_generated_http_driver.rs",
		"app/src/driver/zz_generated_mod.rs",
		"app/src/driver/zz_generated_wire.rs",
		"app/src/lib.rs",
		"core/src/controller/zz_generated_greeting_controller.rs",
		"core/src/controller/zz_generated_mod.rs",
		"core/src/hand/greeting_controller.rs",
		"core/src/lib.rs",
		"core/src/port/zz_generated_greeting_store.rs",
		"core/src/port/zz_generated_mod.rs",
		"core/src/types/zz_generated_greeting.rs",
		"core/src/types/zz_generated_mod.rs",
		"core/src/zz_generated_hand.rs",
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
			path: "core/src/types/zz_generated_greeting.rs",
			want: []string{
				"#[derive(Debug, Clone, PartialEq, serde::Serialize, serde::Deserialize)]",
				"pub struct Greeting {",
				"    pub id: String,",
			},
		},
		{
			name: "the port is a mockable trait with put and get",
			path: "core/src/port/zz_generated_greeting_store.rs",
			want: []string{
				"#[cfg_attr(test, mockall::automock)]",
				"pub trait GreetingStore: Send + Sync {",
				"    fn put(&self, v: Greeting) -> Result<(), GreetingStoreError>;",
				"    fn get(&self, id: &str) -> Result<Option<Greeting>, GreetingStoreError>;",
			},
		},
		{
			name: "the controller wraps the store error with source and delegates to hand",
			path: "core/src/controller/zz_generated_greeting_controller.rs",
			want: []string{
				"pub enum GreetingControllerError {",
				"        #[source]",
				"        source: GreetingStoreError,",
				"pub trait GreetingController: Send + Sync {",
				"    fn create_greeting(&self, body: Greeting) -> Result<Greeting, GreetingControllerError>;",
				"pub struct GreetingControllerImpl<GreetingStorePort: GreetingStore> {",
				"    pub fn new(greeting_store: GreetingStorePort) -> Self {",
				"        crate::hand::greeting_controller::create_greeting(&self.greeting_store, body)",
			},
		},
		{
			name: "the hand body takes the ports and the body",
			path: "core/src/hand/greeting_controller.rs",
			want: []string{
				"pub fn create_greeting<GreetingStorePort: GreetingStore>(greeting_store: &GreetingStorePort, body: Greeting) -> Result<Greeting, GreetingControllerError> {",
			},
		},
		{
			name: "the sqlite adapter owns the table and the audit table",
			path: "app/src/adapter/zz_generated_greeting_sqlite.rs",
			want: []string{
				"pub struct GreetingSqliteStore {",
				"CREATE TABLE IF NOT EXISTS greeting (id TEXT PRIMARY KEY, body TEXT NOT NULL);",
				"CREATE TABLE IF NOT EXISTS audit (at TEXT NOT NULL, table_name TEXT NOT NULL, key TEXT NOT NULL, op TEXT NOT NULL, before TEXT, after TEXT);",
				"impl GreetingStore for GreetingSqliteStore {",
			},
		},
		{
			name: "the driver routes the path to the controller through wire types",
			path: "app/src/driver/zz_generated_http_driver.rs",
			want: []string{
				"    greeting_controller: Arc<dyn GreetingController>,",
				`            .route("/greetings", routing::post(create_greeting))`,
				"        .create_greeting(body.into())",
				"    Ok((StatusCode::CREATED, Json(out.into())))",
			},
		},
		{
			name: "main reads the store path and the address and prints LISTENING",
			path: "app/src/bin/songe-hello-server.rs",
			want: []string{
				`std::env::var("SONGE_STORE_GREETING_PATH")`,
				`std::env::var("SONGE_HELLO_ADDR").unwrap_or_else(|_| "127.0.0.1:0".to_string())`,
				`println!("LISTENING {port}");`,
			},
		},
		{
			name: "lib mounts the modules through path attributes",
			path: "core/src/lib.rs",
			want: []string{
				`#[path = "types/zz_generated_mod.rs"]`,
				`#[path = "zz_generated_hand.rs"]`,
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

func TestEveryGeneratedFileCarriesTheHeaderAndOnlyHandFilesAreWriteOnce(t *testing.T) {
	files, err := hexrust.Generate([]byte(oneStoreOneOperation), hexrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	const header = "// Code generated by hexagonal-rust (forge-dev-codegen). DO NOT EDIT.\n"

	for _, f := range files {
		hand := strings.Contains(f.Path, "/src/hand/")

		if hand != f.WriteOnce {
			t.Errorf("%s: write once must hold for hand files only", f.Path)
		}

		if hand == strings.HasPrefix(f.Content, header) {
			t.Errorf("%s: the header belongs on every file but a hand file", f.Path)
		}
	}
}

func TestASideAnswersOneCrateAtItsOwnRoot(t *testing.T) {
	tests := []struct {
		side   string
		opts   hexrust.Options
		want   string
		reject string
	}{
		{side: "core", opts: hexrust.Options{Service: "svc", Side: "core", CoreDir: "."}, want: "src/", reject: "app/"},
		{side: "app", opts: hexrust.Options{Service: "svc", Side: "app", AppDir: "."}, want: "src/", reject: "core/"},
	}

	for _, tt := range tests {
		t.Run(tt.side+" side stays inside its crate", func(t *testing.T) {
			files, err := hexrust.Generate([]byte(oneStoreOneOperation), tt.opts)
			if err != nil {
				t.Fatalf("generating: %v", err)
			}

			if len(files) == 0 {
				t.Fatal("no files")
			}

			for _, f := range files {
				if !strings.HasPrefix(f.Path, tt.want) || strings.HasPrefix(f.Path, tt.reject) {
					t.Errorf("%s leaks out of the %s side", f.Path, tt.side)
				}
			}
		})
	}

	if _, err := hexrust.Generate([]byte(oneStoreOneOperation), hexrust.Options{Service: "svc", Side: "both"}); err == nil {
		t.Error("an unknown side must be refused")
	}
}

func TestTheOutputRootsPrefixEveryPath(t *testing.T) {
	files, err := hexrust.Generate([]byte(oneStoreOneOperation), hexrust.Options{
		Service: "songe-hello",
		CoreDir: "../songe-hello-core",
		AppDir:  "../songe-hello-app",
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	for _, f := range files {
		if !strings.HasPrefix(f.Path, "../songe-hello-core/src/") && !strings.HasPrefix(f.Path, "../songe-hello-app/src/") {
			t.Errorf("%s is outside both roots", f.Path)
		}
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
			if got := hexrust.Snake(tt.in); got != tt.snake {
				t.Errorf("Snake(%q) = %q, want %q", tt.in, got, tt.snake)
			}

			if got := hexrust.Pascal(tt.in); got != tt.pascal {
				t.Errorf("Pascal(%q) = %q, want %q", tt.in, got, tt.pascal)
			}

			if got := hexrust.Upper(tt.in); got != tt.upper {
				t.Errorf("Upper(%q) = %q, want %q", tt.in, got, tt.upper)
			}
		})
	}
}

const workspaceManifest = `[workspace]
resolver = "2"
members = ["core", "app"]
`

const coreManifest = `[package]
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

const appManifest = `[package]
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
`

func TestTheGeneratedSkeletonPassesCargoCheck(t *testing.T) {
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Skip("cargo is not on PATH")
	}

	files, err := hexrust.Generate([]byte(twoOperationsAndAPathParam), hexrust.Options{Service: "songe-hello"})
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

	write("Cargo.toml", workspaceManifest)
	write("core/Cargo.toml", coreManifest)
	write("app/Cargo.toml", appManifest)

	for _, f := range files {
		write(f.Path, f.Content)
	}

	cmd := exec.Command(cargo, "check", "--workspace", "--all-targets")
	cmd.Dir = root

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cargo check: %v\n%s", err, out)
	}
}
