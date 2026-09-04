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
)

const cargoCheckWorkspaceManifest = `[workspace]
resolver = "2"
members = ["core", "app"]
`

const cargoCheckCoreManifest = `[package]
name = "songe-hello-core"
version = "0.1.0"
edition = "2021"

[dependencies]
serde = { version = "1", features = ["derive"] }
thiserror = "2"

[dev-dependencies]
mockall = "0.13"
`

const cargoCheckAppManifest = `[package]
name = "songe-hello-app"
version = "0.1.0"
edition = "2021"

[dependencies]
songe-hello-core = { path = "../core" }
tonic = "0.14"
tonic-prost = "0.14"
prost = "0.14"
tokio = { version = "1", features = ["rt-multi-thread", "macros"] }

[build-dependencies]
protox = "0.9"
tonic-prost-build = "0.14"
`

const cargoCheckCoreLib = `pub mod controller {
    pub mod hello_controller {
        use crate::types::zz_generated_hello_messages::{PingReply, PingRequest};

        #[derive(Debug, thiserror::Error)]
        pub enum HelloControllerError {
            #[error("running ping: not implemented")]
            NotImplemented,
        }

        pub trait HelloController: Send + Sync {
            fn ping(&self, request: PingRequest) -> Result<PingReply, HelloControllerError>;
        }
    }
}

pub mod port {
    #[path = "../../src/port/zz_generated_hello_client.rs"]
    pub mod zz_generated_hello_client;
}

pub mod types {
    #[path = "../../src/types/zz_generated_hello_messages.rs"]
    pub mod zz_generated_hello_messages;
}
`

const cargoCheckAppLib = `pub mod adapter {
    #[path = "../../src/adapter/zz_generated_hello_grpc_client.rs"]
    pub mod zz_generated_hello_grpc_client;
}

pub mod driver {
    #[path = "../../src/driver/zz_generated_hello_grpc_driver.rs"]
    pub mod zz_generated_hello_grpc_driver;
}
`

func TestTheGeneratedSkeletonPassesCargoCheck(t *testing.T) {
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Skip("cargo is not on PATH")
	}

	files, err := grpcrust.Generate([]byte(helloProto), grpcrust.Options{Service: "songe-hello"})
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

	write("Cargo.toml", cargoCheckWorkspaceManifest)
	write("core/Cargo.toml", cargoCheckCoreManifest)
	write("core/src/lib.rs", cargoCheckCoreLib)
	write("app/Cargo.toml", cargoCheckAppManifest)
	write("app/src/lib.rs", cargoCheckAppLib)

	for _, f := range files {
		if f.Path == "app/zz_generated_build.rs" {
			write("app/build.rs", f.Content)

			continue
		}

		write(f.Path, f.Content)
	}

	cmd := exec.Command(cargo, "check", "--workspace", "--all-targets")
	cmd.Dir = root

	out, err := cmd.CombinedOutput()
	if err != nil {
		lower := strings.ToLower(string(out))
		if strings.Contains(lower, "could not resolve host") ||
			strings.Contains(lower, "failed to get") ||
			strings.Contains(lower, "network") ||
			strings.Contains(lower, "spurious network error") {
			t.Skipf("cargo check needs network access to crates.io, which this run did not have: %v\n%s", err, out)
		}

		t.Fatalf("cargo check: %v\n%s", err, out)
	}
}
