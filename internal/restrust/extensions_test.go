package restrust_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/restrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/cellmanifest"
)

const guardedSpec = `
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
      x-ports: [GreetingStore]
      x-auth: bearer
      x-stream: events
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
    GreetingEvent:
      type: object
      required: [id, count]
      properties:
        id:
          type: string
        count:
          type: integer
`

func generateSpec(t *testing.T, spec string, opts restrust.Options) map[string]restrust.File {
	t.Helper()

	files, err := restrust.Generate([]byte(spec), opts)
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	byPath := map[string]restrust.File{}
	for _, f := range files {
		byPath[f.Path] = f
	}

	return byPath
}

func paths(files map[string]restrust.File) []string {
	out := make([]string, 0, len(files))
	for p := range files {
		out = append(out, p)
	}

	sort.Strings(out)

	return out
}

func TestAnXAuthOperationGuardsTheHandlerWithATicketVerifierAndHandsTheSubjectToTheController(t *testing.T) {
	files := generateSpec(t, guardedSpec, restrust.Options{Service: "songe-hello"})

	driver := files["driver/zz_generated_http_driver.rs"].Content

	for _, want := range []string{
		"use crate::rest::port::ticket_verifier::{TicketVerifier, TicketVerifierError};",
		"pub(crate) ticket_verifier: Arc<dyn TicketVerifier + Send + Sync>,",
		"ticket_verifier: Arc<dyn TicketVerifier + Send + Sync>) -> Self {",
		"fn authenticate(state: &HttpState, headers: &HeaderMap) -> Result<Subject, Rejection> {",
		`reject(StatusCode::UNAUTHORIZED, "authentication", error.to_string())`,
		"async fn count_greeting(State(state): State<HttpState>, headers: HeaderMap, Path(id): Path<String>)",
		"let subject = authenticate(&state, &headers)?;",
		".count_greeting(subject, &id)",
	} {
		if !strings.Contains(driver, want) {
			t.Errorf("the driver lacks %q\n%s", want, driver)
		}
	}

	if strings.Contains(driver, "async fn get_greeting(State(state): State<HttpState>, headers: HeaderMap") {
		t.Errorf("an operation without x-auth gained a header extractor\n%s", driver)
	}

	controller := files["controller/zz_generated_greeting_controller.rs"].Content

	for _, want := range []string{
		"use crate::rest::types::subject::Subject;",
		"fn count_greeting(&self, subject: Subject, id: &str) -> Result<Greeting, GreetingControllerError>;",
		"fn get_greeting(&self, id: &str) -> Result<Greeting, GreetingControllerError>;",
	} {
		if !strings.Contains(controller, want) {
			t.Errorf("the controller lacks %q\n%s", want, controller)
		}
	}

	port := files["port/zz_generated_ticket_verifier.rs"].Content

	for _, want := range []string{
		"#[cfg_attr(test, mockall::automock)]",
		"fn verify(&self, token: &str) -> Result<Subject, TicketVerifierError>;",
		"Refused { reason: String },",
	} {
		if !strings.Contains(port, want) {
			t.Errorf("the ticket verifier port lacks %q\n%s", want, port)
		}
	}

	if !strings.Contains(files["types/zz_generated_subject.rs"].Content, "pub struct Subject {") {
		t.Errorf("the subject type is missing\n%s", files["types/zz_generated_subject.rs"].Content)
	}
}

func TestAnXStreamOperationAnswersAnEventStreamFromASubscribePortTheControllerConsumes(t *testing.T) {
	files := generateSpec(t, guardedSpec, restrust.Options{Service: "songe-hello"})

	driver := files["driver/zz_generated_http_driver.rs"].Content

	for _, want := range []string{
		"fn event_stream<T, W>(events: std::sync::mpsc::Receiver<T>) -> Response",
		`HeaderValue::from_static("text/event-stream")`,
		"async fn stream_greeting_events(State(state): State<HttpState>, headers: HeaderMap, Path(id): Path<String>) -> Result<Response, Rejection> {",
		"Ok(event_stream::<GreetingEvent, GreetingEventWire>(events))",
	} {
		if !strings.Contains(driver, want) {
			t.Errorf("the driver lacks %q\n%s", want, driver)
		}
	}

	controller := files["controller/zz_generated_greeting_controller.rs"].Content

	for _, want := range []string{
		"use crate::rest::port::greeting_event_subscribe::{GreetingEventSubscribe, GreetingEventSubscribeError};",
		"fn stream_greeting_events(&self, subject: Subject, id: &str) -> Result<std::sync::mpsc::Receiver<GreetingEvent>, GreetingControllerError>;",
		"pub(crate) greeting_event_subscribe: Arc<dyn GreetingEventSubscribe + Send + Sync>,",
		`#[error("subscribing to greeting_event events for {id:?}")]`,
	} {
		if !strings.Contains(controller, want) {
			t.Errorf("the controller lacks %q\n%s", want, controller)
		}
	}

	port := files["port/zz_generated_greeting_event_subscribe.rs"].Content

	if !strings.Contains(port, "fn subscribe(&self, key: &str) -> Result<std::sync::mpsc::Receiver<GreetingEvent>, GreetingEventSubscribeError>;") {
		t.Errorf("the subscribe port lacks its method\n%s", port)
	}
}

