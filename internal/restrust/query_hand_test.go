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

const queriedSpec = `
openapi: 3.1.0
info:
  title: Hello API
  version: 1.0.0
paths:
  /greetings:
    get:
      operationId: listGreetings
      x-controller: greeting
      x-ports:
        - GreetingStore
        - kind: clock
          name: GreetingClock
          instant: Instant
          span: Span
          adapters: [memory]
      parameters:
        - name: name
          in: query
          required: true
          schema:
            type: string
        - name: after
          in: query
          schema:
            type: string
        - name: limit
          in: query
          schema:
            type: integer
      responses:
        "200":
          description: The page
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/GreetingPage"
        "422":
          description: Refused
  /greetings/{id}:
    get:
      operationId: getGreeting
      x-controller: greeting
      x-ports: [GreetingStore, GreetingClock]
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
        - name: at
          in: query
          required: true
          schema:
            type: integer
      responses:
        "200":
          description: The greeting
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
        lookups:
          - { by: [name], answers: page }
        adapters: [sqlite, memory]
      required: [id, name, count]
      properties:
        id:
          type: string
        name:
          type: string
        count:
          type: integer
    GreetingPage:
      type: object
      required: [greetings, at]
      properties:
        greetings:
          type: array
          items:
            $ref: "#/components/schemas/Greeting"
        at:
          $ref: "#/components/schemas/Instant"
    Instant:
      type: object
      required: [epochMs]
      properties:
        epochMs:
          type: integer
    Span:
      type: object
      required: [millis]
      properties:
        millis:
          type: integer
    Label:
      type: object
      required: [text]
      properties:
        text:
          type: string
`

func generatedByPath(t *testing.T, spec string) (map[string]string, error) {
	t.Helper()

	files, err := restrust.Generate([]byte(spec), restrust.Options{Service: "songe-hello", Side: restrust.SideBoth})
	if err != nil {
		return nil, err
	}

	byPath := map[string]string{}
	for _, f := range files {
		byPath[f.Path] = f.Content
	}

	return byPath, nil
}

func queriedFiles(t *testing.T) map[string]string {
	t.Helper()

	byPath, err := generatedByPath(t, queriedSpec)
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	return byPath
}

func wantIn(t *testing.T, byPath map[string]string, path string, wants ...string) {
	t.Helper()

	content, ok := byPath[path]
	if !ok {
		t.Fatalf("no file at %s", path)
	}

	for _, want := range wants {
		if !strings.Contains(content, want) {
			t.Fatalf("%s lacks %q:\n%s", path, want, content)
		}
	}
}

func TestTheDriverReadsEveryQueryParameterAndHandsItToTheControllerInDeclarationOrder(t *testing.T) {
	byPath := queriedFiles(t)

	wantIn(t, byPath, "driver/zz_generated_http_driver.rs",
		"use axum::extract::{Json, Path, Query, State};",
		"type QueryMap = std::collections::HashMap<String, String>;",
		"Query(query): Query<QueryMap>",
		`let name = required_query(&query, "name", StatusCode::UNPROCESSABLE_ENTITY)?;`,
		`let after = query.get("after").cloned();`,
		`let limit = optional_integer(&query, "limit", StatusCode::UNPROCESSABLE_ENTITY)?;`,
		".list_greetings(&name, after, limit)",
		`let at = required_integer(&query, "at", StatusCode::BAD_REQUEST)?;`,
		".get_greeting(&id, at)",
	)
}

func TestAMissingRequiredQueryParameterIsAValidationRejectionCarryingTheName(t *testing.T) {
	byPath := queriedFiles(t)

	wantIn(t, byPath, "driver/zz_generated_http_driver.rs",
		`format!("validating {name:?}: the query parameter is required"),`,
		`format!("validating {name:?}: {raw:?} is not an integer"),`,
		`"validation",`,
	)
}

func TestTheControllerTraitTakesEveryQueryParameterWithItsOptionality(t *testing.T) {
	byPath := queriedFiles(t)

	wantIn(t, byPath, "controller/zz_generated_greeting_controller.rs",
		"fn list_greetings(&self, name: &str, after: Option<String>, limit: Option<i64>) -> Result<GreetingPage, GreetingControllerError>;",
		"fn get_greeting(&self, id: &str, at: i64) -> Result<Greeting, GreetingControllerError>;",
	)
}

func TestTheClientAdapterBuildsTheQueryStringIntoTheUrlAndSkipsAnAbsentOptionalParameter(t *testing.T) {
	byPath := queriedFiles(t)

	wantIn(t, byPath, "adapter/zz_generated_greeting_rest_client.rs",
		`let mut query: Vec<(&str, String)> = vec![("name", name.to_string())];`,
		"if let Some(value) = after {\n            query.push((\"after\", value));\n        }",
		"if let Some(value) = limit {\n            query.push((\"limit\", value.to_string()));\n        }",
		`with_query(format!("{}/greetings", self.base_url), query)`,
		`let query: Vec<(&str, String)> = vec![("at", at.to_string())];`,
		"fn encode_query(value: &str) -> String {",
	)
}

func TestAClientWithoutAnyQueryParameterCarriesNoQueryHelper(t *testing.T) {
	files, err := restrust.Generate([]byte(guardedSpec), restrust.Options{Service: "songe-hello", Side: restrust.SideBoth})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	for _, f := range files {
		if strings.Contains(f.Content, "fn with_query(") || strings.Contains(f.Content, "type QueryMap") {
			t.Fatalf("%s carries a query helper the cell never uses", f.Path)
		}
	}
}

func TestAQueryParameterThatIsNeitherAStringNorAnIntegerIsRefused(t *testing.T) {
	broken := strings.Replace(queriedSpec, "        - name: after\n          in: query\n          schema:\n            type: string", "        - name: after\n          in: query\n          schema:\n            type: boolean", 1)

	_, err := restrust.Generate([]byte(broken), restrust.Options{Service: "songe-hello"})
	if err == nil || !strings.Contains(err.Error(), `query parameter "after" must be a string or an integer`) {
		t.Fatalf("a boolean query parameter was not refused: %v", err)
	}
}

func TestAQueryParameterDeclaredTwiceIsRefused(t *testing.T) {
	twice := strings.Replace(queriedSpec, "        - name: limit\n          in: query\n          schema:\n            type: integer", "        - name: after\n          in: query\n          schema:\n            type: string", 1)

	_, err := restrust.Generate([]byte(twice), restrust.Options{Service: "songe-hello"})
	if err == nil || !strings.Contains(err.Error(), `query parameter "after" is declared twice`) {
		t.Fatalf("a repeated query parameter was not refused: %v", err)
	}
}

func TestANameDeclaredBothInPathAndInQueryIsRefused(t *testing.T) {
	clashing := strings.Replace(queriedSpec, "        - name: at\n          in: query", "        - name: id\n          in: query", 1)

	_, err := restrust.Generate([]byte(clashing), restrust.Options{Service: "songe-hello"})
	if err == nil || !strings.Contains(err.Error(), "is declared both in path and in query") {
		t.Fatalf("a name in both places was not refused: %v", err)
	}
}

