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

package vectorsrust_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/hexrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/udprust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/vectorsrust"
)

const cargoUdpProto = `syntax = "proto3";

package songe.hello.udp.v1;

service HelloDatagram {
  rpc Echo(Echo) returns (Echo);
}

message Echo {
  string payload = 1;
  uint64 count = 2;
}

message Nothing {}
`

const cargoCrateManifest = `[package]
name = "songe-hello"
version = "0.1.0"
edition = "2021"

[dependencies]
anyhow = "1"
axum = "0.8"
prost = "0.14"
rusqlite = { version = "0.40", features = ["bundled"] }
serde = { version = "1", features = ["derive"] }
serde_json = "1"
thiserror = "2"
tokio = { version = "1", features = ["full"] }

[dev-dependencies]
mockall = "0.15"
tower = "0.5"
http-body-util = "0.1"
`

const cargoGreetingControllerImpl = `use crate::controller::{GreetingController, GreetingControllerError, GreetingControllerImpl};
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

const cargoDatagramControllerImpl = `use crate::udp::controller::{
    HelloDatagramController, HelloDatagramControllerError, HelloDatagramControllerImpl,
};
use crate::udp::types::context::Context;
use crate::udp::types::hello_datagram_messages::Echo;

impl HelloDatagramController for HelloDatagramControllerImpl {
    fn echo(&self, request: Echo, context: &Context) -> Result<Echo, HelloDatagramControllerError> {
        let _ = context;

        Ok(Echo {
            payload: request.payload,
            count: request.count + 1,
        })
    }
}
`

const cargoSpec = `
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

const cargoVectors = `{
  "cases": [
    {
      "case": "creating_a_greeting_succeeds",
      "operation": "createGreeting",
      "input": { "name": "Songe" },
      "controllerReply": { "id": "g1", "name": "Songe", "count": 0 },
      "expectedStatus": 201,
      "expectedBody": { "id": "g1", "name": "Songe", "count": 0 }
    },
    {
      "case": "creating_a_greeting_with_an_empty_name_is_refused",
      "operation": "createGreeting",
      "input": { "name": "" },
      "expectedStatus": 400,
      "expectedErrorSubstring": "name"
    },
    {
      "case": "getting_a_known_id_returns_it",
      "operation": "getGreeting",
      "input": { "id": "g1" },
      "controllerReply": { "id": "g1", "name": "Songe", "count": 0 },
      "expectedStatus": 200,
      "expectedBody": { "id": "g1", "name": "Songe", "count": 0 }
    },
    {
      "case": "getting_an_unknown_id_answers_not_found",
      "operation": "getGreeting",
      "input": { "id": "missing" },
      "expectedStatus": 404,
      "expectedErrorSubstring": "not found"
    },
    {
      "case": "a_datagram_echo_comes_back_with_the_count_raised_by_one",
      "operation": "udp_echo",
      "input": { "sessionId": "0123456789abcdef", "payload": "songe", "count": 7 },
      "controllerReply": { "payload": "songe", "count": 8 },
      "expectedBody": { "sessionId": "0123456789abcdef", "payload": "songe", "count": 8 }
    }
  ]
}`

func buildCargoWorkspace(t *testing.T, mangleWire func(string) string) (root string, cargo string) {
	t.Helper()

	var err error

	cargo, err = exec.LookPath("cargo")
	if err != nil {
		t.Skip("cargo is not on PATH")
	}

	hexFiles, err := hexrust.Generate([]byte(cargoSpec), hexrust.Options{Service: "songe-hello", Cells: []string{"udp"}})
	if err != nil {
		t.Fatalf("generating the hexagonal skeleton: %v", err)
	}

	udpFiles, err := udprust.Generate([]byte(cargoUdpProto), udprust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating the udp cell: %v", err)
	}

	vectorFiles, err := vectorsrust.Generate([]byte(cargoSpec), []byte(cargoVectors), vectorsrust.Options{
		Service: "songe-hello",
		Proto:   []byte(cargoUdpProto),
	})
	if err != nil {
		t.Fatalf("generating the vectors: %v", err)
	}

	root = t.TempDir()

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

	write("Cargo.toml", cargoCrateManifest)

	for _, f := range hexFiles {
		if strings.HasSuffix(f.Path, ".yaml") {
			continue
		}

		content := f.Content
		if f.Path == "src/driver/zz_generated_wire.rs" && mangleWire != nil {
			content = mangleWire(content)
		}

		write(f.Path, content)
	}

	for _, f := range udpFiles {
		if strings.HasSuffix(f.Path, ".yaml") {
			continue
		}

		write(filepath.Join("src", "udp", f.Path), f.Content)
	}

	for _, f := range vectorFiles {
		write(f.Path, f.Content)
	}

	write("src/controller/greeting_controller.rs", cargoGreetingControllerImpl)
	write("src/udp/controller/hello_datagram_controller.rs", cargoDatagramControllerImpl)

	return root, cargo
}

func runCargoTest(t *testing.T, root, cargo string) ([]byte, error) {
	t.Helper()

	cmd := exec.Command(cargo, "test", "--workspace")
	cmd.Dir = root

	return cmd.CombinedOutput()
}

func skipOnNetworkError(t *testing.T, err error, out []byte) {
	t.Helper()

	lower := strings.ToLower(string(out))
	if strings.Contains(lower, "could not resolve host") ||
		strings.Contains(lower, "failed to get") ||
		strings.Contains(lower, "network") ||
		strings.Contains(lower, "spurious network error") ||
		strings.Contains(lower, "403 forbidden") {
		t.Skipf("cargo test needs network access to crates.io, which this run did not have: %v\n%s", err, out)
	}
}

func TestTheGeneratedVectorsPassAgainstTheGeneratedDriverAndAMockedController(t *testing.T) {
	root, cargo := buildCargoWorkspace(t, nil)

	out, err := runCargoTest(t, root, cargo)
	if err != nil {
		skipOnNetworkError(t, err, out)
		t.Fatalf("cargo test: %v\n%s", err, out)
	}
}

func TestAMangledRequestBodyMappingMakesTheWithPredicateFailTheGeneratedTest(t *testing.T) {
	mangle := func(content string) string {
		const original = `impl From<CreateGreetingRequestWire> for CreateGreetingRequest {
    fn from(w: CreateGreetingRequestWire) -> Self {
        Self {
            name: w.name,
        }
    }
}`

		const mangled = `impl From<CreateGreetingRequestWire> for CreateGreetingRequest {
    fn from(w: CreateGreetingRequestWire) -> Self {
        let _ = w.name;
        Self {
            name: "mangled".to_string(),
        }
    }
}`

		if !strings.Contains(content, original) {
			t.Fatalf("the wire content changed shape, update the mangle fixture:\n%s", content)
		}

		return strings.Replace(content, original, mangled, 1)
	}

	root, cargo := buildCargoWorkspace(t, mangle)

	out, err := runCargoTest(t, root, cargo)
	if err == nil {
		t.Fatalf("cargo test succeeded although the driver mangles the request body before it reaches the controller:\n%s", out)
	}

	skipOnNetworkError(t, err, out)

	if !strings.Contains(string(out), "creating_a_greeting_succeeds") {
		t.Fatalf("cargo test failed but not on the test asserting the body, got:\n%s", out)
	}
}