func TestTheServerManifestListsTheVerifierAndSubscribePortsAndTheDriverConsumesTheVerifier(t *testing.T) {
	files := generateSpec(t, guardedSpec, restrust.Options{Service: "songe-hello"})

	m, err := cellmanifest.Parse([]byte(files[cellmanifest.FileName].Content))
	if err != nil {
		t.Fatalf("parsing the manifest: %v", err)
	}

	if !reflect.DeepEqual(m.Requires.Ports, []string{"TicketVerifier", "GreetingEventSubscribe"}) {
		t.Errorf("requires.ports = %q", m.Requires.Ports)
	}

	if !reflect.DeepEqual(m.Provides.Drivers[0].Ports, []string{"TicketVerifier"}) {
		t.Errorf("driver ports = %q", m.Provides.Drivers[0].Ports)
	}

	if !reflect.DeepEqual(m.Provides.Controllers[0].Ports, []string{"GreetingEventSubscribe", "GreetingStore"}) {
		t.Errorf("controller ports = %q", m.Provides.Controllers[0].Ports)
	}

	traits := []string{}
	for _, p := range m.Provides.Ports {
		traits = append(traits, p.Trait+" "+p.Module)
	}

	want := []string{
		"TicketVerifier rest::port::ticket_verifier",
		"GreetingStore rest::port::greeting_store",
		"GreetingEventSubscribe rest::port::greeting_event_subscribe",
	}

	if !reflect.DeepEqual(traits, want) {
		t.Errorf("ports\n got %q\nwant %q", traits, want)
	}
}

func TestTheClientSideEmitsAClientPortATokenSourcePortAndAReqwestAdapterAndNoDriver(t *testing.T) {
	files := generateSpec(t, guardedSpec, restrust.Options{Service: "songe-hello", Side: restrust.SideClient})

	want := []string{
		"adapter/mod.rs",
		"adapter/zz_generated_greeting_rest_client.rs",
		"adapter/zz_generated_wire.rs",
		"controller/mod.rs",
		"driver/mod.rs",
		"mod.rs",
		"port/mod.rs",
		"port/zz_generated_greeting_client.rs",
		"port/zz_generated_token_source.rs",
		"types/mod.rs",
		"types/zz_generated_create_greeting_request.rs",
		"types/zz_generated_greeting.rs",
		"types/zz_generated_greeting_event.rs",
		"zz_generated_cell.yaml",
	}

	if got := paths(files); !reflect.DeepEqual(got, want) {
		t.Fatalf("emitted paths\n got %q\nwant %q", got, want)
	}

	port := files["port/zz_generated_greeting_client.rs"].Content

	for _, want := range []string{
		"#[cfg_attr(test, mockall::automock)]",
		"pub trait GreetingClient: Send + Sync {",
		"fn create_greeting(&self, body: CreateGreetingRequest) -> Result<Greeting, GreetingClientError>;",
		"fn count_greeting(&self, id: &str) -> Result<Greeting, GreetingClientError>;",
		"fn stream_greeting_events(&self, id: &str) -> Result<std::sync::mpsc::Receiver<GreetingEvent>, GreetingClientError>;",
		"Authentication { operation: String, message: String },",
		"RateLimiting { operation: String, message: String },",
	} {
		if !strings.Contains(port, want) {
			t.Errorf("the client port lacks %q\n%s", want, port)
		}
	}

	adapter := files["adapter/zz_generated_greeting_rest_client.rs"].Content

	for _, want := range []string{
		"pub fn new(config: GreetingRestClientConfig, token_source: Arc<dyn TokenSource + Send + Sync>) -> Self {",
		`.request(reqwest::Method::POST, format!("{}/greetings/{}/count", self.base_url, id))`,
		"let request = request.bearer_auth(self.bearer(operation)?);",
		"self.block(stream_events::<GreetingEventWire, GreetingEvent>(operation, request))",
		"self.block(answer::<GreetingWire>(operation, request)).map(Greeting::from)",
		`"authentication" => GreetingClientError::Authentication { operation, message: wire.message },`,
	} {
		if !strings.Contains(adapter, want) {
			t.Errorf("the client adapter lacks %q\n%s", want, adapter)
		}
	}

	if strings.Contains(files["driver/mod.rs"].Content, "pub mod") {
		t.Errorf("the client side emitted a driver\n%s", files["driver/mod.rs"].Content)
	}

	m, err := cellmanifest.Parse([]byte(files[cellmanifest.FileName].Content))
	if err != nil {
		t.Fatalf("parsing the manifest: %v", err)
	}

	if len(m.Provides.Drivers) != 0 || len(m.Provides.Controllers) != 0 {
		t.Errorf("the client side provides drivers %+v and controllers %+v", m.Provides.Drivers, m.Provides.Controllers)
	}

	if len(m.Provides.Adapters) != 1 || m.Provides.Adapters[0].Name != "rest_client" || !reflect.DeepEqual(m.Provides.Adapters[0].Ports, []string{"TokenSource"}) {
		t.Errorf("adapters = %+v", m.Provides.Adapters)
	}

	traits := []string{}
	for _, p := range m.Provides.Ports {
		traits = append(traits, p.Trait)
	}

	if !reflect.DeepEqual(traits, []string{"TokenSource", "GreetingClient"}) {
		t.Errorf("ports = %q", traits)
	}
}

