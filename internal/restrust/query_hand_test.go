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
          - { by: name, answers: page }
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
			name:    "methods a clock never declares",
			replace: "        - kind: clock\n          name: GreetingClock\n          instant: Instant\n          span: Span\n          adapters: [memory]\n          methods:\n            - name: now\n              reply: Instant",
			want:    "declares methods, a clock answers now and elapsed",
		},
		{
			name:    "the name of a store port",
			replace: "        - kind: clock\n          name: GreetingStore\n          instant: Instant\n          span: Span\n          adapters: [memory]",
			want:    "takes the name of a store or subscribe port",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			broken := strings.Replace(queriedSpec, clockDeclaration, tc.replace, 1)

			_, err := restrust.Generate([]byte(broken), restrust.Options{Service: "songe-hello"})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want an error carrying %q, got %v", tc.want, err)
			}
		})
	}
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
		"self.rows_where(GreetingColumn::Name, name, after.as_deref(), limit)",
	)

	wantIn(t, byPath, "adapter/zz_generated_greeting_sqlite.rs",
		"column.property()",
		"self.rows_where(GreetingColumn::Name, name, after.as_deref(), limit)",
	)

	for _, path := range []string{"adapter/zz_generated_greeting_memory.rs", "adapter/zz_generated_greeting_sqlite.rs"} {
		if strings.Contains(byPath[path], "_ => String::new()") {
			t.Errorf("%s still answers a default for a column it cannot read", path)
		}
	}
}

func TestAPortNameThatNoOperationDeclaresIsRefused(t *testing.T) {
	unknown := strings.Replace(queriedSpec, "      x-ports: [GreetingStore, GreetingClock]", "      x-ports: [GreetingStore, GreetingCalendar]", 1)

	_, err := restrust.Generate([]byte(unknown), restrust.Options{Service: "songe-hello"})
	if err == nil || !strings.Contains(err.Error(), "no operation declares it with a kind") {
		t.Fatalf("an undeclared port name was not refused: %v", err)
	}
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

func TestTheQueriedCellWithADeclaredPortCompilesOnceTheUserWritesTheControllerImpl(t *testing.T) {
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Skip("cargo is not on PATH")
	}

	spec := strings.Replace(queriedSpec, clockDeclaration, bothClocksDeclaration, 1)

	files, err := restrust.Generate([]byte(spec), restrust.Options{Service: "songe-hello", Side: restrust.SideBoth})
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

	write("src/rest/controller/greeting_controller.rs", queriedGreetingControllerImpl)

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

	t.Fatalf("cargo clippy: %v\n%s", err, out)
}
