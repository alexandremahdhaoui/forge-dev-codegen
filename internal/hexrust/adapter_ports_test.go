package hexrust_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/hexrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/restrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/cellmanifest"
)

const guardedHelloSpec = `
openapi: 3.1.0
info:
  title: Hello API
  version: 1.0.0
paths:
  /greetings/{id}/count:
    post:
      operationId: countGreeting
      x-controller: greeting
      x-ports: [GreetingStore]
      x-auth: bearer
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        "200":
          description: The counted greeting
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Greeting"
  /greetings/{id}/events:
    get:
      operationId: streamGreetingEvents
      x-controller: greeting
      x-auth: bearer
      x-stream:
        from: Greeting
        adapters: [memory]
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        "200":
          description: The events of one greeting
          content:
            text/event-stream:
              schema:
                $ref: "#/components/schemas/GreetingEvent"
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
    GreetingEvent:
      type: object
      required: [id]
      properties:
        id:
          type: string
`

const accountSpec = `
openapi: 3.1.0
info:
  title: Account API
  version: 1.0.0
paths:
  /accounts:
    post:
      operationId: createAccount
      x-controller: account
      x-auth: bearer
      requestBody:
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/Account"
      responses:
        "201":
          description: The created account
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Account"
components:
  schemas:
    Account:
      type: object
      required: [id]
      properties:
        id:
          type: string
`

const guardedWiring = `binary: songe-hello-node
drivers:
  rest: { enabled: true }
  tui: { enabled: true }
`

var guardedCrateRootPorts = map[string][]string{"TicketVerifier": {"secret"}}

var tokenSourceAdapter = cellmanifest.Adapter{
	Name:       "file",
	Type:       "TicketFile",
	Module:     "tui::adapter::ticket_file",
	Implements: "TokenSource",
	Config: map[string]cellmanifest.ConfigField{
		"path": {Type: cellmanifest.FieldTypeString, Default: "ticket.txt", Description: "Where the ticket is read from"},
	},
}

func writeCell(t *testing.T, root, cell string, manifest cellmanifest.Manifest) {
	t.Helper()

	body, err := cellmanifest.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshalling the manifest: %v", err)
	}

	write := writeUnder(t, root)
	write(filepath.Join("src", cell, hexrust.CellConfigFile), "name: songe-hello\nkind: "+cell+"\n")
	write(filepath.Join("src", cell, cellmanifest.FileName), string(body))
}

func writeRestCell(t *testing.T, root, cell, spec, side string, sources bool) {
	t.Helper()

	write := writeUnder(t, root)

	files, err := restrust.Generate([]byte(spec), restrust.Options{Service: "songe-hello", Cell: cell, Side: side})
	if err != nil {
		t.Fatalf("generating the %s cell: %v", cell, err)
	}

	write(filepath.Join("src", cell, hexrust.CellConfigFile), "name: songe-hello\nkind: rest\n")

	for _, f := range files {
		if f.Path == cellmanifest.FileName || (sources && strings.HasSuffix(f.Path, ".rs")) {
			write(filepath.Join("src", cell, f.Path), f.Content)
		}
	}
}

func standUpGuardedRestCell(t *testing.T) string {
	return standUpGuardedCells(t, true)
}

func standUpGuardedCells(t *testing.T, withTokenSource bool) string {
	t.Helper()

	root := t.TempDir()

	writeRestCell(t, root, "rest", guardedHelloSpec, restrust.SideBoth, false)

	adapters := []cellmanifest.Adapter{}
	if withTokenSource {
		adapters = append(adapters, tokenSourceAdapter)
	}

	writeCell(t, root, "tui", cellmanifest.Manifest{
		Version:   cellmanifest.Version,
		Cell:      "tui",
		Generator: "a test",
		Provides: cellmanifest.Provides{
			Drivers: []cellmanifest.Driver{{
				Name:     "tui",
				Type:     "TuiDriver",
				Module:   "tui::driver::tui_driver",
				Requires: []string{"ScreenController"},
			}},
			Controllers: []cellmanifest.Controller{{
				Trait:  "ScreenController",
				Impl:   "ScreenControllerImpl",
				Module: "tui::controller",
				Ports:  []string{"GreetingClient"},
			}},
			Adapters: adapters,
		},
	})

	return root
}

func mainOf(t *testing.T, files []hexrust.File) string {
	t.Helper()

	for _, f := range files {
		if f.Path == "src/bin/zz_generated_songe_hello_node.rs" {
			return f.Content
		}
	}

	t.Fatal("no main was emitted")

	return ""
}