func TestTheServerSideIsTheDefaultAndEmitsNoClient(t *testing.T) {
	files := generateSpec(t, helloSpec, restrust.Options{Service: "songe-hello"})

	for _, unexpected := range []string{
		"adapter/zz_generated_greeting_rest_client.rs",
		"port/zz_generated_greeting_client.rs",
		"port/zz_generated_token_source.rs",
		"port/zz_generated_ticket_verifier.rs",
		"types/zz_generated_subject.rs",
	} {
		if _, emitted := files[unexpected]; emitted {
			t.Errorf("%s was emitted for a server side cell without x-auth", unexpected)
		}
	}
}

func TestBothSidesEmitTheDriverAndTheClientInOneCell(t *testing.T) {
	files := generateSpec(t, guardedSpec, restrust.Options{Service: "songe-hello", Side: restrust.SideBoth})

	for _, want := range []string{
		"driver/zz_generated_http_driver.rs",
		"driver/zz_generated_wire.rs",
		"adapter/zz_generated_wire.rs",
		"adapter/zz_generated_greeting_rest_client.rs",
		"adapter/zz_generated_greeting_sqlite.rs",
		"port/zz_generated_greeting_client.rs",
		"port/zz_generated_token_source.rs",
		"port/zz_generated_ticket_verifier.rs",
		"port/zz_generated_greeting_event_subscribe.rs",
	} {
		if _, emitted := files[want]; !emitted {
			t.Errorf("%s was not emitted for a both sides cell", want)
		}
	}
}

func TestASideThatIsNotServerClientOrBothIsRefused(t *testing.T) {
	_, err := restrust.Generate([]byte(helloSpec), restrust.Options{Service: "songe-hello", Side: "proxy"})
	if err == nil || !strings.Contains(err.Error(), `side "proxy"`) {
		t.Fatalf("side proxy was not refused by name: %v", err)
	}
}

func TestAnXAuthValueOtherThanBearerIsRefused(t *testing.T) {
	spec := strings.Replace(guardedSpec, "x-auth: bearer\n      x-stream: events", "x-auth: basic\n      x-stream: events", 1)

	_, err := restrust.Generate([]byte(spec), restrust.Options{Service: "songe-hello"})
	if err == nil || !strings.Contains(err.Error(), `x-auth is "basic"`) {
		t.Fatalf("x-auth basic was not refused by name: %v", err)
	}
}

