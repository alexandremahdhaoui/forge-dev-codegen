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

package grpcrust_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/grpcrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/restrust"
)

const cargoCheckCrateManifest = `[package]
name = "songe-hello"
version = "0.1.0"
edition = "2021"

[dependencies]
serde = { version = "1", features = ["derive"] }
thiserror = "2"
tonic = "0.14"
tonic-prost = "0.14"
prost = "0.14"
tokio = { version = "1", features = ["rt-multi-thread", "macros", "net"] }

[dev-dependencies]
mockall = "0.15"

[build-dependencies]
protox = "0.9"
tonic-prost-build = "0.14"
`

const cargoCheckCellLib = `pub mod grpc;
`

const cargoCheckBuildScript = `include!("src/grpc/zz_generated_build.rs");
`

const helloControllerImpl = `use crate::grpc::controller::{HelloController, HelloControllerError, HelloControllerImpl};
use crate::grpc::types::hello_messages::{PingReply, PingRequest};

impl HelloController for HelloControllerImpl {
    fn ping(&self, request: PingRequest) -> Result<PingReply, HelloControllerError> {
        Ok(PingReply {
            message: request.message,
            count: request.count + 1,
        })
    }
}
`

func TestTheGeneratedCellPassesCargoCheck(t *testing.T) {
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Skip("cargo is not on PATH")
	}

	files, err := grpcrust.Generate([]byte(helloProto), grpcrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating the cell: %v", err)
	}

	root := t.TempDir()
	write := writerUnder(t, root)

	write("Cargo.toml", cargoCheckCrateManifest)
	write("src/lib.rs", cargoCheckCellLib)
	write("build.rs", cargoCheckBuildScript)

	for _, f := range files {
		if strings.HasSuffix(f.Path, ".yaml") {
			continue
		}

		write(filepath.Join("src", "grpc", f.Path), f.Content)
	}

	write("src/grpc/controller/hello_controller.rs", helloControllerImpl)

	runCargoCheck(t, cargo, root)
}

func writerUnder(t *testing.T, root string) func(rel, content string) {
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

func runCargoCheck(t *testing.T, cargo, root string) {
	t.Helper()

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

const bothEnginesOpenapi = `
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

const bothEnginesCrateManifest = `[package]
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
tonic = "0.14"
tonic-prost = "0.14"

[dev-dependencies]
mockall = "0.15"

[build-dependencies]
protox = "0.9"
tonic-prost-build = "0.14"
`

const bothEnginesCrateLib = `pub mod grpc;
pub mod rest;
`

const bothEnginesGreetingControllerImpl = `use crate::rest::controller::{
    GreetingController, GreetingControllerError, GreetingControllerImpl,
};
use crate::rest::port::greeting_store::GreetingStore;
use crate::rest::types::greeting::Greeting;

impl GreetingController for GreetingControllerImpl {
    fn create_greeting(&self, body: Greeting) -> Result<Greeting, GreetingControllerError> {
        self.greeting_store
            .put(body.clone())
            .map_err(|source| GreetingControllerError::GreetingStore {
                id: body.id.clone(),
                source,
            })?;

        Ok(body)
    }
}
`

func TestBothEnginesFillOneCrateAndTheCratePassesCargoCheck(t *testing.T) {
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Skip("cargo is not on PATH")
	}

	restFiles, err := restrust.Generate([]byte(bothEnginesOpenapi), restrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating the rest cell: %v", err)
	}

	cellFiles, err := grpcrust.Generate([]byte(helloProto), grpcrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating the cell: %v", err)
	}

	root := t.TempDir()
	written := map[string]bool{}
	writeFile := writerUnder(t, root)

	write := func(rel, content string) {
		t.Helper()

		if written[rel] {
			t.Fatalf("%s is claimed by both engines", rel)
		}

		written[rel] = true

		writeFile(rel, content)
	}

	write("Cargo.toml", bothEnginesCrateManifest)
	write("build.rs", cargoCheckBuildScript)
	write("src/lib.rs", bothEnginesCrateLib)

	for _, f := range restFiles {
		if strings.HasSuffix(f.Path, ".yaml") {
			continue
		}

		write(filepath.Join("src", "rest", f.Path), f.Content)
	}

	for _, f := range cellFiles {
		if strings.HasSuffix(f.Path, ".yaml") {
			continue
		}

		write(filepath.Join("src", "grpc", f.Path), f.Content)
	}

	write("src/rest/controller/greeting_controller.rs", bothEnginesGreetingControllerImpl)
	write("src/grpc/controller/hello_controller.rs", helloControllerImpl)

	runCargoCheck(t, cargo, root)
}