func TestADeclaredPortEmitsItsTraitItsErrorEnumAndItsMockWithoutAnyUserWrittenPortFile(t *testing.T) {
	byPath := queriedFiles(t)

	wantIn(t, byPath, "port/zz_generated_greeting_clock.rs",
		"use crate::rest::types::instant::Instant;",
		"use crate::rest::types::span::Span;",
		"pub enum GreetingClockError {",
		`#[error("calling {method:?} on the greeting_clock port: refused: {reason}")]`,
		"#[cfg_attr(test, mockall::automock)]",
		"pub trait GreetingClock: Send + Sync {",
		"fn now(&self) -> Result<Instant, GreetingClockError>;",
		"fn elapsed(&self, request: Instant) -> Result<Span, GreetingClockError>;",
	)

	wantIn(t, byPath, "port/mod.rs", "pub mod zz_generated_greeting_clock;", "pub use zz_generated_greeting_clock as greeting_clock;")
}

func TestTheControllerHoldsTheDeclaredPortAsABoxedFieldAndCarriesItsErrorArm(t *testing.T) {
	byPath := queriedFiles(t)

	wantIn(t, byPath, "controller/zz_generated_greeting_controller.rs",
		"use crate::rest::port::greeting_clock::{GreetingClock, GreetingClockError};",
		"pub(crate) greeting_clock: Arc<dyn GreetingClock + Send + Sync>,",
		`#[error("calling the greeting_clock port for {id:?}")]`,
		"GreetingClock {\n        id: String,\n        #[source]\n        source: GreetingClockError,\n    },",
	)
}

func TestTheManifestDeclaresTheDeclaredPortAndTheAdapterItsKindEmits(t *testing.T) {
	byPath := queriedFiles(t)

	raw, ok := byPath[cellmanifest.FileName]
	if !ok {
		t.Fatal("no cell manifest")
	}

	m, err := cellmanifest.Parse([]byte(raw))
	if err != nil {
		t.Fatalf("reading the manifest: %v", err)
	}

	declared := false
	for _, p := range m.Provides.Ports {
		declared = declared || (p.Trait == "GreetingClock" && p.Module == "rest::port::greeting_clock")
	}

	if !declared {
		t.Fatalf("the manifest never declares the port: %+v", m.Provides.Ports)
	}

	answered := false
	for _, a := range m.Provides.Adapters {
		answered = answered || a.Implements == "GreetingClock"
	}

	if !answered {
		t.Fatalf("the manifest never provides the adapter the clock kind emits: %+v", m.Provides.Adapters)
	}
}

func TestTheControllerPortsOfTheManifestHoldTheDeclaredPortBesideTheStore(t *testing.T) {
	byPath := queriedFiles(t)

	m, err := cellmanifest.Parse([]byte(byPath[cellmanifest.FileName]))
	if err != nil {
		t.Fatalf("reading the manifest: %v", err)
	}

	if len(m.Provides.Controllers) != 1 {
		t.Fatalf("want one controller, got %d", len(m.Provides.Controllers))
	}

	if !reflect.DeepEqual(m.Provides.Controllers[0].Ports, []string{"GreetingClock", "GreetingStore"}) {
		t.Fatalf("want the declared port beside the store, got %v", m.Provides.Controllers[0].Ports)
	}
}

const clockDeclaration = "        - kind: clock\n          name: GreetingClock\n          instant: Instant\n          span: Span\n          adapters: [memory]"

const verifierDeclaration = "        - kind: verifier\n          name: GoogleVerifier\n          subject: GoogleIdentity\n          adapters: [secret]"

const googleIdentitySchema = `    GoogleIdentity:
      type: object
      description: What the Google verifier reads off a token it accepted. No client ever sends it.
      required: [subject, email]
      properties:
        subject:
          type: string
        email:
          type: string
`

func specWithAGoogleVerifier(t *testing.T) string {
	t.Helper()

	spec := strings.Replace(queriedSpec, clockDeclaration, bothClocksDeclaration+"\n"+verifierDeclaration, 1)
	if spec == queriedSpec {
		t.Fatal("the clock declaration was never replaced, the fixture moved")
	}

	withSchema := strings.Replace(spec, "    Label:", googleIdentitySchema+"    Label:", 1)
	if withSchema == spec {
		t.Fatal("the Label schema was never found, the fixture moved")
	}

	return withSchema
}

const systemClockDeclaration = "        - kind: clock\n          name: GreetingClock\n          instant: Instant\n          span: Span\n          adapters: [system]"

const bothClocksDeclaration = "        - kind: clock\n          name: GreetingClock\n          instant: Instant\n          span: Span\n          adapters: [memory, system]"

func TestAClockPortKindEmitsItsMemoryAdapterAndProvidesItInsteadOfRequiringThePort(t *testing.T) {
	files := queriedFiles(t)

	clock, emitted := files["adapter/zz_generated_greeting_clock_memory.rs"]
	if !emitted {
		t.Fatalf("the memory clock was not emitted, the cell wrote %d files", len(files))
	}

	for _, want := range []string{"fn now(&self)", "fn elapsed(&self, request: Instant)", "is not a moment this clock has reached", "AtomicI64"} {
		if !strings.Contains(clock, want) {
			t.Errorf("the memory clock never carried %q:\n%s", want, clock)
		}
	}

	m, err := cellmanifest.Parse([]byte(files[cellmanifest.FileName]))
	if err != nil {
		t.Fatalf("parsing the manifest: %v", err)
	}

	for _, required := range m.Requires.Ports {
		if required == "GreetingClock" {
			t.Error("the cell provides the clock adapter and still requires the port")
		}
	}
}

func TestTheSystemClockReadsTheRealClockThroughTheStandardLibraryAndTakesNoConfiguration(t *testing.T) {
	spec := strings.Replace(queriedSpec, clockDeclaration, systemClockDeclaration, 1)

	files, err := generatedByPath(t, spec)
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	system, emitted := files["adapter/zz_generated_greeting_clock_system.rs"]
	if !emitted {
		t.Fatalf("the system clock was not emitted, the cell wrote %d files", len(files))
	}

	for _, want := range []string{
		"use std::time::{SystemTime, UNIX_EPOCH};",
		"SystemTime::now()",
		".duration_since(UNIX_EPOCH)",
		"pub struct GreetingClockSystemConfig {}",
		"impl GreetingClock for GreetingClockSystem",
	} {
		if !strings.Contains(system, want) {
			t.Errorf("the system clock never carried %q:\n%s", want, system)
		}
	}

	if strings.Contains(system, "AtomicI64") {
		t.Error("the system clock counts its own moments instead of reading the machine")
	}

	m, err := cellmanifest.Parse([]byte(files[cellmanifest.FileName]))
	if err != nil {
		t.Fatalf("parsing the manifest: %v", err)
	}

	for _, adapter := range m.Provides.Adapters {
		if adapter.Name != "greeting_clock_system" {
			continue
		}

		if len(adapter.Config) != 0 {
			t.Fatalf("the system clock declares configuration, a machine clock starts where the machine is: %v", adapter.Config)
		}

		return
	}

	t.Fatalf("the manifest never provides the system clock: %+v", m.Provides.Adapters)
}

func TestBothClockAdaptersStandBesideEachOtherBehindOnePort(t *testing.T) {
	spec := strings.Replace(queriedSpec, clockDeclaration, bothClocksDeclaration, 1)

	files, err := generatedByPath(t, spec)
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	for _, want := range []string{
		"adapter/zz_generated_greeting_clock_memory.rs",
		"adapter/zz_generated_greeting_clock_system.rs",
	} {
		if _, emitted := files[want]; !emitted {
			t.Errorf("the cell never wrote %q", want)
		}
	}

	mod := files["adapter/mod.rs"]
	for _, want := range []string{"pub mod zz_generated_greeting_clock_memory;", "pub mod zz_generated_greeting_clock_system;"} {
		if !strings.Contains(mod, want) {
			t.Errorf("the adapter mod.rs never mounted %q:\n%s", want, mod)
		}
	}
}

