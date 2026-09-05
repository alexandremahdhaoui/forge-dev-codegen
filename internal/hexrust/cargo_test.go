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
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const nodeCrateManifest = `[workspace]

[package]
name = "songe-hello"
version = "0.1.0"
edition = "2021"
build = "zz_generated_build.rs"

[[bin]]
name = "songe-hello-node"
path = "src/bin/zz_generated_songe_hello_node.rs"

[dependencies]
anyhow = "1"
axum = "0.8"
prost = "0.14"
rusqlite = { version = "0.40", features = ["bundled"] }
serde = { version = "1", features = ["derive"] }
serde_json = "1"
songe-common = { path = "SONGE_COMMON_DIR" }
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

const nodeGreetingControllerImpl = `use crate::rest::controller::{
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

const nodeHelloControllerImpl = `use crate::grpc::controller::{HelloController, HelloControllerError, HelloControllerImpl};
use crate::grpc::types::hello_messages::{PingReply, PingRequest};

impl HelloController for HelloControllerImpl {
    fn ping(&self, request: PingRequest) -> Result<PingReply, HelloControllerError> {
        Ok(PingReply {
            message: request.message,
        })
    }
}
`

const nodeDatagramControllerImpl = `use crate::udp::controller::{
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

const nodeMemoryAdapter = `use std::collections::HashMap;
use std::sync::Mutex;

use crate::adapter::GreetingMemoryStoreConfig;
use crate::rest::port::greeting_store::{GreetingStore, GreetingStoreError};
use crate::rest::types::greeting::Greeting;

pub struct GreetingMemoryStore {
    capacity: usize,
    rows: Mutex<HashMap<String, Greeting>>,
}

impl GreetingMemoryStore {
    pub fn new(config: GreetingMemoryStoreConfig) -> Self {
        Self {
            capacity: config.capacity.unsigned_abs() as usize,
            rows: Mutex::new(HashMap::new()),
        }
    }
}

impl GreetingStore for GreetingMemoryStore {
    fn put(&self, v: Greeting) -> Result<(), GreetingStoreError> {
        let mut rows = self.rows.lock().expect("the memory store lock");

        if rows.len() >= self.capacity && !rows.contains_key(&v.id) {
            rows.clear();
        }

        rows.insert(v.id.clone(), v);

        Ok(())
    }

    fn get(&self, id: &str) -> Result<Option<Greeting>, GreetingStoreError> {
        let rows = self.rows.lock().expect("the memory store lock");

        Ok(rows.get(id).cloned())
    }
}
`

const nodeConfigLoader = `#[derive(Debug)]
pub struct ConfigError {
    message: String,
}

impl std::fmt::Display for ConfigError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", self.message)
    }
}

impl std::error::Error for ConfigError {}

pub struct SongeHelloConfig {
    pub greeting_store: String,
    pub greeting_store_memory_capacity: i64,
    pub greeting_store_sqlite_path: String,
    pub driver_grpc: bool,
    pub driver_rest: bool,
    pub driver_udp: bool,
    pub grpc_addr: String,
    pub rest_addr: String,
    pub udp_addr: String,
}

impl Default for SongeHelloConfig {
    fn default() -> Self {
        Self {
            greeting_store: "sqlite".to_string(),
            greeting_store_memory_capacity: 100,
            greeting_store_sqlite_path: ":memory:".to_string(),
            driver_grpc: true,
            driver_rest: true,
            driver_udp: true,
            grpc_addr: "127.0.0.1:0".to_string(),
            rest_addr: "127.0.0.1:0".to_string(),
            udp_addr: "127.0.0.1:0".to_string(),
        }
    }
}

impl SongeHelloConfig {
    pub fn load(args: &[String]) -> Result<Self, ConfigError> {
        let mut config = Self::default();

        for arg in args {
            let Some((key, value)) = arg.trim_start_matches("--").split_once('=') else {
                return Err(ConfigError {
                    message: format!("reading {arg:?}: an argument is --key=value"),
                });
            };

            match key {
                "greeting_store" => config.greeting_store = value.to_string(),
                "greeting_store_memory_capacity" => {
                    config.greeting_store_memory_capacity = parse_integer(key, value)?
                }
                "greeting_store_sqlite_path" => {
                    config.greeting_store_sqlite_path = value.to_string()
                }
                "driver_grpc" => config.driver_grpc = parse_boolean(key, value)?,
                "driver_rest" => config.driver_rest = parse_boolean(key, value)?,
                "driver_udp" => config.driver_udp = parse_boolean(key, value)?,
                "grpc_addr" => config.grpc_addr = value.to_string(),
                "rest_addr" => config.rest_addr = value.to_string(),
                "udp_addr" => config.udp_addr = value.to_string(),
                other => {
                    return Err(ConfigError {
                        message: format!("reading {other:?}: it names no key"),
                    })
                }
            }
        }

        Ok(config)
    }
}