func TestAnAdapterConsumingAPortGetsItAfterItsConfigAndThatPortIsBuiltFirst(t *testing.T) {
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

	for _, want := range []string{
		"let token_source: Arc<dyn TokenSource + Send + Sync> = match config.token_source.as_str() {",
		"let greeting_client: Arc<dyn GreetingClient + Send + Sync> = match config.greeting_client.as_str() {",
		`"rest_greeting_client" => Arc::new(`,
		"GreetingRestClient::new(GreetingRestClientConfig {\n                base_url: config.greeting_client_rest_greeting_client_base_url.clone(),\n            }, token_source.clone()),",
		"use songe_hello::rest::adapter::greeting_rest_client::{GreetingRestClient, GreetingRestClientConfig};",
		"use songe_hello::port::token_source::TokenSource;",
		"use songe_hello::port::ticket_verifier::TicketVerifier;",
		"let mut rest_driver = HttpDriver::new(",
		"greeting_controller.clone(),\n            ticket_verifier.clone(),\n        );",
		"Arc::new(GreetingControllerImpl::new(greeting_event_subscribe.clone(), greeting_store.clone()));",
	} {
		if !strings.Contains(main, want) {
			t.Errorf("main lacks %q\n%s", want, main)
		}
	}

	tokenAt := strings.Index(main, "let token_source:")
	clientAt := strings.Index(main, "let greeting_client:")

	if tokenAt < 0 || clientAt < 0 || tokenAt > clientAt {
		t.Errorf("the token source is built at %d and the client that consumes it at %d, the port an adapter consumes must be built first\n%s", tokenAt, clientAt, main)
	}
}

