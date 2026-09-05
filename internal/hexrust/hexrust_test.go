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
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/grpcrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/hexrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/restrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/udprust"
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

const helloGrpcProto = `syntax = "proto3";

package songe.hello.v1;

service Hello {
  rpc Ping(PingRequest) returns (PingReply);
}

message PingRequest {
  string message = 1;
}

message PingReply {
  string message = 1;
}
`

const helloUdpProto = `syntax = "proto3";

package songe.hello.udp.v1;

service HelloDatagram {
  rpc Echo(Echo) returns (Echo);
}

message Echo {
  string payload = 1;
  uint64 count = 2;
}
`

const helloWiring = `binary: songe-hello-node
ports:
  GreetingStore:
    default: sqlite
    adapters:
      sqlite: {}
      memory:
        type: GreetingMemoryStore
        module: adapter::greeting_memory
        config:
          capacity: { type: integer, default: 100 }
drivers:
  rest: { enabled: true }
  grpc: { enabled: true }
  udp:  { enabled: true }
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

func standUpCells(t *testing.T, cells ...string) string {
	t.Helper()

	root := t.TempDir()
	write := writeUnder(t, root)

	for _, cell := range cells {
		write(filepath.Join("src", cell, hexrust.CellConfigFile), "name: songe-hello\nkind: "+cell+"\n")

		for _, f := range cellFiles(t, cell) {
			if f.Path != cellmanifest.FileName {
				continue
			}

			write(filepath.Join("src", cell, f.Path), f.Content)
		}
	}

	return root
}

type emitted struct {
	Path    string
	Content string
}

func cellFiles(t *testing.T, cell string) []emitted {
	t.Helper()

	out := []emitted{}

	switch cell {
	case "rest":
		files, err := restrust.Generate([]byte(helloSpec), restrust.Options{Service: "songe-hello"})
		if err != nil {
			t.Fatalf("generating the rest cell: %v", err)
		}

		for _, f := range files {
			out = append(out, emitted{Path: f.Path, Content: f.Content})
		}
	case "grpc":
		files, err := grpcrust.Generate([]byte(helloGrpcProto), grpcrust.Options{Service: "songe-hello"})
		if err != nil {
			t.Fatalf("generating the grpc cell: %v", err)
		}

		for _, f := range files {
			out = append(out, emitted{Path: f.Path, Content: f.Content})
		}
	case "udp":
		files, err := udprust.Generate([]byte(helloUdpProto), udprust.Options{Service: "songe-hello"})
		if err != nil {
			t.Fatalf("generating the udp cell: %v", err)
		}

		for _, f := range files {
			out = append(out, emitted{Path: f.Path, Content: f.Content})
		}
	}

	return out
}

func generateHello(t *testing.T, root, wiring string) map[string]string {
	t.Helper()

	files, err := hexrust.Generate(hexrust.Options{
		Service: "songe-hello",
		SrcDir:  root,
		Cells:   []string{"grpc", "rest", "udp"},
		Wiring:  []byte(wiring),
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	byPath := map[string]string{}
	for _, f := range files {
		byPath[f.Path] = f.Content
	}

	return byPath
}

func TestTheSkeletonEmitsTheCrateRootTheRootLayersTheConfigModuleAndMain(t *testing.T) {
	root := standUpCells(t, "grpc", "rest", "udp")

	files, err := hexrust.Generate(hexrust.Options{
		Service: "songe-hello",
		SrcDir:  root,
		Cells:   []string{"grpc", "rest", "udp"},
		Wiring:  []byte(helloWiring),
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	got := []string{}
	for _, f := range files {
		got = append(got, f.Path)
	}

	want := []string{
		"src/adapter/mod.rs",
		"src/adapter/zz_generated_greeting_memory_config.rs",
		"src/bin/zz_generated_songe_hello_node.rs",
		"src/config/mod.rs",
		"src/controller/mod.rs",
		"src/driver/mod.rs",
		"src/lib.rs",
		"src/port/mod.rs",
		"src/types/mod.rs",
		"zz_generated_config_spec.yaml",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("emitted paths\n got %q\nwant %q", got, want)
	}
}

func TestTheCrateRootMountsEveryRootLayerTheConfigModuleAndEveryCell(t *testing.T) {
	root := standUpCells(t, "grpc", "rest", "udp")
	files := generateHello(t, root, helloWiring)

	for _, want := range []string{
		"pub mod adapter;",
		"pub mod config;",
		"pub mod controller;",
		"pub mod driver;",
		"pub mod port;",
		"pub mod types;",
		"pub mod grpc;",
		"pub mod rest;",
		"pub mod udp;",
	} {
		if !strings.Contains(files["src/lib.rs"], want) {
			t.Errorf("src/lib.rs lacks %q\n%s", want, files["src/lib.rs"])
		}
	}

	if !strings.Contains(files["src/config/mod.rs"], "pub mod zz_generated_config;") ||
		!strings.Contains(files["src/config/mod.rs"], "pub use zz_generated_config::*;") {
		t.Errorf("the config module does not mount the generated loader\n%s", files["src/config/mod.rs"])
	}
}

func TestAHandWrittenCandidateGetsAModLineAndAConfigStruct(t *testing.T) {
	root := standUpCells(t, "grpc", "rest", "udp")
	files := generateHello(t, root, helloWiring)

	for _, want := range []string{
		"#![allow(clippy::disallowed_methods, clippy::disallowed_types)]",
		"pub mod zz_generated_greeting_memory_config;",
		"mod greeting_memory;",
		"pub use zz_generated_greeting_memory_config::GreetingMemoryStoreConfig;",
	} {
		if !strings.Contains(files["src/adapter/mod.rs"], want) {
			t.Errorf("src/adapter/mod.rs lacks %q\n%s", want, files["src/adapter/mod.rs"])
		}
	}

	for _, want := range []string{
		"pub struct GreetingMemoryStoreConfig {",
		"    pub capacity: i64,",
	} {
		if !strings.Contains(files["src/adapter/zz_generated_greeting_memory_config.rs"], want) {
			t.Errorf("the hand candidate config lacks %q\n%s", want, files["src/adapter/zz_generated_greeting_memory_config.rs"])
		}
	}
}

func TestMainMatchesEachPortBuildsEachControllerAndGuardsEachDriver(t *testing.T) {
	root := standUpCells(t, "grpc", "rest", "udp")
	files := generateHello(t, root, helloWiring)

	main := files["src/bin/zz_generated_songe_hello_node.rs"]

	for _, want := range []string{
		"use songe_hello::config::SongeHelloConfig;",
		"let config = SongeHelloConfig::load(&args).context(\"loading the configuration\")?;",
		"let greeting_store: Arc<dyn GreetingStore + Send + Sync> = match config.greeting_store.as_str() {",
		`"memory" => Arc::new(`,
		"GreetingMemoryStore::new(GreetingMemoryStoreConfig {",
		"capacity: config.greeting_store_memory_capacity,",
		`"sqlite" => Arc::new(`,
		"path: config.greeting_store_sqlite_path.clone(),",
		`.context("building the sqlite adapter of songe-hello-node")?,`,
		"other => bail!(",
		`"building GreetingStore: {other:?} names no adapter, the adapters are memory, sqlite"`,
		"let greeting_controller: Arc<dyn GreetingController + Send + Sync> =",
		"Arc::new(GreetingControllerImpl::new(greeting_store.clone()));",
		"if config.driver_rest {",
		"if config.driver_grpc {",
		"if config.driver_udp {",
		".bind()",
		".announce()",
		"handles.push(tokio::spawn(async move {",
		"chain(&error)",
		"if handles.is_empty() {",
		"enable at least one of grpc, rest, udp",
		"for handle in handles {",
	} {
		if !strings.Contains(main, want) {
			t.Errorf("main lacks %q\n%s", want, main)
		}
	}
}

func TestTheConfigSpecHoldsOneKeyPerChoicePerAdapterFieldPerDriverAndPerDriverField(t *testing.T) {
	root := standUpCells(t, "grpc", "rest", "udp")
	files := generateHello(t, root, helloWiring)

	spec := files["zz_generated_config_spec.yaml"]

	for _, want := range []string{
		"# Code generated by hexagonal-rust (forge-dev-codegen). DO NOT EDIT.",
		"greeting_store:",
		"default: sqlite",
		"greeting_store_memory_capacity:",
		"greeting_store_sqlite_path:",
		"driver_grpc:",
		"driver_rest:",
		"driver_udp:",
		"grpc_addr:",
		"rest_addr:",
		"udp_addr:",
	} {
		if !strings.Contains(spec, want) {
			t.Errorf("the config spec lacks %q\n%s", want, spec)
		}
	}

	if strings.Contains(spec, "paths:") {
		t.Errorf("the config spec holds more than components.schemas.Spec\n%s", spec)
	}
}

func TestEveryConfigKeyNamesItsEnvAfterTheBinaryAndNotAfterTheCell(t *testing.T) {
	root := standUpCells(t, "grpc", "rest", "udp")
	files := generateHello(t, root, helloWiring)

	spec := files["zz_generated_config_spec.yaml"]

	for _, want := range []string{
		"x-env: SONGE_HELLO_NODE_GREETING_STORE",
		"x-env: SONGE_HELLO_NODE_GREETING_STORE_SQLITE_PATH",
		"x-env: SONGE_HELLO_NODE_REST_ADDR",
		"x-env: SONGE_HELLO_NODE_GRPC_ADDR",
		"x-env: SONGE_HELLO_NODE_UDP_ADDR",
		"x-env: SONGE_HELLO_NODE_DRIVER_UDP",
		"x-flag: greeting-store",
		"x-flag: rest-addr",
	} {
		if !strings.Contains(spec, want) {
			t.Errorf("the config spec lacks %q\n%s", want, spec)
		}
	}
}

func TestAFieldNoManifestDescribedStillCarriesADescriptionSoTheDocCommentIsNeverEmpty(t *testing.T) {
	root := standUpCells(t, "grpc", "rest", "udp")
	files := generateHello(t, root, helloWiring)

	spec := files["zz_generated_config_spec.yaml"]

	want := "description: The greeting store memory capacity of songe-hello-node"
	if !strings.Contains(spec, want) {
		t.Errorf("the config spec lacks %q\n%s", want, spec)
	}

	if strings.Contains(spec, "description: \"\"") || strings.Contains(spec, "description: ''") {
		t.Errorf("the config spec carries an empty description\n%s", spec)
	}
}

func TestTheConfigTypeIsNamedForTheCellBecauseTheConfigGeneratorNamesItThatWay(t *testing.T) {
	root := standUpCells(t, "grpc", "rest", "udp")
	files := generateHello(t, root, helloWiring)

	main := files["src/bin/zz_generated_songe_hello_node.rs"]

	if strings.Contains(main, "SongeHelloNodeConfig") {
		t.Errorf("main names the config type after the binary, and the config generator names it after the cell\n%s", main)
	}
}

func TestAListedCellThatOwnsNoGeneratorIsRefused(t *testing.T) {
	root := standUpCells(t, "grpc", "rest")

	_, err := hexrust.Generate(hexrust.Options{
		Service: "songe-hello",
		SrcDir:  root,
		Cells:   []string{"grpc", "rest", "udp"},
		Wiring:  []byte(helloWiring),
	})

	want := `mounting cell "udp": src/udp/forge-dev.yaml is missing, a listed cell owns its own generator`
	if err == nil || err.Error() != want {
		t.Fatalf("generating reported %v, want %q", err, want)
	}
}

func TestAListedCellWithNoManifestIsRefused(t *testing.T) {
	root := standUpCells(t, "grpc", "rest", "udp")

	if err := os.Remove(filepath.Join(root, "src", "udp", cellmanifest.FileName)); err != nil {
		t.Fatalf("removing the manifest: %v", err)
	}

	_, err := hexrust.Generate(hexrust.Options{
		Service: "songe-hello",
		SrcDir:  root,
		Cells:   []string{"grpc", "rest", "udp"},
		Wiring:  []byte(helloWiring),
	})

	want := `mounting cell "udp": src/udp/zz_generated_cell.yaml is missing, build the cell before the skeleton`
	if err == nil || err.Error() != want {
		t.Fatalf("generating reported %v, want %q", err, want)
	}
}

func TestTheWiringIsRefusedWhenItFightsTheManifests(t *testing.T) {
	tests := []struct {
		name   string
		wiring string
		want   string
	}{
		{
			name: "a port a controller consumes with no candidate",
			wiring: `binary: songe-hello-node
drivers:
  rest: { enabled: true }
  grpc: { enabled: true }
  udp: { enabled: true }
`,
			want: `wiring the ports: controller "GreetingController" consumes port "GreetingStore" and the wiring names no candidate for it`,
		},
		{
			name: "a candidate no manifest provides",
			wiring: `binary: songe-hello-node
ports:
  GreetingStore:
    default: postgres
    adapters:
      postgres: {}
drivers:
  rest: { enabled: true }
  grpc: { enabled: true }
  udp: { enabled: true }
`,
			want: `wiring port "GreetingStore": candidate "postgres" declares no type and no cell manifest provides an adapter named "postgres"`,
		},
		{
			name: "a driver no manifest provides",
			wiring: `binary: songe-hello-node
ports:
  GreetingStore:
    default: sqlite
    adapters:
      sqlite: {}
drivers:
  rest: { enabled: true }
  grpc: { enabled: true }
  udp: { enabled: true }
  websocket: { enabled: true }
`,
			want: `wiring driver "websocket": no cell manifest provides a driver with that name, the cells provide grpc, rest, udp`,
		},
		{
			name: "a driver the wiring never names",
			wiring: `binary: songe-hello-node
ports:
  GreetingStore:
    default: sqlite
    adapters:
      sqlite: {}
drivers:
  rest: { enabled: true }
  grpc: { enabled: true }
`,
			want: `wiring the drivers: cell "udp" provides driver "udp" and the wiring never names it`,
		},
		{
			name: "an unknown key",
			wiring: `binary: songe-hello-node
portz:
  GreetingStore:
    default: sqlite
`,
			want: `unknown field "portz"`,
		},
		{
			name: "a default naming no candidate",
			wiring: `binary: songe-hello-node
ports:
  GreetingStore:
    default: postgres
    adapters:
      sqlite: {}
drivers:
  rest: { enabled: true }
`,
			want: `port "GreetingStore" has default "postgres", which names no candidate, the candidates are sqlite`,
		},
		{
			name: "a hand written candidate outside the adapter layer",
			wiring: `binary: songe-hello-node
ports:
  GreetingStore:
    default: memory
    adapters:
      memory:
        type: GreetingMemoryStore
        module: rest::adapter::greeting_memory
drivers:
  rest: { enabled: true }
  grpc: { enabled: true }
  udp: { enabled: true }
`,
			want: `wiring port "GreetingStore": candidate "memory" has module "rest::adapter::greeting_memory", a hand written adapter lives under the adapter layer`,
		},
		{
			name: "a candidate that implements another port",
			wiring: `binary: songe-hello-node
ports:
  GreetingStore:
    default: grpc_client
    adapters:
      grpc_client: {}
drivers:
  rest: { enabled: true }
  grpc: { enabled: true }
  udp: { enabled: true }
`,
			want: `wiring port "GreetingStore": candidate "grpc_client" implements "HelloClient" instead`,
		},
	}

	root := standUpCells(t, "grpc", "rest", "udp")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := hexrust.Generate(hexrust.Options{
				Service: "songe-hello",
				SrcDir:  root,
				Cells:   []string{"grpc", "rest", "udp"},
				Wiring:  []byte(tt.wiring),
			})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("generating reported %v, want it to name %q", err, tt.want)
			}
		})
	}
}

func TestAHandWrittenCandidateTheWiringCallsFallibleGetsAContextAndAQuestionMark(t *testing.T) {
	root := standUpCells(t, "grpc", "rest", "udp")

	fallible := `binary: songe-hello-node
ports:
  GreetingStore:
    default: memory
    adapters:
      sqlite: {}
      memory:
        type: GreetingMemoryStore
        module: adapter::greeting_memory
        fallible: true
        config:
          capacity: { type: integer, default: 100 }
drivers:
  rest: { enabled: true }
  grpc: { enabled: true }
  udp:  { enabled: true }
`

	main := generateHello(t, root, fallible)["src/bin/zz_generated_songe_hello_node.rs"]

	want := `.context("building the memory adapter of songe-hello-node")?,`
	if !strings.Contains(main, want) {
		t.Errorf("main lacks %q\n%s", want, main)
	}
}

func TestAHandWrittenCandidateTheWiringLeavesInfallibleGetsNoQuestionMark(t *testing.T) {
	root := standUpCells(t, "grpc", "rest", "udp")

	main := generateHello(t, root, helloWiring)["src/bin/zz_generated_songe_hello_node.rs"]

	armStart := `"memory" => Arc::new(`
	armEnd := "\n        ),"

	start := strings.Index(main, armStart)
	if start < 0 {
		t.Fatalf("main lacks the memory candidate arm %q\n%s", armStart, main)
	}

	length := strings.Index(main[start:], armEnd)
	if length < 0 {
		t.Fatalf("main never closes the memory candidate arm\n%s", main)
	}

	arm := main[start : start+length]

	if !strings.Contains(arm, "GreetingMemoryStore::new(GreetingMemoryStoreConfig {") {
		t.Fatalf("the memory candidate arm never builds the adapter\n%s", arm)
	}

	if strings.Contains(arm, "?") {
		t.Errorf("main asks a question mark of an adapter that never fails\n%s", arm)
	}
}

func TestAPortACellRequiresAndNoCellDeclaresIsRefused(t *testing.T) {
	root := standUpCells(t, "rest")

	manifest := cellmanifest.Manifest{
		Version:   cellmanifest.Version,
		Cell:      "ws",
		Generator: "a test",
		Requires:  cellmanifest.Requires{Ports: []string{"Clock"}},
	}

	body, err := cellmanifest.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshalling the manifest: %v", err)
	}

	write := writeUnder(t, root)
	write(filepath.Join("src", "ws", hexrust.CellConfigFile), "name: songe-hello\nkind: ws\n")
	write(filepath.Join("src", "ws", cellmanifest.FileName), string(body))

	_, err = hexrust.Generate(hexrust.Options{
		Service: "songe-hello",
		SrcDir:  root,
		Cells:   []string{"rest", "ws"},
		Wiring: []byte(`binary: songe-hello-node
ports:
  GreetingStore:
    default: sqlite
    adapters:
      sqlite: {}
drivers:
  rest: { enabled: true }
`),
	})

	want := `wiring the ports: cell "ws" requires port "Clock" and no cell manifest declares that port trait`
	if err == nil || err.Error() != want {
		t.Fatalf("generating reported %v, want %q", err, want)
	}
}

func TestADriverRequiringAControllerNoManifestProvidesIsRefused(t *testing.T) {
	root := standUpCells(t, "rest")

	manifest := cellmanifest.Manifest{
		Version:   cellmanifest.Version,
		Cell:      "ws",
		Generator: "a test",
		Provides: cellmanifest.Provides{
			Drivers: []cellmanifest.Driver{{
				Name:     "ws",
				Type:     "WsDriver",
				Module:   "ws::driver::ws_driver",
				Requires: []string{"NobodyController"},
			}},
		},
	}

	body, err := cellmanifest.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshalling the manifest: %v", err)
	}

	write := writeUnder(t, root)
	write(filepath.Join("src", "ws", hexrust.CellConfigFile), "name: songe-hello\nkind: ws\n")
	write(filepath.Join("src", "ws", cellmanifest.FileName), string(body))

	_, err = hexrust.Generate(hexrust.Options{
		Service: "songe-hello",
		SrcDir:  root,
		Cells:   []string{"rest", "ws"},
		Wiring: []byte(`binary: songe-hello-node
ports:
  GreetingStore:
    default: sqlite
    adapters:
      sqlite: {}
drivers:
  rest: { enabled: true }
  ws: { enabled: true }
`),
	})

	want := `wiring driver "ws": it requires controller "NobodyController" and no cell manifest provides it`
	if err == nil || err.Error() != want {
		t.Fatalf("generating reported %v, want %q", err, want)
	}
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
			_, err := hexrust.Generate(hexrust.Options{
				Service: "songe-hello",
				Cells:   tt.cells,
				Wiring:  []byte(helloWiring),
			})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want an error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestASkeletonWithNoWiringIsRefused(t *testing.T) {
	_, err := hexrust.Generate(hexrust.Options{Service: "songe-hello"})

	want := "emitting the skeleton: the cell names no wiring file, add wiring.specPath beside openapi"
	if err == nil || err.Error() != want {
		t.Fatalf("generating reported %v, want %q", err, want)
	}
}

func TestASkeletonWithNoServiceNameIsRefused(t *testing.T) {
	_, err := hexrust.Generate(hexrust.Options{Wiring: []byte(helloWiring)})
	if err == nil || !strings.Contains(err.Error(), "the service name is required") {
		t.Fatalf("generating reported %v", err)
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

func TestALayoutWithNoCellsMountsNothingExtra(t *testing.T) {
	got, err := hexrust.CellsFromLayout(map[string]interface{}{})
	if err != nil {
		t.Fatalf("reading the layout: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("want no cells, got %+v", got)
	}
}

func TestNoGeneratedFileCarriesAPathAttribute(t *testing.T) {
	root := standUpCells(t, "grpc", "rest", "udp")
	files := generateHello(t, root, helloWiring)

	for path, content := range files {
		if strings.Contains(content, "#[path") {
			t.Errorf("%s mounts a module with a path attribute\n%s", path, content)
		}
	}
}