func TestAClockTakesItsPropertyNamesFromTheSchemasItDeclaresAndNeverFromTheEngine(t *testing.T) {
	spec := strings.Replace(queriedSpec, "        epochMs:\n          type: integer", "        atMillis:\n          type: integer", 1)
	spec = strings.Replace(spec, "required: [epochMs]", "required: [atMillis]", 1)
	spec = strings.Replace(spec, "        millis:\n          type: integer", "        howLong:\n          type: integer", 1)
	spec = strings.Replace(spec, "required: [millis]", "required: [howLong]", 1)

	files, err := generatedByPath(t, spec)
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	clock := files["adapter/zz_generated_greeting_clock_memory.rs"]

	for _, want := range []string{
		"pub at_millis: i64,",
		"at_millis: AtomicI64::new(config.at_millis),",
		"Instant {\n            at_millis:",
		"let how_long = now",
		"Ok(Span { how_long })",
	} {
		if !strings.Contains(clock, want) {
			t.Errorf("the memory clock never carried %q:\n%s", want, clock)
		}
	}

	for _, invented := range []string{"epoch_ms", "Span { millis }", "let millis = now"} {
		if strings.Contains(clock, invented) {
			t.Errorf("the memory clock still spells %q, a name the engine invented:\n%s", invented, clock)
		}
	}

	m, err := cellmanifest.Parse([]byte(files[cellmanifest.FileName]))
	if err != nil {
		t.Fatalf("parsing the manifest: %v", err)
	}

	for _, adapter := range m.Provides.Adapters {
		if adapter.Name != "greeting_clock_memory" {
			continue
		}

		if _, named := adapter.Config["at_millis"]; !named {
			t.Fatalf("the memory clock config never took the name the schema declares: %v", adapter.Config)
		}

		return
	}

	t.Fatal("the manifest never provides the memory clock")
}

func TestAVerifierPortIsDeclaredOnAnOperationAndTheEngineWritesItsTraitItsAdapterAndItsMock(t *testing.T) {
	files, err := generatedByPath(t, specWithAGoogleVerifier(t))
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	port, emitted := files["port/zz_generated_google_verifier.rs"]
	if !emitted {
		t.Fatalf("the verifier port was not emitted, the cell wrote %d files", len(files))
	}

	for _, want := range []string{
		"use crate::rest::types::google_identity::GoogleIdentity;",
		"pub enum GoogleVerifierError {",
		`#[error("calling {method:?} on the google_verifier port: refused: {reason}")]`,
		"#[cfg_attr(test, mockall::automock)]",
		"pub trait GoogleVerifier: Send + Sync {",
		"fn verify(&self, offered: &str) -> Result<GoogleIdentity, GoogleVerifierError>;",
	} {
		if !strings.Contains(port, want) {
			t.Errorf("the verifier port never carried %q:\n%s", want, port)
		}
	}

	adapter, emitted := files["adapter/zz_generated_google_verifier_secret.rs"]
	if !emitted {
		t.Fatal("the verifier secret adapter was not emitted")
	}

	for _, want := range []string{
		"pub struct GoogleVerifierSecretConfig {",
		"pub secret: String,",
		"pub subject: String,",
		"pub email: String,",
		"impl GoogleVerifier for GoogleVerifierSecret {",
		"the offered string does not match the configured secret",
	} {
		if !strings.Contains(adapter, want) {
			t.Errorf("the verifier secret adapter never carried %q:\n%s", want, adapter)
		}
	}
}

func TestAVerifierPortReachesTheManifestAsAPortAnAdapterAndAControllerField(t *testing.T) {
	files, err := generatedByPath(t, specWithAGoogleVerifier(t))
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	m, err := cellmanifest.Parse([]byte(files[cellmanifest.FileName]))
	if err != nil {
		t.Fatalf("parsing the manifest: %v", err)
	}

	declared := false
	for _, p := range m.Provides.Ports {
		declared = declared || (p.Trait == "GoogleVerifier" && p.Module == "rest::port::google_verifier")
	}

	if !declared {
		t.Errorf("the manifest never declares the verifier port: %+v", m.Provides.Ports)
	}

	for _, required := range m.Requires.Ports {
		if required == "GoogleVerifier" {
			t.Error("the cell provides the verifier adapter and still requires the port")
		}
	}

	for _, adapter := range m.Provides.Adapters {
		if adapter.Name != "google_verifier_secret" {
			continue
		}

		if adapter.Implements != "GoogleVerifier" {
			t.Errorf("the verifier adapter implements %q", adapter.Implements)
		}

		for _, field := range []string{"secret", "subject", "email"} {
			if _, named := adapter.Config[field]; !named {
				t.Errorf("the verifier adapter config never named %q: %v", field, adapter.Config)
			}
		}

		controller := files["controller/zz_generated_greeting_controller.rs"]
		if !strings.Contains(controller, "pub(crate) google_verifier: Arc<dyn GoogleVerifier + Send + Sync>,") {
			t.Errorf("the controller never held the verifier:\n%s", controller)
		}

		return
	}

	t.Fatalf("the manifest never provides the verifier adapter: %+v", m.Provides.Adapters)
}

func TestEveryFieldOfTheSecretVerifierAdapterIsRequiredWithNoDefault(t *testing.T) {
	files, err := generatedByPath(t, specWithAGoogleVerifier(t))
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	m, err := cellmanifest.Parse([]byte(files[cellmanifest.FileName]))
	if err != nil {
		t.Fatalf("parsing the manifest: %v", err)
	}

	for _, adapter := range m.Provides.Adapters {
		if adapter.Name != "google_verifier_secret" {
			continue
		}

		for _, field := range []string{"secret", "subject", "email"} {
			declared := adapter.Config[field]
			if !declared.Required || declared.Default != nil {
				t.Errorf("field %q = %+v, want required with no default", field, declared)
			}
		}

		return
	}

	t.Fatalf("the manifest never provides the verifier adapter: %+v", m.Provides.Adapters)
}

func TestOnlyTheSecretFieldOfTheSecretVerifierAdapterIsMarkedSecretInTheManifest(t *testing.T) {
	files, err := generatedByPath(t, specWithAGoogleVerifier(t))
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	m, err := cellmanifest.Parse([]byte(files[cellmanifest.FileName]))
	if err != nil {
		t.Fatalf("parsing the manifest: %v", err)
	}

	for _, adapter := range m.Provides.Adapters {
		if adapter.Name != "google_verifier_secret" {
			continue
		}

		for field, want := range map[string]bool{"secret": true, "subject": false, "email": false} {
			if got := adapter.Config[field].Secret; got != want {
				t.Errorf("field %q secret = %v, want %v", field, got, want)
			}
		}

		return
	}

	t.Fatalf("the manifest never provides the verifier adapter: %+v", m.Provides.Adapters)
}

func TestAControllerHoldingAPortNoBodyReadsYetStillCompiles(t *testing.T) {
	out, err := clippyOverTheVerifierCell(t, controllerImplLeavingTheVerifierUnread)
	if err != nil {
		t.Fatalf("a port the body has not reached yet failed the build, so a crate could never go green until every feature exists: %v\n%s", err, out)
	}
}

func TestAControllerMissingAMethodStillFailsTheBuildSoTheAllowSilencesNothingThatMatters(t *testing.T) {
	out, err := clippyOverTheVerifierCell(t, controllerImplMissingAMethod)
	if err == nil {
		t.Fatalf("a controller missing a method built cleanly, so the allow silenced something that matters:\n%s", out)
	}

	if !strings.Contains(out, "E0046") {
		t.Fatalf("the build failed for some reason other than the missing method:\n%s", out)
	}
}