fn parse_integer(key: &str, value: &str) -> Result<i64, ConfigError> {
    value.parse().map_err(|_| ConfigError {
        message: format!("reading {key:?}: {value:?} is not an integer"),
    })
}

fn parse_boolean(key: &str, value: &str) -> Result<bool, ConfigError> {
    value.parse().map_err(|_| ConfigError {
        message: format!("reading {key:?}: {value:?} is not a boolean"),
    })
}
`

var (
	sharedTargetOnce sync.Once
	sharedTargetDir  string
)

func TestMain(m *testing.M) {
	code := m.Run()

	if sharedTargetDir != "" {
		_ = os.RemoveAll(sharedTargetDir)
	}

	os.Exit(code)
}

func targetDir(t *testing.T) string {
	t.Helper()

	sharedTargetOnce.Do(func() {
		dir, err := os.MkdirTemp("", "hexrust-cargo-target")
		if err != nil {
			t.Fatalf("making the shared cargo target directory: %v", err)
		}

		sharedTargetDir = dir
	})

	return sharedTargetDir
}

func songeCommonDir(t *testing.T) string {
	t.Helper()

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("locating the repo root: %v", err)
	}

	dir := filepath.Join(repoRoot, "..", "songe-common")

	crateManifest := filepath.Join(dir, "Cargo.toml")
	if _, err := os.Stat(crateManifest); err != nil {
		t.Fatalf("songe-common is not checked out beside the repo, %s is missing: %v", crateManifest, err)
	}

	workspaceManifest := filepath.Join(repoRoot, "..", "Cargo.toml")
	if _, err := os.Stat(workspaceManifest); err != nil {
		t.Fatalf("songe-common inherits its dependencies from the workspace, %s is missing: %v", workspaceManifest, err)
	}

	return dir
}

func standUpTheNodeCrate(t *testing.T) (string, string) {
	t.Helper()

	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Skip("cargo is not on PATH")
	}

	root := standUpCells(t, "grpc", "rest", "udp")
	write := writeUnder(t, root)

	for _, cell := range []string{"grpc", "rest", "udp"} {
		for _, f := range cellFiles(t, cell) {
			if !strings.HasSuffix(f.Path, ".rs") && !strings.HasSuffix(f.Path, ".proto") {
				continue
			}

			write(filepath.Join("src", cell, f.Path), f.Content)
		}
	}

	for path, content := range generateHello(t, root, helloWiring) {
		if !strings.HasSuffix(path, ".rs") {
			continue
		}

		write(path, content)
	}

	write("Cargo.toml", strings.ReplaceAll(nodeCrateManifest, "SONGE_COMMON_DIR", songeCommonDir(t)))
	write("src/config/zz_generated_config.rs", nodeConfigLoader)
	write("src/adapter/greeting_memory.rs", nodeMemoryAdapter)
	write("src/rest/controller/greeting_controller.rs", nodeGreetingControllerImpl)
	write("src/grpc/controller/hello_controller.rs", nodeHelloControllerImpl)
	write("src/udp/controller/hello_datagram_controller.rs", nodeDatagramControllerImpl)

	return cargo, root
}

func runCargo(t *testing.T, cargo, root string, args ...string) (string, error) {
	t.Helper()

	cmd := exec.Command(cargo, args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CARGO_TARGET_DIR="+targetDir(t))

	out, err := cmd.CombinedOutput()

	return string(out), err
}

func skipOnNetwork(t *testing.T, out string, err error) {
	t.Helper()

	lower := strings.ToLower(out)
	if strings.Contains(lower, "could not resolve host") ||
		strings.Contains(lower, "failed to get") ||
		strings.Contains(lower, "spurious network error") {
		t.Skipf("cargo needs network access to crates.io, which this run did not have: %v\n%s", err, out)
	}
}

func TestTheGeneratedNodeCrateCompilesOnceTheUserWritesItsFourFiles(t *testing.T) {
	cargo, root := standUpTheNodeCrate(t)

	out, err := runCargo(t, cargo, root, "build", "--workspace", "--all-targets")
	if err != nil {
		skipOnNetwork(t, out, err)
		t.Fatalf("cargo build: %v\n%s", err, out)
	}
}

func TestDeletingTheControllerImplFailsTheBuildWithE0583NamingTheFile(t *testing.T) {
	cargo, root := standUpTheNodeCrate(t)

	if err := os.Remove(filepath.Join(root, "src", "rest", "controller", "greeting_controller.rs")); err != nil {
		t.Fatalf("removing the controller impl: %v", err)
	}

	out, err := runCargo(t, cargo, root, "check", "--workspace")
	if err == nil {
		t.Fatalf("the build passed without the controller impl\n%s", out)
	}

	skipOnNetwork(t, out, err)

	for _, want := range []string{"E0583", "greeting_controller"} {
		if !strings.Contains(out, want) {
			t.Fatalf("the build never named %q\n%s", want, out)
		}
	}
}

func TestRemovingOneControllerMethodFailsTheBuildWithE0046(t *testing.T) {
	cargo, root := standUpTheNodeCrate(t)

	stripped := `use crate::udp::controller::{
    HelloDatagramController, HelloDatagramControllerError, HelloDatagramControllerImpl,
};
use crate::udp::types::context::Context;
use crate::udp::types::hello_datagram_messages::Echo;

