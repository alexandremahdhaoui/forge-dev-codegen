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
      x-store:
        key: id
        lookups: []
        adapters: [sqlite]
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

func generateHello(t *testing.T, root, wiring string, cells ...string) map[string]string {
	t.Helper()

	if len(cells) == 0 {
		cells = []string{"grpc", "rest", "udp"}
	}

	files, err := hexrust.Generate(hexrust.Options{
		Service: "songe-hello",
		SrcDir:  root,
		Cells:   cells,
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
		"src/bin/zz_generated_songe_hello_node.rs",
		"src/config/mod.rs",
		"src/controller/mod.rs",
		"src/driver/mod.rs",
		"src/lib.rs",
		"src/port/mod.rs",
		"src/types/mod.rs",
		"zz_generated_build.rs",
		"zz_generated_config_spec.yaml",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("emitted paths\n got %q\nwant %q", got, want)
	}
}

func TestTheCrateRootBuildScriptIncludesEveryCellBuildScriptByPath(t *testing.T) {
	root := standUpCells(t, "grpc", "rest", "udp")
	files := generateHello(t, root, helloWiring)

	build, emitted := files["zz_generated_build.rs"]
	if !emitted {
		t.Fatalf("no build script was emitted at the crate root\n%q", files)
	}

	for _, want := range []string{
		"// Code generated by hexagonal-rust (forge-dev-codegen). DO NOT EDIT.",
		`include!("src/grpc/zz_generated_build.rs");`,
	} {
		if !strings.Contains(build, want) {
			t.Errorf("zz_generated_build.rs lacks %q\n%s", want, build)
		}
	}

	if strings.Count(build, "include!") != 1 {
		t.Errorf("only the grpc cell declares a build script, got\n%s", build)
	}
}

func TestNoCrateRootBuildScriptIsEmittedWhenNoCellDeclaresOne(t *testing.T) {
	root := standUpCells(t, "rest", "udp")

	files, err := hexrust.Generate(hexrust.Options{
		Service: "songe-hello",
		SrcDir:  root,
		Cells:   []string{"rest", "udp"},
		Wiring: []byte(`binary: songe-hello-node
ports:
  GreetingStore:
    default: sqlite
drivers:
  rest: { enabled: true }
  udp:  { enabled: true }
`),
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	for _, f := range files {
		if f.Path == "zz_generated_build.rs" {
			t.Fatalf("a build script was emitted with no cell declaring one\n%s", f.Content)
		}
	}
}

func TestTwoCellsThatEachDeclareABuildScriptAreRefusedBecauseOneCrateHoldsOneFnMain(t *testing.T) {
	root := standUpCells(t, "grpc", "rest")

	manifest := cellmanifest.Manifest{
		Version:     cellmanifest.Version,
		Cell:        "ws",
		Generator:   "a test",
		BuildScript: "zz_generated_build.rs",
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
		Cells:   []string{"grpc", "rest", "ws"},
		Wiring: []byte(`binary: songe-hello-node
ports:
  GreetingStore:
    default: sqlite
drivers:
  rest: { enabled: true }
  grpc: { enabled: true }
`),
	})

	want := `cells "grpc" and "ws" both declare a build script, one crate holds one fn main`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("generating reported %v, want it to name %q", err, want)
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

func TestTheCrateRootReexportsEveryPortACellProvidesSoAnyCellReachesItAtOnePath(t *testing.T) {
	root := standUpCells(t, "grpc", "rest", "udp")
	files := generateHello(t, root, helloWiring)

	mod := files["src/port/mod.rs"]

	if !strings.Contains(mod, "pub use crate::rest::port::greeting_store as greeting_store;") {
		t.Errorf("src/port/mod.rs never re-exported the store the rest cell provides\n%s", mod)
	}

	if strings.Contains(mod, "pub mod greeting_store;") {
		t.Errorf("the crate root declared a module for a port a cell owns\n%s", mod)
	}
}

func TestACellProvidingAPortTheCrateRootWouldWriteTakesItOverAndTheLayoutIsRefusedByName(t *testing.T) {
	root := standUpGuardedRestCell(t)

	writeCell(t, root, "extra", cellmanifest.Manifest{
		Version:   cellmanifest.Version,
		Cell:      "extra",
		Generator: "a test",
		Provides: cellmanifest.Provides{
			Ports: []cellmanifest.Port{{Trait: "TicketVerifier", Module: "extra::port::ticket_verifier"}},
		},
	})

	_, err := hexrust.Generate(hexrust.Options{
		Service: "songe-hello",
		SrcDir:  root,
		Cells:   []string{"extra", "rest", "tui"},
		Ports:   guardedCrateRootPorts,
		Wiring:  []byte(guardedWiring),
	})
	if err == nil || !strings.Contains(err.Error(), "no cell requires that port, so the crate root never emits it") {
		t.Fatalf("want the crate root to say it writes nothing for a port a cell owns, got %v", err)
	}
}

func TestTheCrateRootStillAliasesThePortsItWritesItselfBesideWhatItReexports(t *testing.T) {
	root := standUpGuardedRestCell(t)

	files, err := hexrust.Generate(hexrust.Options{
		Service: "songe-hello",
		SrcDir:  root,
		Cells:   []string{"rest", "tui"},
		Ports:   guardedCrateRootPorts,
		Wiring:  []byte(guardedWiring),
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	mod := ""

	for _, f := range files {
		if f.Path == "src/port/mod.rs" {
			mod = f.Content
		}
	}

	for _, want := range []string{
		"pub use zz_generated_ticket_verifier as ticket_verifier;",
		"pub use crate::rest::port::greeting_store as greeting_store;",
	} {
		if !strings.Contains(mod, want) {
			t.Errorf("src/port/mod.rs lacks %q\n%s", want, mod)
		}
	}
}

func TestTheCrateRootAdapterLayerCarriesNoShellForACellProvidedAdapter(t *testing.T) {
	root := standUpCells(t, "grpc", "rest", "udp")
	files := generateHello(t, root, helloWiring)

	mod := files["src/adapter/mod.rs"]

	if !strings.Contains(mod, "#![allow(clippy::disallowed_methods, clippy::disallowed_types)]") {
		t.Errorf("src/adapter/mod.rs lost its allow line\n%s", mod)
	}

	for _, unwanted := range []string{"greeting_memory", "GreetingMemoryStoreConfig", "mod "} {
		if strings.Contains(mod, unwanted) {
			t.Errorf("src/adapter/mod.rs still carries %q, an adapter a cell provides needs no crate root shell\n%s", unwanted, mod)
		}
	}

	for path := range files {
		if strings.HasSuffix(path, "_config.rs") && strings.HasPrefix(path, "src/adapter/") {
			t.Errorf("the crate root wrote an adapter config shell at %s", path)
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
		`"sqlite" => Arc::new(`,
		"GreetingSqliteStore::new(GreetingSqliteStoreConfig {",
		"path: config.greeting_store_sqlite_path.clone(),",
		`"sqlite" => Arc::new(`,
		"path: config.greeting_store_sqlite_path.clone(),",
		`.context("building the sqlite adapter of songe-hello-node")?,`,
		"other => bail!(",
		`"building GreetingStore: {other:?} names no adapter, the adapters are sqlite"`,
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

	writeCell(t, root, "ws", cellmanifest.Manifest{
		Version:   cellmanifest.Version,
		Cell:      "ws",
		Generator: "a test",
		Provides: cellmanifest.Provides{
			Adapters: []cellmanifest.Adapter{{
				Name:       "quiet",
				Type:       "QuietStore",
				Module:     "ws::adapter::quiet",
				Implements: "GreetingStore",
				Config:     map[string]cellmanifest.ConfigField{"capacity": {Type: cellmanifest.FieldTypeInteger, Default: 100}},
			}},
		},
	})

	files := generateHello(t, root, `binary: songe-hello-node
ports:
  GreetingStore:
    default: quiet
drivers:
  rest: { enabled: true }
  grpc: { enabled: true }
  udp:  { enabled: true }
`, "grpc", "rest", "udp", "ws")

	spec := files["zz_generated_config_spec.yaml"]

	want := "description: The greeting store quiet capacity of songe-hello-node"
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
			name: "a default no manifest provides",
			wiring: `binary: songe-hello-node
ports:
  GreetingStore:
    default: postgres
drivers:
  rest: { enabled: true }
  grpc: { enabled: true }
  udp: { enabled: true }
`,
			want: `wiring port "GreetingStore": the default is "postgres", which no cell provides, the cells provide sqlite`,
		},
		{
			name: "a driver no manifest provides",
			wiring: `binary: songe-hello-node
ports:
  GreetingStore:
    default: sqlite
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
			name: "a port that names no default",
			wiring: `binary: songe-hello-node
ports:
  GreetingStore: {}
drivers:
  rest: { enabled: true }
`,
			want: `port "GreetingStore" names no default, a wiring entry exists to pick which adapter main builds`,
		},
		{
			name: "an adapter the wiring tries to describe itself",
			wiring: `binary: songe-hello-node
ports:
  GreetingStore:
    default: memory
    adapters:
      memory:
        type: GreetingMemoryStore
        module: adapter::greeting_memory
drivers:
  rest: { enabled: true }
`,
			want: `unknown field "adapters"`,
		},
		{
			name: "a default that implements another port",
			wiring: `binary: songe-hello-node
ports:
  GreetingStore:
    default: grpc_client
drivers:
  rest: { enabled: true }
  grpc: { enabled: true }
  udp: { enabled: true }
`,
			want: `wiring port "GreetingStore": the default is "grpc_client", which no cell provides, the cells provide sqlite`,
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

func TestAnAdapterTheManifestMarksFallibleGetsAContextAndAQuestionMark(t *testing.T) {
	root := standUpCells(t, "grpc", "rest", "udp")

	main := generateHello(t, root, helloWiring)["src/bin/zz_generated_songe_hello_node.rs"]

	want := `.context("building the sqlite adapter of songe-hello-node")?,`
	if !strings.Contains(main, want) {
		t.Errorf("main lacks %q\n%s", want, main)
	}
}

func TestAnAdapterTheManifestLeavesInfallibleGetsNoQuestionMark(t *testing.T) {
	root := standUpGuardedRestCell(t)

	files, err := hexrust.Generate(hexrust.Options{
		Service: "songe-hello",
		SrcDir:  root,
		Cells:   []string{"rest", "tui"},
		Ports:   guardedCrateRootPorts,
		Wiring:  []byte(guardedWiring),
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	main := mainOf(t, files)

	armStart := `"greeting_event_memory_feed" => Arc::new(`
	armEnd := "\n        ),"

	start := strings.Index(main, armStart)
	if start < 0 {
		t.Fatalf("main lacks the memory feed arm %q\n%s", armStart, main)
	}

	length := strings.Index(main[start:], armEnd)
	if length < 0 {
		t.Fatalf("main never closes the memory feed arm\n%s", main)
	}

	arm := main[start : start+length]

	if !strings.Contains(arm, "GreetingEventMemoryFeed::new(GreetingEventMemoryFeedConfig {") {
		t.Fatalf("the memory feed arm never builds the adapter\n%s", arm)
	}

	if strings.Contains(arm, "?") {
		t.Errorf("main asks a question mark of an adapter that never fails\n%s", arm)
	}
}

func TestASilentWiringPicksTheOnlyAdapterTheCellsProvideForAPort(t *testing.T) {
	root := standUpCells(t, "grpc", "rest", "udp")
	files := generateHello(t, root, `binary: songe-hello-node
drivers:
  rest: { enabled: true }
  grpc: { enabled: true }
  udp: { enabled: true }
`)

	main := files["src/bin/zz_generated_songe_hello_node.rs"]

	for _, want := range []string{
		"let greeting_store: Arc<dyn GreetingStore + Send + Sync> = match config.greeting_store.as_str() {",
		`"sqlite" => Arc::new(`,
		`"building GreetingStore: {other:?} names no adapter, the adapters are sqlite"`,
	} {
		if !strings.Contains(main, want) {
			t.Errorf("main lacks %q\n%s", want, main)
		}
	}

	if !strings.Contains(files["zz_generated_config_spec.yaml"], "default: sqlite") {
		t.Errorf("the config spec does not default the choice to the only adapter\n%s", files["zz_generated_config_spec.yaml"])
	}
}

func writeWsCell(t *testing.T, root string, manifest cellmanifest.Manifest) {
	t.Helper()

	body, err := cellmanifest.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshalling the manifest: %v", err)
	}

	write := writeUnder(t, root)
	write(filepath.Join("src", "ws", hexrust.CellConfigFile), "name: songe-hello\nkind: ws\n")
	write(filepath.Join("src", "ws", cellmanifest.FileName), string(body))
}

func TestAPortAControllerConsumesWithNoAdapterAnywhereIsRefused(t *testing.T) {
	root := standUpCells(t, "rest")

	writeWsCell(t, root, cellmanifest.Manifest{
		Version:   cellmanifest.Version,
		Cell:      "ws",
		Generator: "a test",
		Provides: cellmanifest.Provides{
			Drivers: []cellmanifest.Driver{{
				Name:     "ws",
				Type:     "WsDriver",
				Module:   "ws::driver::ws_driver",
				Requires: []string{"WsController"},
			}},
			Controllers: []cellmanifest.Controller{{
				Trait:  "WsController",
				Impl:   "WsControllerImpl",
				Module: "ws::controller",
				Ports:  []string{"Clock"},
			}},
			Ports: []cellmanifest.Port{{Trait: "Clock", Module: "ws::port::clock"}},
		},
	})

	_, err := hexrust.Generate(hexrust.Options{
		Service: "songe-hello",
		SrcDir:  root,
		Cells:   []string{"rest", "ws"},
		Wiring: []byte(`binary: songe-hello-node
drivers:
  rest: { enabled: true }
  ws: { enabled: true }
`),
	})

	want := `wiring the ports: controller "WsController" consumes port "Clock" and no cell provides an adapter for it`
	if err == nil || err.Error() != want {
		t.Fatalf("generating reported %v, want %q", err, want)
	}
}

func TestAPortWithTwoProvidedAdaptersAndASilentWiringIsRefused(t *testing.T) {
	root := standUpCells(t, "rest")

	writeWsCell(t, root, cellmanifest.Manifest{
		Version:   cellmanifest.Version,
		Cell:      "ws",
		Generator: "a test",
		Provides: cellmanifest.Provides{
			Adapters: []cellmanifest.Adapter{{
				Name:       "greeting_redis",
				Type:       "GreetingRedisStore",
				Module:     "ws::adapter::greeting_redis",
				Implements: "GreetingStore",
			}},
		},
	})

	_, err := hexrust.Generate(hexrust.Options{
		Service: "songe-hello",
		SrcDir:  root,
		Cells:   []string{"rest", "ws"},
		Wiring: []byte(`binary: songe-hello-node
drivers:
  rest: { enabled: true }
`),
	})

	want := `wiring the ports: controller "GreetingController" consumes port "GreetingStore", the cells provide greeting_redis, sqlite for it and the wiring names no choice`
	if err == nil || err.Error() != want {
		t.Fatalf("generating reported %v, want %q", err, want)
	}
}

func TestADriverConsumingPortsGetsThemAfterItsControllers(t *testing.T) {
	root := standUpCells(t, "rest")

	writeWsCell(t, root, cellmanifest.Manifest{
		Version:   cellmanifest.Version,
		Cell:      "ws",
		Generator: "a test",
		Provides: cellmanifest.Provides{
			Drivers: []cellmanifest.Driver{{
				Name:     "ws",
				Type:     "WsDriver",
				Module:   "ws::driver::ws_driver",
				Requires: []string{"GreetingController"},
				Ports:    []string{"SessionGate", "GreetingStore"},
			}},
			Adapters: []cellmanifest.Adapter{{
				Name:       "gate_memory",
				Type:       "GateMemory",
				Module:     "ws::adapter::gate_memory",
				Implements: "SessionGate",
			}},
			Ports: []cellmanifest.Port{{Trait: "SessionGate", Module: "ws::port::session_gate"}},
		},
	})

	files, err := hexrust.Generate(hexrust.Options{
		Service: "songe-hello",
		SrcDir:  root,
		Cells:   []string{"rest", "ws"},
		Wiring: []byte(`binary: songe-hello-node
drivers:
  rest: { enabled: true }
  ws: { enabled: true }
`),
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	var main string
	for _, f := range files {
		if f.Path == "src/bin/zz_generated_songe_hello_node.rs" {
			main = f.Content
		}
	}

	for _, want := range []string{
		"let session_gate: Arc<dyn SessionGate + Send + Sync> = match config.session_gate.as_str() {",
		`"gate_memory" => Arc::new(`,
		"use songe_hello::ws::port::session_gate::SessionGate;",
		"let mut ws_driver = WsDriver::new(",
		"greeting_controller.clone(),\n            session_gate.clone(),\n            greeting_store.clone(),\n        );",
	} {
		if !strings.Contains(main, want) {
			t.Errorf("main lacks %q\n%s", want, main)
		}
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