func TestAVerifierDeclarationIsRefusedWhenItBreaksTheContract(t *testing.T) {
	cases := []struct {
		name    string
		replace string
		want    string
	}{
		{
			name:    "no subject",
			replace: "        - kind: verifier\n          name: GoogleVerifier\n          adapters: [secret]",
			want:    "names no subject",
		},
		{
			name:    "a subject that is no schema",
			replace: "        - kind: verifier\n          name: GoogleVerifier\n          subject: Person\n          adapters: [secret]",
			want:    `names subject "Person", which is not a schema of components.schemas`,
		},
		{
			name:    "a subject carrying something that is not a string",
			replace: "        - kind: verifier\n          name: GoogleVerifier\n          subject: Greeting\n          adapters: [secret]",
			want:    `whose property "count" is not a required string`,
		},
		{
			name:    "an unknown adapter kind",
			replace: "        - kind: verifier\n          name: GoogleVerifier\n          subject: GoogleIdentity\n          adapters: [okta]",
			want:    `adapter kind "okta", a verifier adapter is one of jwks, secret`,
		},
		{
			name:    "no adapters",
			replace: "        - kind: verifier\n          name: GoogleVerifier\n          subject: GoogleIdentity",
			want:    "names no adapters",
		},
		{
			name:    "methods a verifier never declares",
			replace: "        - kind: verifier\n          name: GoogleVerifier\n          subject: GoogleIdentity\n          adapters: [secret]\n          methods:\n            - name: verify\n              reply: GoogleIdentity",
			want:    "declares methods, a verifier answers verify and the engine writes them",
		},
		{
			name:    "the name of a store port",
			replace: "        - kind: verifier\n          name: GreetingStore\n          subject: GoogleIdentity\n          adapters: [secret]",
			want:    "takes the name of a store or subscribe port",
		},
		{
			name:    "an instant that belongs to a clock port",
			replace: "        - kind: verifier\n          name: GoogleVerifier\n          subject: GoogleIdentity\n          instant: Instant\n          adapters: [secret]",
			want:    `verifier port "GoogleVerifier" declares instant, which belongs to the clock kind, a verifier port declares only subject`,
		},
		{
			name:    "a value that belongs to a string port",
			replace: "        - kind: verifier\n          name: GoogleVerifier\n          subject: GoogleIdentity\n          value: GoogleIdentity\n          adapters: [secret]",
			want:    `verifier port "GoogleVerifier" declares value, which belongs to the string kind, a verifier port declares only subject`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			broken := strings.Replace(specWithAGoogleVerifier(t), verifierDeclaration, tc.replace, 1)

			_, err := restrust.Generate([]byte(broken), restrust.Options{Service: "songe-hello"})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want an error carrying %q, got %v", tc.want, err)
			}
		})
	}
}

const jwksVerifierDeclaration = "        - kind: verifier\n          name: GoogleVerifier\n          subject: GoogleIdentity\n          adapters: [jwks, secret]\n          jwks:\n            url: https://www.googleapis.com/oauth2/v3/certs\n            issuers: [https://accounts.google.com, accounts.google.com]"

func specWithAJwksVerifier(t *testing.T) string {
	t.Helper()

	spec := specWithAGoogleVerifier(t)

	withJwks := strings.Replace(spec, verifierDeclaration, jwksVerifierDeclaration, 1)
	if withJwks == spec {
		t.Fatal("the verifier declaration was never replaced, the fixture moved")
	}

	return withJwks
}

func TestAJwksVerifierAdapterIsEmittedBesideTheSecretOneBehindOnePort(t *testing.T) {
	files, err := generatedByPath(t, specWithAJwksVerifier(t))
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	if _, emitted := files["adapter/zz_generated_google_verifier_secret.rs"]; !emitted {
		t.Error("the secret verifier left when the jwks verifier joined it")
	}

	jwks, emitted := files["adapter/zz_generated_google_verifier_jwks.rs"]
	if !emitted {
		t.Fatalf("the jwks verifier was not emitted, the cell wrote %d files", len(files))
	}

	for _, want := range []string{
		`const ISSUERS: &[&str] = &["https://accounts.google.com", "accounts.google.com"];`,
		"pub struct GoogleVerifierJwksConfig {\n    pub audience: String,\n    pub url: String,\n}",
		"#[serde(rename = \"subject\")]\n    subject: String,",
		"#[serde(rename = \"email\")]\n    email: String,",
		"email_verified: Option<bool>,",
		"use crate::rest::port::google_verifier::{GoogleVerifier, GoogleVerifierError};",
		"impl GoogleVerifier for GoogleVerifierJwks {",
		"tokio::task::block_in_place",
	} {
		if !strings.Contains(jwks, want) {
			t.Errorf("the jwks verifier never carried %q:\n%s", want, jwks)
		}
	}

	mod := files["adapter/mod.rs"]
	for _, want := range []string{"pub mod zz_generated_google_verifier_jwks;", "pub mod zz_generated_google_verifier_secret;"} {
		if !strings.Contains(mod, want) {
			t.Errorf("the adapter mod.rs never mounted %q:\n%s", want, mod)
		}
	}

	ports := 0
	for p := range files {
		if strings.HasPrefix(p, "port/zz_generated_google_verifier") {
			ports++
		}
	}

	if ports != 1 {
		t.Errorf("the cell wrote %d verifier ports, two adapters stand behind one port", ports)
	}
}

func TestAJwksVerifierReachesTheManifestWithItsAudienceAndItsUrlConfigKeys(t *testing.T) {
	files, err := generatedByPath(t, specWithAJwksVerifier(t))
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	m, err := cellmanifest.Parse([]byte(files[cellmanifest.FileName]))
	if err != nil {
		t.Fatalf("parsing the manifest: %v", err)
	}

	for _, adapter := range m.Provides.Adapters {
		if adapter.Name != "google_verifier_jwks" {
			continue
		}

		if adapter.Implements != "GoogleVerifier" || adapter.Type != "GoogleVerifierJwks" || adapter.Module != "rest::adapter::google_verifier_jwks" {
			t.Errorf("the jwks adapter is declared as %+v", adapter)
		}

		want := map[string]cellmanifest.ConfigField{
			"audience": {
				Type:        cellmanifest.FieldTypeString,
				Required:    true,
				Description: "The audience a token must name for the google_verifier jwks verifier to accept it",
			},
			"url": {
				Type:        cellmanifest.FieldTypeString,
				Default:     "https://www.googleapis.com/oauth2/v3/certs",
				Description: "The url the google_verifier jwks verifier fetches its published key set from",
			},
		}

		if !reflect.DeepEqual(adapter.Config, want) {
			t.Fatalf("the jwks adapter config is %+v, want %+v", adapter.Config, want)
		}

		return
	}

	t.Fatalf("the manifest never provides the jwks adapter: %+v", m.Provides.Adapters)
}