func TestAnXStreamOperationThatIsNotAGetIsRefused(t *testing.T) {
	spec := strings.Replace(guardedSpec, "x-auth: bearer\n      parameters:", "x-auth: bearer\n      x-stream: events\n      parameters:", 1)

	_, err := restrust.Generate([]byte(spec), restrust.Options{Service: "songe-hello"})
	if err == nil || !strings.Contains(err.Error(), "x-stream is only allowed on a GET operation") {
		t.Fatalf("x-stream on a POST was not refused: %v", err)
	}
}

func TestAnXStreamOperationWithoutAnEventStreamSchemaIsRefused(t *testing.T) {
	spec := strings.Replace(guardedSpec, "            text/event-stream:\n", "            application/json:\n", 1)

	_, err := restrust.Generate([]byte(spec), restrust.Options{Service: "songe-hello"})
	if err == nil || !strings.Contains(err.Error(), "text/event-stream schema") {
		t.Fatalf("a stream operation without an event schema was not refused: %v", err)
	}
}

func TestXPortsMayNameTheSubscribePortOfItsOwnStreamAndNoOther(t *testing.T) {
	named := strings.Replace(guardedSpec, "x-ports: [GreetingStore]\n      x-auth: bearer\n      x-stream: events", "x-ports: [GreetingStore, GreetingEventSubscribe]\n      x-auth: bearer\n      x-stream: events", 1)

	if _, err := restrust.Generate([]byte(named), restrust.Options{Service: "songe-hello"}); err != nil {
		t.Fatalf("naming the subscribe port of the stream operation was refused: %v", err)
	}

	foreign := strings.Replace(guardedSpec, "x-ports: [GreetingStore]\n      x-auth: bearer\n      parameters:", "x-ports: [GreetingStore, GreetingEventSubscribe]\n      x-auth: bearer\n      parameters:", 1)

	_, err := restrust.Generate([]byte(foreign), restrust.Options{Service: "songe-hello"})
	if err == nil || !strings.Contains(err.Error(), "a subscribe port is <Event>Subscribe of an x-stream operation") {
		t.Fatalf("a subscribe port on a plain operation was not refused: %v", err)
	}
}

const guardedCrateManifest = `[workspace]

[package]
name = "songe-hello"
version = "0.1.0"
edition = "2021"

[dependencies]
axum = "0.8"
http-body-util = { version = "0.1", features = ["channel"] }
reqwest = { version = "0.13", default-features = false, features = ["json", "rustls"] }
rusqlite = { version = "0.40", features = ["bundled"] }
serde = { version = "1", features = ["derive"] }
serde_json = "1"
thiserror = "2"
tokio = { version = "1", features = ["full"] }

[dev-dependencies]
mockall = "0.15"
`

const guardedGreetingControllerImpl = `use crate::rest::controller::{
    GreetingController, GreetingControllerError, GreetingControllerImpl,
};
use crate::rest::types::create_greeting_request::CreateGreetingRequest;
use crate::rest::types::greeting::Greeting;
use crate::rest::types::greeting_event::GreetingEvent;
use crate::rest::types::subject::Subject;

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

    fn count_greeting(&self, subject: Subject, id: &str) -> Result<Greeting, GreetingControllerError> {
        let mut greeting = self.get_greeting(id)?;
        greeting.count += 1;
        greeting.name = subject.id;

        Ok(greeting)
    }

    fn stream_greeting_events(
        &self,
        _subject: Subject,
        id: &str,
    ) -> Result<std::sync::mpsc::Receiver<GreetingEvent>, GreetingControllerError> {
        self.greeting_event_subscribe
            .subscribe(id)
            .map_err(|source| GreetingControllerError::GreetingEventSubscribe {
                id: id.to_string(),
                source,
            })
    }
}
`

func TestTheGuardedCellWithBothSidesCompilesOnceTheUserWritesTheControllerImpl(t *testing.T) {
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Skip("cargo is not on PATH")
	}

	files, err := restrust.Generate([]byte(guardedSpec), restrust.Options{Service: "songe-hello", Side: restrust.SideBoth})
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

	write("Cargo.toml", guardedCrateManifest)
	write("src/lib.rs", cellCrateLib)

	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".rs") {
			continue
		}

		write(filepath.Join("src", "rest", f.Path), f.Content)
	}

	write("src/rest/controller/greeting_controller.rs", guardedGreetingControllerImpl)

	cmd := exec.Command(cargo, "clippy", "--workspace", "--all-targets", "--", "-D", "warnings")
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
