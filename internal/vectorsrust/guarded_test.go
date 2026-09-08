package vectorsrust_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/crateports"
	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/restrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/vectorsrust"
)

const guardedSpec = `
openapi: 3.1.0
info:
  title: Hello API
  version: 1.0.0
paths:
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

const guardedVectors = `{
  "cases": [
    {
      "case": "counting_with_a_valid_ticket_hands_the_subject_to_the_controller",
      "operation": "countGreeting",
      "input": { "id": "g1" },
      "bearer": "open",
      "subject": "friend",
      "controllerReply": { "id": "g1", "name": "Songe", "count": 1 },
      "expectedStatus": 200,
      "expectedBody": { "id": "g1", "name": "Songe", "count": 1 }
    },
    {
      "case": "counting_without_a_ticket_is_refused_before_the_controller",
      "operation": "countGreeting",
      "input": { "id": "g1" },
      "expectedStatus": 401,
      "expectedErrorSubstring": "bearer"
    },
    {
      "case": "counting_with_a_refused_ticket_answers_authentication",
      "operation": "countGreeting",
      "input": { "id": "g1" },
      "bearer": "wrong",
      "expectedStatus": 401,
      "expectedErrorSubstring": "refused"
    },
    {
      "case": "streaming_with_a_valid_ticket_answers_the_first_event",
      "operation": "streamGreetingEvents",
      "input": { "id": "g1" },
      "bearer": "open",
      "subject": "friend",
      "controllerReply": { "id": "g1", "count": 1 },
      "expectedStatus": 200,
      "expectedBody": { "id": "g1", "count": 1 }
    },
    {
      "case": "getting_a_known_id_returns_it",
      "operation": "getGreeting",
      "input": { "id": "g1" },
      "controllerReply": { "id": "g1", "name": "Songe", "count": 0 },
      "expectedStatus": 200,
      "expectedBody": { "id": "g1", "name": "Songe", "count": 0 }
    }
  ]
}`

const guardedCrateManifest = `[workspace]

[package]
name = "songe-hello"
version = "0.1.0"
edition = "2021"

[dependencies]
axum = "0.8"
http-body-util = { version = "0.1", features = ["channel"] }
rusqlite = { version = "0.40", features = ["bundled"] }
serde = { version = "1", features = ["derive"] }
serde_json = "1"
thiserror = "2"
tokio = { version = "1", features = ["full"] }

[dev-dependencies]
mockall = "0.15"
tower = "0.5"
`

const guardedControllerImpl = `use crate::rest::controller::{
    GreetingController, GreetingControllerError, GreetingControllerImpl,
};
use crate::rest::types::greeting::Greeting;
use crate::rest::types::greeting_event::GreetingEvent;
use crate::types::subject::Subject;

impl GreetingController for GreetingControllerImpl {
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

const testHeader = "// Code generated by a test. DO NOT EDIT."

func writeCrateRootPorts(write func(rel, content string)) {
	write("src/lib.rs", "pub mod port;\npub mod rest;\npub mod types;\n")
	write("src/types/mod.rs", testHeader+"\n\npub mod zz_generated_subject;\n\npub use zz_generated_subject as subject;\n")
	write("src/types/"+crateports.SubjectFile, crateports.SubjectSource(testHeader))
	write("src/port/mod.rs", testHeader+"\n\npub mod zz_generated_ticket_verifier;\n\npub use zz_generated_ticket_verifier as ticket_verifier;\n")
	write("src/port/"+crateports.TicketVerifierFile, crateports.TicketVerifierSource(testHeader))
}

func TestTheGeneratedVectorsCarryTheBearerAndReadTheStreamAgainstTheGuardedDriver(t *testing.T) {
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Skip("cargo is not on PATH")
	}

	restFiles, err := restrust.Generate([]byte(guardedSpec), restrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating the rest cell: %v", err)
	}

	vectorFiles, err := vectorsrust.Generate([]byte(guardedSpec), []byte(guardedVectors), vectorsrust.Options{Service: "songe-hello"})
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

	write("Cargo.toml", guardedCrateManifest)
	writeCrateRootPorts(write)

	for _, f := range restFiles {
		if strings.HasSuffix(f.Path, ".yaml") {
			continue
		}

		write(filepath.Join("src", "rest", f.Path), f.Content)
	}

	for _, f := range vectorFiles {
		write(f.Path, f.Content)
	}

	write("src/rest/controller/greeting_controller.rs", guardedControllerImpl)

	cmd := exec.Command(cargo, "test", "--workspace")
	cmd.Dir = root