func TestAJwksDeclarationIsRefusedWhenItBreaksTheContract(t *testing.T) {
	cases := []struct {
		name    string
		replace string
		want    string
	}{
		{
			name:    "jwks listed with no jwks block",
			replace: "        - kind: verifier\n          name: GoogleVerifier\n          subject: GoogleIdentity\n          adapters: [jwks]",
			want:    `verifier port "GoogleVerifier" lists the jwks adapter and names no jwks url`,
		},
		{
			name:    "jwks listed with no url",
			replace: "        - kind: verifier\n          name: GoogleVerifier\n          subject: GoogleIdentity\n          adapters: [jwks]\n          jwks:\n            issuers: [https://accounts.google.com]",
			want:    `verifier port "GoogleVerifier" lists the jwks adapter and names no jwks url`,
		},
		{
			name:    "jwks listed with no issuers",
			replace: "        - kind: verifier\n          name: GoogleVerifier\n          subject: GoogleIdentity\n          adapters: [jwks]\n          jwks:\n            url: https://www.googleapis.com/oauth2/v3/certs",
			want:    `verifier port "GoogleVerifier" lists the jwks adapter and names no jwks issuers`,
		},
		{
			name:    "jwks listed with an empty issuers list",
			replace: "        - kind: verifier\n          name: GoogleVerifier\n          subject: GoogleIdentity\n          adapters: [jwks]\n          jwks:\n            url: https://www.googleapis.com/oauth2/v3/certs\n            issuers: []",
			want:    `verifier port "GoogleVerifier" lists the jwks adapter and names no jwks issuers`,
		},
		{
			name:    "a jwks block on a port whose adapters do not list jwks",
			replace: "        - kind: verifier\n          name: GoogleVerifier\n          subject: GoogleIdentity\n          adapters: [secret]\n          jwks:\n            url: https://www.googleapis.com/oauth2/v3/certs\n            issuers: [https://accounts.google.com]",
			want:    `verifier port "GoogleVerifier" declares a jwks block while its adapters do not list jwks`,
		},
		{
			name:    "a jwks verifier whose subject declares no email",
			replace: "        - kind: verifier\n          name: GoogleVerifier\n          subject: Label\n          adapters: [jwks]\n          jwks:\n            url: https://www.googleapis.com/oauth2/v3/certs\n            issuers: [https://accounts.google.com]",
			want:    `verifier port "GoogleVerifier" lists the jwks adapter and its subject "Label" declares no email property`,
		},
		{
			name:    "a jwks block on a string port",
			replace: "        - kind: string\n          name: GoogleAddress\n          value: GoogleIdentity\n          adapters: [setting]\n          jwks:\n            url: https://www.googleapis.com/oauth2/v3/certs\n            issuers: [https://accounts.google.com]",
			want:    `string port "GoogleAddress" declares jwks, which belongs to the verifier kind`,
		},
		{
			name:    "a jwks block on a clock port",
			replace: "        - kind: clock\n          name: GoogleClock\n          instant: Instant\n          span: Span\n          adapters: [memory]\n          jwks:\n            url: https://www.googleapis.com/oauth2/v3/certs\n            issuers: [https://accounts.google.com]",
			want:    `clock port "GoogleClock" declares jwks, which belongs to the verifier kind`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			broken := strings.Replace(specWithAGoogleVerifier(t), verifierDeclaration, tc.replace, 1)

			_, err := restrust.Generate([]byte(broken), restrust.Options{Service: "songe-hello"})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want an error carrying %q, got %v", tc.want, err)
			}
		})
	}
}

const stringDeclaration = "        - kind: string\n          name: CombatAddress\n          value: Address\n          adapters: [setting]"

const addressSchema = `    Address:
      type: object
      required: [address]
      properties:
        address:
          type: string
          description: The host and port the combat service listens on.
`

func specWithACombatAddress(t *testing.T) string {
	t.Helper()

	spec := strings.Replace(queriedSpec, clockDeclaration, clockDeclaration+"\n"+stringDeclaration, 1)
	if spec == queriedSpec {
		t.Fatal("the clock declaration was never replaced, the fixture moved")
	}

	withSchema := strings.Replace(spec, "    Label:", addressSchema+"    Label:", 1)
	if withSchema == spec {
		t.Fatal("the Label schema was never found, the fixture moved")
	}

	return withSchema
}

func TestAStringPortIsDeclaredOnAnOperationAndTheEngineWritesItsTraitItsAdapterAndItsMock(t *testing.T) {
	files, err := generatedByPath(t, specWithACombatAddress(t))
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	wantIn(t, files, "port/zz_generated_combat_address.rs",
		"use crate::rest::types::address::Address;",
		"pub enum CombatAddressError {",
		"#[cfg_attr(test, mockall::automock)]",
		"pub trait CombatAddress: Send + Sync {",
		"fn get(&self) -> Result<Address, CombatAddressError>;",
	)

	wantIn(t, files, "adapter/zz_generated_combat_address_setting.rs",
		"use crate::rest::types::address::Address;",
		"pub struct CombatAddressSettingConfig {\n    pub address: String,\n}",
		"pub struct CombatAddressSetting {\n    value: Address,\n}",
		"value: Address {\n                address: config.address,\n            },",
		"impl CombatAddress for CombatAddressSetting {",
		"fn get(&self) -> Result<Address, CombatAddressError> {\n        Ok(self.value.clone())\n    }",
	)

	wantIn(t, files, "adapter/mod.rs", "pub mod zz_generated_combat_address_setting;")
}

func TestAStringPortReachesTheManifestAsAPortAnAdapterWithOneRequiredConfigKeyAndAControllerField(t *testing.T) {
	files, err := generatedByPath(t, specWithACombatAddress(t))
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	m, err := cellmanifest.Parse([]byte(files[cellmanifest.FileName]))
	if err != nil {
		t.Fatalf("parsing the manifest: %v", err)
	}

	declared := false
	for _, p := range m.Provides.Ports {
		declared = declared || (p.Trait == "CombatAddress" && p.Module == "rest::port::combat_address")
	}

	if !declared {
		t.Errorf("the manifest never declares the string port: %+v", m.Provides.Ports)
	}

	for _, required := range m.Requires.Ports {
		if required == "CombatAddress" {
			t.Error("the cell provides the setting adapter and still requires the port")
		}
	}

	controller := files["controller/zz_generated_greeting_controller.rs"]
	if !strings.Contains(controller, "pub(crate) combat_address: Arc<dyn CombatAddress + Send + Sync>,") {
		t.Errorf("the controller never held the string port:\n%s", controller)
	}

	for _, adapter := range m.Provides.Adapters {
		if adapter.Name != "combat_address_setting" {
			continue
		}

		if adapter.Implements != "CombatAddress" {
			t.Errorf("the setting adapter implements %q", adapter.Implements)
		}

		want := map[string]cellmanifest.ConfigField{
			"address": {
				Type:        cellmanifest.FieldTypeString,
				Required:    true,
				Description: "The address the combat_address port answers, read as the Address it returns",
			},
		}

		if !reflect.DeepEqual(adapter.Config, want) {
			t.Errorf("the setting adapter config = %+v, want one required string key with no default that is not secret", adapter.Config)
		}

		return
	}

	t.Fatalf("the manifest never provides the setting adapter: %+v", m.Provides.Adapters)
}