func TestTheCrateRootEmitsSubjectTicketVerifierAndTokenSourceOnceForTwoRestCells(t *testing.T) {
	root := t.TempDir()

	writeRestCell(t, root, "rest", guardedHelloSpec, restrust.SideBoth, false)
	writeRestCell(t, root, "account", accountSpec, restrust.SideClient, false)

	files, err := hexrust.Generate(hexrust.Options{
		Service: "songe-hello",
		SrcDir:  root,
		Cells:   []string{"account", "rest"},
		Ports:   guardedCrateRootPorts,
		Wiring: []byte(`binary: songe-hello-node
drivers:
  rest: { enabled: true }
`),
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	byPath := map[string]string{}
	for _, f := range files {
		byPath[f.Path] = f.Content
	}

	for path, want := range map[string]string{
		"src/port/zz_generated_ticket_verifier.rs": "pub trait TicketVerifier: Send + Sync {",
		"src/port/zz_generated_token_source.rs":    "pub trait TokenSource: Send + Sync {",
		"src/types/zz_generated_subject.rs":        "pub struct Subject {",
	} {
		content, emitted := byPath[path]
		if !emitted {
			t.Errorf("%s was not emitted at the crate root", path)

			continue
		}

		if !strings.HasPrefix(content, "// Code generated by hexagonal-rust (forge-dev-codegen). DO NOT EDIT.") || !strings.Contains(content, want) {
			t.Errorf("%s lacks the header or %q\n%s", path, want, content)
		}
	}

	portMod := byPath["src/port/mod.rs"]

	for _, want := range []string{
		"pub mod zz_generated_ticket_verifier;",
		"pub mod zz_generated_token_source;",
		"pub use zz_generated_ticket_verifier as ticket_verifier;",
		"pub use zz_generated_token_source as token_source;",
	} {
		if !strings.Contains(portMod, want) {
			t.Errorf("src/port/mod.rs lacks %q\n%s", want, portMod)
		}
	}

	if !strings.Contains(byPath["src/types/mod.rs"], "pub use zz_generated_subject as subject;") {
		t.Errorf("src/types/mod.rs does not mount the subject\n%s", byPath["src/types/mod.rs"])
	}

	main := byPath["src/bin/zz_generated_songe_hello_node.rs"]

	if strings.Count(main, "use songe_hello::port::ticket_verifier::TicketVerifier;") != 1 {
		t.Errorf("main imports the ticket verifier other than once\n%s", main)
	}

	spec := byPath[hexrust.ConfigSpecFile]

	if strings.Count(spec, "ticket_verifier:") != 1 {
		t.Errorf("the config spec holds the ticket verifier choice other than once\n%s", spec)
	}
}

func TestACrateWithNoAuthAndNoClientEmitsNoRootPort(t *testing.T) {
	root := standUpCells(t, "grpc", "rest", "udp")
	files := generateHello(t, root, helloWiring)

	for _, unexpected := range []string{
		"src/port/zz_generated_ticket_verifier.rs",
		"src/port/zz_generated_token_source.rs",
		"src/types/zz_generated_subject.rs",
	} {
		if _, emitted := files[unexpected]; emitted {
			t.Errorf("%s was emitted with no cell requiring it", unexpected)
		}
	}
}

func TestTheConfigSpecHoldsTheChoiceOfAPortOnlyAnAdapterConsumes(t *testing.T) {
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

	var spec string
	for _, f := range files {
		if f.Path == hexrust.ConfigSpecFile {
			spec = f.Content
		}
	}

	for _, want := range []string{
		"token_source:", "token_source_file_path:", "greeting_client:", "greeting_client_rest_greeting_client_base_url:",
	} {
		if !strings.Contains(spec, want) {
			t.Errorf("the config spec lacks %q\n%s", want, spec)
		}
	}
}

func TestAPortOnlyAnAdapterConsumesWithNoCandidateIsRefusedNamingTheAdapter(t *testing.T) {
	root := standUpGuardedCells(t, false)

	_, err := hexrust.Generate(hexrust.Options{
		Service: "songe-hello",
		SrcDir:  root,
		Cells:   []string{"rest", "tui"},
		Ports:   guardedCrateRootPorts,
		Wiring:  []byte(guardedWiring),
	})
	if err == nil || !strings.Contains(err.Error(), `adapter "rest_greeting_client" consumes port "TokenSource" and no cell provides an adapter for it`) {
		t.Fatalf("the missing token source adapter was not refused by name: %v", err)
	}
}

func TestAdaptersThatConsumeEachOthersPortsInACycleAreRefused(t *testing.T) {
	root := standUpCells(t, "rest")

	writeCell(t, root, "ws", cellmanifest.Manifest{
		Version:   cellmanifest.Version,
		Cell:      "ws",
		Generator: "a test",
		Provides: cellmanifest.Provides{
			Drivers: []cellmanifest.Driver{{
				Name:     "ws",
				Type:     "WsDriver",
				Module:   "ws::driver::ws_driver",
				Requires: []string{"GreetingController"},
				Ports:    []string{"Left"},
			}},
			Adapters: []cellmanifest.Adapter{
				{Name: "left_memory", Type: "LeftMemory", Module: "ws::adapter::left_memory", Implements: "Left", Ports: []string{"Right"}},
				{Name: "right_memory", Type: "RightMemory", Module: "ws::adapter::right_memory", Implements: "Right", Ports: []string{"Left"}},
			},
			Ports: []cellmanifest.Port{
				{Trait: "Left", Module: "ws::port::left"},
				{Trait: "Right", Module: "ws::port::right"},
			},
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
	if err == nil || !strings.Contains(err.Error(), "the adapters of Left, Right consume each other in a cycle") {
		t.Fatalf("the cycle was not refused by name: %v", err)
	}
}

func secretRefusal(t *testing.T, ports map[string][]string, want string) {
	t.Helper()

	root := t.TempDir()
	writeRestCell(t, root, "rest", guardedHelloSpec, restrust.SideServer, true)

	_, err := hexrust.Generate(hexrust.Options{
		Service: "songe-hello",
		SrcDir:  root,
		Cells:   []string{"rest"},
		Ports:   ports,
		Wiring: []byte(`binary: songe-hello-node
drivers:
  rest: { enabled: true }
`),
	})
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("wanted a refusal naming %q, got %v", want, err)
	}
}

func TestACrateRootPortTheRootNeverEmitsIsRefusedByName(t *testing.T) {
	secretRefusal(t, map[string][]string{"GreetingStore": {"secret"}}, `crate root port "GreetingStore": the crate root emits an adapter for TicketVerifier only`)
}

func TestAnUnknownCrateRootAdapterKindIsRefusedWithTheKindsThatAreAllowed(t *testing.T) {
	secretRefusal(t, map[string][]string{"TicketVerifier": {"vault"}}, `adapter kind "vault", a crate root adapter is one of secret`)
}

func TestACrateRootPortWithAnEmptyAdaptersListIsRefusedByName(t *testing.T) {
	secretRefusal(t, map[string][]string{"TicketVerifier": {}}, "the adapters list is empty")
}