	out, err := cmd.CombinedOutput()
	if err != nil {
		skipOnNetworkError(t, err, out)
		t.Fatalf("cargo test: %v\n%s", err, out)
	}

	for _, want := range []string{
		"counting_with_a_valid_ticket_hands_the_subject_to_the_controller ... ok",
		"counting_without_a_ticket_is_refused_before_the_controller ... ok",
		"counting_with_a_refused_ticket_answers_authentication ... ok",
		"streaming_with_a_valid_ticket_answers_the_first_event ... ok",
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("cargo test output lacks %q\n%s", want, out)
		}
	}
}

func TestABearerOnAnOperationWithoutXAuthIsRefused(t *testing.T) {
	vectors := `{"cases": [{"case": "a", "operation": "getGreeting", "input": {"id": "g1"}, "bearer": "open", "controllerReply": {"id": "g1", "name": "n", "count": 0}, "expectedStatus": 200}]}`

	_, err := vectorsrust.Generate([]byte(guardedSpec), []byte(vectors), vectorsrust.Options{Service: "songe-hello"})
	if err == nil || !strings.Contains(err.Error(), "carries no x-auth") {
		t.Fatalf("a bearer on a plain operation was not refused: %v", err)
	}
}

func TestASuccessCaseOnAnXAuthOperationNeedsTheSubjectTheVerifierAnswers(t *testing.T) {
	vectors := `{"cases": [{"case": "a", "operation": "countGreeting", "input": {"id": "g1"}, "bearer": "open", "controllerReply": {"id": "g1", "name": "n", "count": 0}, "expectedStatus": 200}]}`

	_, err := vectorsrust.Generate([]byte(guardedSpec), []byte(vectors), vectorsrust.Options{Service: "songe-hello"})
	if err == nil || !strings.Contains(err.Error(), "needs a bearer and the subject") {
		t.Fatalf("a success case without a subject was not refused: %v", err)
	}
}

func TestASubjectWithoutABearerIsRefused(t *testing.T) {
	vectors := `{"cases": [{"case": "a", "operation": "countGreeting", "input": {"id": "g1"}, "subject": "friend", "expectedStatus": 401, "expectedErrorSubstring": "bearer"}]}`

	_, err := vectorsrust.Generate([]byte(guardedSpec), []byte(vectors), vectorsrust.Options{Service: "songe-hello"})
	if err == nil || !strings.Contains(err.Error(), "there is no bearer") {
		t.Fatalf("a subject without a bearer was not refused: %v", err)
	}
}

func TestA401OnAnOperationWithoutXAuthIsStillUnmockable(t *testing.T) {
	vectors := `{"cases": [{"case": "a", "operation": "getGreeting", "input": {"id": "g1"}, "expectedStatus": 401, "expectedErrorSubstring": "bearer"}]}`

	_, err := vectorsrust.Generate([]byte(guardedSpec), []byte(vectors), vectorsrust.Options{Service: "songe-hello"})
	if err == nil || !strings.Contains(err.Error(), "expectedStatus 401 matches none") {
		t.Fatalf("a 401 on a plain operation was not refused: %v", err)
	}
}

func TestTheEmittedTestArmsTheVerifierAndSendsTheBearerHeader(t *testing.T) {
	files, err := vectorsrust.Generate([]byte(guardedSpec), []byte(guardedVectors), vectorsrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	content := files[0].Content

	for _, want := range []string{
		"use songe_hello::port::ticket_verifier::TicketVerifierError;",
		"use songe_hello::types::subject::Subject;",
		"mockall::mock! {\n    pub TicketVerifier {}",
		"impl songe_hello::port::ticket_verifier::TicketVerifier for TicketVerifier {",
		`.with(mockall::predicate::eq("open"))`,
		`.returning(|_token| Ok(Subject { id: "friend".to_string() }));`,
		`.returning(|_token| Err(TicketVerifierError::Refused { reason: "refused".to_string() }));`,
		"ticket_verifier.expect_verify().never();",
		"greeting_controller.expect_count_greeting().never();",
		`.header("authorization", format!("Bearer {}", "open"))`,
		`mockall::predicate::eq(Subject { id: "friend".to_string() }), mockall::predicate::eq("g1")`,
		"std::sync::Arc::new(ticket_verifier));",
		"let got: serde_json::Value = first_event(&body_bytes);",
		`Some("text/event-stream")`,
		`#[tokio::test(flavor = "multi_thread")]`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("the emitted test lacks %q\n%s", want, content)
		}
	}

	if strings.Contains(content, "rest::types::subject") {
		t.Errorf("the emitted test imports Subject from the cell instead of the crate root\n%s", content)
	}
}