func TestAStringDeclarationIsRefusedWhenItBreaksTheContract(t *testing.T) {
	cases := []struct {
		name    string
		replace string
		want    string
	}{
		{
			name:    "no value",
			replace: "        - kind: string\n          name: CombatAddress\n          adapters: [setting]",
			want:    `string port "CombatAddress" names no value, a string declaration names the one schema it answers`,
		},
		{
			name:    "a value that is no schema",
			replace: "        - kind: string\n          name: CombatAddress\n          value: Place\n          adapters: [setting]",
			want:    `string port "CombatAddress" names value "Place", which is not a schema of components.schemas`,
		},
		{
			name:    "two properties",
			replace: "        - kind: string\n          name: CombatAddress\n          value: GreetingPage\n          adapters: [setting]",
			want:    `names value "GreetingPage", which declares 2 properties, a string port answers one string so its value carries one property`,
		},
		{
			name:    "a property that is not a required string",
			replace: "        - kind: string\n          name: CombatAddress\n          value: Instant\n          adapters: [setting]",
			want:    `names value "Instant", whose one property "epochMs" is of type integer, a string port reads one config string so that property is a required string`,
		},
		{
			name:    "a string property that is not required",
			replace: "        - kind: string\n          name: CombatAddress\n          value: MaybeAddress\n          adapters: [setting]",
			want:    `reading GET /greetings: string port "CombatAddress" names value "MaybeAddress", whose one property "address" is optional and of type string, a string port reads one config string so that property is a required string`,
		},
		{
			name:    "no adapters",
			replace: "        - kind: string\n          name: CombatAddress\n          value: Address",
			want:    `string port "CombatAddress" names no adapters`,
		},
		{
			name:    "an empty adapters list",
			replace: "        - kind: string\n          name: CombatAddress\n          value: Address\n          adapters: []",
			want:    `string port "CombatAddress" names an empty adapters list`,
		},
		{
			name:    "an unknown adapter kind",
			replace: "        - kind: string\n          name: CombatAddress\n          value: Address\n          adapters: [env]",
			want:    `adapter kind "env", a string adapter is one of setting`,
		},
		{
			name:    "declared methods",
			replace: "        - kind: string\n          name: CombatAddress\n          value: Address\n          adapters: [setting]\n          methods:\n            - name: get\n              reply: Address",
			want:    "declares methods, a string answers get and the engine writes them",
		},
		{
			name:    "the name of a store port",
			replace: "        - kind: string\n          name: GreetingStore\n          value: Address\n          adapters: [setting]",
			want:    "takes the name of a store or subscribe port",
		},
		{
			name:    "an instant that belongs to a clock port",
			replace: "        - kind: string\n          name: CombatAddress\n          value: Address\n          instant: Instant\n          adapters: [setting]",
			want:    `reading GET /greetings: string port "CombatAddress" declares instant, which belongs to the clock kind, a string port declares only value`,
		},
		{
			name:    "a span that belongs to a clock port",
			replace: "        - kind: string\n          name: CombatAddress\n          value: Address\n          span: Span\n          adapters: [setting]",
			want:    `reading GET /greetings: string port "CombatAddress" declares span, which belongs to the clock kind, a string port declares only value`,
		},
		{
			name:    "a subject that belongs to a verifier port",
			replace: "        - kind: string\n          name: CombatAddress\n          value: Address\n          subject: Address\n          adapters: [setting]",
			want:    `reading GET /greetings: string port "CombatAddress" declares subject, which belongs to the verifier kind, a string port declares only value`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			broken := strings.Replace(withOptionalSchemas(t, specWithACombatAddress(t)), stringDeclaration, tc.replace, 1)

			_, err := restrust.Generate([]byte(broken), restrust.Options{Service: "songe-hello"})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want an error carrying %q, got %v", tc.want, err)
			}
		})
	}
}

func TestAClockDeclarationIsRefusedWhenItBreaksTheContract(t *testing.T) {
	cases := []struct {
		name    string
		replace string
		want    string
	}{
		{
			name:    "no instant",
			replace: "        - kind: clock\n          name: GreetingClock\n          span: Span\n          adapters: [memory]",
			want:    "names no instant",
		},
		{
			name:    "no span",
			replace: "        - kind: clock\n          name: GreetingClock\n          instant: Instant\n          adapters: [memory]",
			want:    "names no span",
		},
		{
			name:    "an instant that is no schema",
			replace: "        - kind: clock\n          name: GreetingClock\n          instant: Moment\n          span: Span\n          adapters: [memory]",
			want:    `names instant "Moment", which is not a schema of components.schemas`,
		},
		{
			name:    "one schema for both",
			replace: "        - kind: clock\n          name: GreetingClock\n          instant: Instant\n          span: Instant\n          adapters: [memory]",
			want:    "as both its instant and its span",
		},
		{
			name:    "no adapters",
			replace: "        - kind: clock\n          name: GreetingClock\n          instant: Instant\n          span: Span",
			want:    "names no adapters",
		},
		{
			name:    "an unknown adapter kind",
			replace: "        - kind: clock\n          name: GreetingClock\n          instant: Instant\n          span: Span\n          adapters: [ntp]",
			want:    `adapter kind "ntp", a clock adapter is one of memory, system`,
		},
		{
			name:    "an instant carrying more than one property",
			replace: "        - kind: clock\n          name: GreetingClock\n          instant: Greeting\n          span: Span\n          adapters: [memory]",
			want:    `names instant "Greeting", which declares 3 properties`,
		},
		{
			name:    "a span whose one property is no integer",
			replace: "        - kind: clock\n          name: GreetingClock\n          instant: Instant\n          span: Label\n          adapters: [memory]",
			want:    `whose one property "text" is of type string, a clock counts in whole units so that property is a required integer`,
		},
		{
			name:    "an integer property that is not required",
			replace: "        - kind: clock\n          name: GreetingClock\n          instant: MaybeInstant\n          span: Span\n          adapters: [memory]",
			want:    `reading GET /greetings: clock port "GreetingClock" names instant "MaybeInstant", whose one property "epochMs" is optional and of type integer, a clock counts in whole units so that property is a required integer`,
		},
		{
			name:    "methods a clock never declares",
			replace: "        - kind: clock\n          name: GreetingClock\n          instant: Instant\n          span: Span\n          adapters: [memory]\n          methods:\n            - name: now\n              reply: Instant",
			want:    "declares methods, a clock answers now and elapsed",
		},
		{
			name:    "the name of a store port",
			replace: "        - kind: clock\n          name: GreetingStore\n          instant: Instant\n          span: Span\n          adapters: [memory]",
			want:    "takes the name of a store or subscribe port",
		},
		{
			name:    "a value that belongs to a string port",
			replace: "        - kind: clock\n          name: GreetingClock\n          instant: Instant\n          span: Span\n          value: Label\n          adapters: [memory]",
			want:    `reading GET /greetings: clock port "GreetingClock" declares value, which belongs to the string kind, a clock port declares only instant, span`,
		},
		{
			name:    "a subject that belongs to a verifier port",
			replace: "        - kind: clock\n          name: GreetingClock\n          instant: Instant\n          span: Span\n          subject: Label\n          adapters: [memory]",
			want:    `reading GET /greetings: clock port "GreetingClock" declares subject, which belongs to the verifier kind, a clock port declares only instant, span`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			broken := strings.Replace(withOptionalSchemas(t, queriedSpec), clockDeclaration, tc.replace, 1)

			_, err := restrust.Generate([]byte(broken), restrust.Options{Service: "songe-hello"})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want an error carrying %q, got %v", tc.want, err)
			}
		})
	}
}

func clockRefusal(t *testing.T, declaration string) string {
	t.Helper()

	broken := strings.Replace(queriedSpec, clockDeclaration, declaration, 1)
	if broken == queriedSpec {
		t.Fatal("the clock declaration was never found, the fixture moved")
	}

	_, err := restrust.Generate([]byte(broken), restrust.Options{Service: "songe-hello"})
	if err == nil {
		t.Fatal("the broken clock declaration was accepted")
	}

	return err.Error()
}

func TestAClockNamingNoInstantIsRefusedSayingAClockDeclarationNamesTheTwoSchemasItSpeaks(t *testing.T) {
	got := clockRefusal(t, "        - kind: clock\n          name: GreetingClock\n          span: Span\n          adapters: [memory]")

	want := `reading GET /greetings: clock port "GreetingClock" names no instant, a clock declaration names the two schemas it speaks`
	if got != want {
		t.Fatalf("want the refusal\n%s\ngot\n%s", want, got)
	}
}