impl HelloDatagramController for HelloDatagramControllerImpl {}

fn unused(request: Echo, context: &Context) -> Result<Echo, HelloDatagramControllerError> {
    let _ = context;

    Ok(request)
}
`

	writeUnder(t, root)("src/udp/controller/hello_datagram_controller.rs", stripped)

	out, err := runCargo(t, cargo, root, "check", "--workspace")
	if err == nil {
		t.Fatalf("the build passed with a method missing\n%s", out)
	}

	skipOnNetwork(t, out, err)

	for _, want := range []string{"E0046", "echo"} {
		if !strings.Contains(out, want) {
			t.Fatalf("the build never named %q\n%s", want, out)
		}
	}
}

func TestAUserFileWithNoImplBlockFailsTheBuildNamingTheTrait(t *testing.T) {
	cargo, root := standUpTheNodeCrate(t)

	empty := `use crate::rest::controller::{
    GreetingController, GreetingControllerError, GreetingControllerImpl,
};
use crate::rest::port::greeting_store::GreetingStore;
use crate::rest::types::greeting::Greeting;

fn unused(
    controller: &GreetingControllerImpl,
    store: &dyn GreetingStore,
    body: Greeting,
) -> Result<Greeting, GreetingControllerError> {
    let _ = (controller, store);

    Ok(body)
}
`

	writeUnder(t, root)("src/rest/controller/greeting_controller.rs", empty)

	out, err := runCargo(t, cargo, root, "check", "--workspace")
	if err == nil {
		t.Fatalf("the build passed with no impl block at all\n%s", out)
	}

	skipOnNetwork(t, out, err)

	for _, want := range []string{"E0277", "GreetingController"} {
		if !strings.Contains(out, want) {
			t.Fatalf("the build never named %q\n%s", want, out)
		}
	}
}

func nodeBinary(t *testing.T, cargo, root string) string {
	t.Helper()

	out, err := runCargo(t, cargo, root, "build", "--bin", "songe-hello-node")
	if err != nil {
		skipOnNetwork(t, out, err)
		t.Fatalf("cargo build: %v\n%s", err, out)
	}

	return filepath.Join(targetDir(t), "debug", "songe-hello-node")
}

func TestAnUnknownPortValueFailsAtRunTimeNamingEveryCandidate(t *testing.T) {
	cargo, root := standUpTheNodeCrate(t)
	binary := nodeBinary(t, cargo, root)

	cmd := exec.Command(binary, "--greeting_store=postgres")
	cmd.Dir = root

	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("the node started on an adapter it never had\n%s", out)
	}

	for _, want := range []string{"GreetingStore", "memory", "sqlite", "postgres"} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("the refusal never named %q\n%s", want, out)
		}
	}
}

func TestADisabledUdpDriverStartsTheNodeWithNoUdpSocket(t *testing.T) {
	cargo, root := standUpTheNodeCrate(t)
	binary := nodeBinary(t, cargo, root)

	cmd := exec.Command(binary, "--driver_udp=false")
	cmd.Dir = root
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("reading the node stdout: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("starting the node: %v", err)
	}

	defer func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_ = cmd.Wait()
	}()

	lines := make(chan string, 8)

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			lines <- scanner.Text()
		}

		close(lines)
	}()

	announced := map[string]bool{}
	deadline := time.After(10 * time.Second)

	for len(announced) < 2 {
		select {
		case line, open := <-lines:
			if !open {
				t.Fatalf("the node stopped after announcing %v", announced)
			}

			if strings.HasPrefix(line, "LISTENING_UDP") {
				t.Fatalf("the node bound a udp socket although the udp driver is disabled: %q", line)
			}

			if strings.HasPrefix(line, "LISTENING_GRPC") || strings.HasPrefix(line, "LISTENING ") {
				announced[strings.Fields(line)[0]] = true
			}
		case <-deadline:
			t.Fatalf("the node announced only %v in ten seconds", announced)
		}
	}
}