func TestAClockInstantCarryingThreePropertiesIsRefusedSayingAClockReadsAndWritesOneNumber(t *testing.T) {
	got := clockRefusal(t, "        - kind: clock\n          name: GreetingClock\n          instant: Greeting\n          span: Span\n          adapters: [memory]")

	want := `reading GET /greetings: clock port "GreetingClock" names instant "Greeting", which declares 3 properties, a clock reads and writes one number so its instant carries one property and the engine takes that property's name from the schema`
	if got != want {
		t.Fatalf("want the refusal\n%s\ngot\n%s", want, got)
	}
}

const optionalSchemas = `    MaybeAddress:
      type: object
      properties:
        address:
          type: string
    MaybeInstant:
      type: object
      properties:
        epochMs:
          type: integer
`

func withOptionalSchemas(t *testing.T, spec string) string {
	t.Helper()

	withSchemas := strings.Replace(spec, "    Label:", optionalSchemas+"    Label:", 1)
	if withSchemas == spec {
		t.Fatal("the Label schema was never found, the fixture moved")
	}

	return withSchemas
}

func TestAnXPortsDeclarationOfAnyKindButTheDeclaredOnesIsRefusedByName(t *testing.T) {
	cases := []struct {
		name    string
		replace string
	}{
		{
			name:    "an unknown kind",
			replace: "        - kind: store\n          name: GreetingClock\n          instant: Instant\n          span: Span\n          adapters: [memory]",
		},
		{
			name:    "the hand kind no declaration carries any more",
			replace: "        - kind: hand\n          name: GreetingClock\n          methods:\n            - name: now\n              reply: Instant",
		},
		{
			name:    "no kind at all",
			replace: "        - name: GreetingClock\n          instant: Instant\n          span: Span\n          adapters: [memory]",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			broken := strings.Replace(queriedSpec, clockDeclaration, tc.replace, 1)

			_, err := restrust.Generate([]byte(broken), restrust.Options{Service: "songe-hello"})
			if err == nil || !strings.Contains(err.Error(), "the declared kinds are clock") {
				t.Fatalf("want the declared kinds named, got %v", err)
			}
		})
	}
}

func TestTheSamePortDeclaredTwiceWithADifferentDeclarationIsRefused(t *testing.T) {
	twice := strings.Replace(queriedSpec, "      x-ports: [GreetingStore, GreetingClock]", "      x-ports:\n        - GreetingStore\n        - kind: clock\n          name: GreetingClock\n          instant: Span\n          span: Instant\n          adapters: [memory]", 1)

	_, err := restrust.Generate([]byte(twice), restrust.Options{Service: "songe-hello"})
	if err == nil || !strings.Contains(err.Error(), "a second time with a different declaration") {
		t.Fatalf("two disagreeing declarations were not refused: %v", err)
	}
}

func TestAnXPortsEntryThatIsNeitherANameNorADeclarationIsRefused(t *testing.T) {
	broken := strings.Replace(queriedSpec, "      x-ports: [GreetingStore, GreetingClock]", "      x-ports: [7]", 1)

	_, err := restrust.Generate([]byte(broken), restrust.Options{Service: "songe-hello"})
	if err == nil || !strings.Contains(err.Error(), "an entry is either a port name or an object naming kind, name and what that kind needs") {
		t.Fatalf("a number in x-ports was not refused: %v", err)
	}
}

func TestAQueryParameterNameRustCannotSpellIsRefused(t *testing.T) {
	broken := strings.Replace(queriedSpec, "        - name: after\n          in: query", "        - name: 1st\n          in: query", 1)

	_, err := restrust.Generate([]byte(broken), restrust.Options{Service: "songe-hello"})
	if err == nil || !strings.Contains(err.Error(), `reading query parameter "1st"`) {
		t.Fatalf("an unspellable query parameter name was not refused: %v", err)
	}
}

func TestALookupColumnIsAnEnumSoNeitherStoreAnswersADefaultForAColumnItCannotRead(t *testing.T) {
	byPath := queriedFiles(t)

	wantIn(t, byPath, "port/zz_generated_greeting_store.rs",
		"pub enum GreetingColumn {",
		"    Name,",
		`            Self::Name => "name",`,
	)

	wantIn(t, byPath, "adapter/zz_generated_greeting_memory.rs",
		"fn field_of(row: &Greeting, column: GreetingColumn) -> String {",
		"        GreetingColumn::Name => row.name.clone(),",
		"self.rows_where(&[(GreetingColumn::Name, name)], after.as_deref(), limit)",
	)

	wantIn(t, byPath, "adapter/zz_generated_greeting_sqlite.rs",
		"column.property()",
		"self.rows_where(&[(GreetingColumn::Name, name)], after.as_deref(), limit)",
	)

	for _, path := range []string{"adapter/zz_generated_greeting_memory.rs", "adapter/zz_generated_greeting_sqlite.rs"} {
		if strings.Contains(byPath[path], "_ => String::new()") {
			t.Errorf("%s still answers a default for a column it cannot read", path)
		}
	}
}

func TestARestControllerConstructorTakesOneGeneratedPortsStructHoldingEveryPortByName(t *testing.T) {
	files := queriedFiles(t)

	wantIn(t, files, "controller/zz_generated_greeting_controller.rs",
		"pub struct GreetingControllerPorts {\n    pub greeting_clock: Arc<dyn GreetingClock + Send + Sync>,\n    pub greeting_store: Arc<dyn GreetingStore + Send + Sync>,\n}",
		"impl GreetingControllerImpl {\n    pub fn new(ports: GreetingControllerPorts) -> Self {\n        let GreetingControllerPorts { greeting_clock, greeting_store } = ports;\n\n        Self { greeting_clock, greeting_store }\n    }\n}",
	)

	wantIn(t, files, "controller/mod.rs", "GreetingControllerImpl, GreetingControllerPorts}")

	if strings.Contains(files["controller/zz_generated_greeting_controller.rs"], "too_many_arguments") {
		t.Errorf("the controller still carries a too many arguments allow:\n%s", files["controller/zz_generated_greeting_controller.rs"])
	}
}

func TestABareNameNoOperationDeclaresIsAForeignPortTheCellRequiresAndWritesNoneOf(t *testing.T) {
	foreign := strings.Replace(queriedSpec, "      x-ports: [GreetingStore, GreetingClock]", "      x-ports: [GreetingStore, AuthzClient]", 1)

	byPath, err := generatedByPath(t, foreign)
	if err != nil {
		t.Fatalf("a bare port name was refused: %v", err)
	}

	wantIn(t, byPath, "controller/zz_generated_greeting_controller.rs",
		"use crate::port::authz_client::{AuthzClient, AuthzClientError};",
		"pub(crate) authz_client: Arc<dyn AuthzClient + Send + Sync>,",
		"pub struct GreetingControllerPorts {\n    pub authz_client: Arc<dyn AuthzClient + Send + Sync>,\n    pub greeting_clock: Arc<dyn GreetingClock + Send + Sync>,\n    pub greeting_store: Arc<dyn GreetingStore + Send + Sync>,\n}",
		"pub fn new(ports: GreetingControllerPorts) -> Self {\n        let GreetingControllerPorts { authz_client, greeting_clock, greeting_store } = ports;",
	)

	if _, written := byPath["port/zz_generated_authz_client.rs"]; written {
		t.Error("the cell wrote a port file for a port another cell provides")
	}

	m, err := cellmanifest.Parse([]byte(byPath[cellmanifest.FileName]))
	if err != nil {
		t.Fatalf("reading the manifest: %v", err)
	}

	if !holds(m.Requires.Ports, "AuthzClient") {
		t.Errorf("requires.ports = %q, want AuthzClient in it", m.Requires.Ports)
	}

	for _, p := range m.Provides.Ports {
		if p.Trait == "AuthzClient" {
			t.Errorf("the manifest provides the foreign port %+v", p)
		}
	}

	if !holds(m.Provides.Controllers[0].Ports, "AuthzClient") {
		t.Errorf("controller ports = %q, want AuthzClient in it", m.Provides.Controllers[0].Ports)
	}
}

func holds(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}

	return false
}

func TestTheControllerErrorEnumAndTheDriverCoverTheWholeTaxonomy(t *testing.T) {
	byPath := queriedFiles(t)

	wantIn(t, byPath, "controller/zz_generated_greeting_controller.rs",
		"Authentication { subject: String, reason: String },",
		"Authorization { subject: String, reason: String },",
		"NotFound { id: String },",
		"Invalid { field: String, reason: String },",
		"Semantic { resource: String, reason: String },",
		"RateLimited { subject: String, reason: String },",
		"NotImplemented { operation: String },",
	)

	wantIn(t, byPath, "driver/zz_generated_http_driver.rs",
		`GreetingControllerError::Authentication { .. } => reject(StatusCode::UNAUTHORIZED, "authentication", error.to_string()),`,
		`GreetingControllerError::Authorization { .. } => reject(StatusCode::FORBIDDEN, "authorization", error.to_string()),`,
		`GreetingControllerError::NotFound { .. } => reject(StatusCode::NOT_FOUND, "semantic", error.to_string()),`,
		`GreetingControllerError::Invalid { .. } => reject(invalid, "validation", error.to_string()),`,
		`GreetingControllerError::Semantic { .. } => reject(StatusCode::CONFLICT, "semantic", error.to_string()),`,
		`GreetingControllerError::RateLimited { .. } => reject(StatusCode::TOO_MANY_REQUESTS, "rateLimiting", error.to_string()),`,
		`GreetingControllerError::NotImplemented { .. } => reject(StatusCode::NOT_IMPLEMENTED, "runtime", error.to_string()),`,
	)

	wantIn(t, byPath, "driver/zz_generated_http_driver.rs",
		`GreetingControllerError::GreetingStore { .. } => {`,
		`reject(StatusCode::INTERNAL_SERVER_ERROR, "runtime", "internal error".to_string())`,
	)
}

const queriedCrateLib = `pub mod port;
pub mod rest;
pub mod types;
`

const queriedGreetingControllerImpl = `use crate::rest::controller::{
    GreetingController, GreetingControllerError, GreetingControllerImpl,
};
use crate::rest::types::greeting::Greeting;
use crate::rest::types::greeting_page::GreetingPage;

impl GreetingController for GreetingControllerImpl {
    fn list_greetings(
        &self,
        name: &str,
        after: Option<String>,
        limit: Option<i64>,
    ) -> Result<GreetingPage, GreetingControllerError> {
        if limit.is_some_and(|asked| asked > 100) {
            return Err(GreetingControllerError::RateLimited {
                subject: name.to_string(),
                reason: "too many at once".to_string(),
            });
        }

        let who = self
            .google_verifier
            .verify(name)
            .map_err(|source| GreetingControllerError::GoogleVerifier {
                id: name.to_string(),
                source,
            })?;

        if who.email.is_empty() {
            return Err(GreetingControllerError::Authentication {
                subject: who.subject,
                reason: "the verified identity carries no email".to_string(),
            });
        }

        let at = self
            .greeting_clock
            .now()
            .map_err(|source| GreetingControllerError::GreetingClock {
                id: "now".to_string(),
                source,
            })?;

        let Some(cursor) = after else {
            return Ok(GreetingPage {
                greetings: Vec::new(),
                at,
            });
        };

        Err(GreetingControllerError::Semantic {
            resource: cursor,
            reason: "the cursor names no greeting".to_string(),
        })
    }

    fn get_greeting(&self, id: &str, at: i64) -> Result<Greeting, GreetingControllerError> {
        if at < 0 {
            return Err(GreetingControllerError::Authorization {
                subject: id.to_string(),
                reason: "the past is closed".to_string(),
            });
        }

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

const controllerImplLeavingTheVerifierUnread = `use crate::rest::controller::{
    GreetingController, GreetingControllerError, GreetingControllerImpl,
};
use crate::rest::types::greeting::Greeting;
use crate::rest::types::greeting_page::GreetingPage;

impl GreetingController for GreetingControllerImpl {
    fn list_greetings(
        &self,
        name: &str,
        _after: Option<String>,
        _limit: Option<i64>,
    ) -> Result<GreetingPage, GreetingControllerError> {
        let at = self
            .greeting_clock
            .now()
            .map_err(|source| GreetingControllerError::GreetingClock {
                id: name.to_string(),
                source,
            })?;

        Ok(GreetingPage {
            greetings: Vec::new(),
            at,
        })
    }

    fn get_greeting(&self, id: &str, _at: i64) -> Result<Greeting, GreetingControllerError> {
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

const controllerImplMissingAMethod = `use crate::rest::controller::{
    GreetingController, GreetingControllerError, GreetingControllerImpl,
};
use crate::rest::types::greeting_page::GreetingPage;

impl GreetingController for GreetingControllerImpl {
    fn list_greetings(
        &self,
        name: &str,
        _after: Option<String>,
        _limit: Option<i64>,
    ) -> Result<GreetingPage, GreetingControllerError> {
        let at = self
            .greeting_clock
            .now()
            .map_err(|source| GreetingControllerError::GreetingClock {
                id: name.to_string(),
                source,
            })?;

        Ok(GreetingPage {
            greetings: Vec::new(),
            at,
        })
    }
}
`

func clippyOverTheVerifierCell(t *testing.T, controllerImpl string) (string, error) {
	t.Helper()

	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Skip("cargo is not on PATH")
	}

	files, err := restrust.Generate([]byte(specWithAGoogleVerifier(t)), restrust.Options{Service: "songe-hello", Side: restrust.SideBoth})
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
	write("src/lib.rs", queriedCrateLib)
	WriteCrateRootPorts(t, write, "// Code generated by a test. DO NOT EDIT.")

	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".rs") {
			continue
		}

		write(filepath.Join("src", "rest", f.Path), f.Content)
	}

	write("src/rest/controller/greeting_controller.rs", controllerImpl)

	cmd := exec.Command(cargo, "clippy", "--workspace", "--all-targets", "--", "-D", "warnings")
	cmd.Dir = root

	out, err := cmd.CombinedOutput()

	lower := strings.ToLower(string(out))
	if err != nil && (strings.Contains(lower, "could not resolve host") ||
		strings.Contains(lower, "failed to get") ||
		strings.Contains(lower, "spurious network error")) {
		t.Skipf("cargo clippy needs network access to crates.io, which this run did not have: %v\n%s", err, out)
	}

	return string(out), err
}

func TestTheQueriedCellWithADeclaredPortCompilesOnceTheUserWritesTheControllerImpl(t *testing.T) {
	out, err := clippyOverTheVerifierCell(t, queriedGreetingControllerImpl)
	if err != nil {
		t.Fatalf("cargo clippy: %v\n%s", err, out)
	}
}
