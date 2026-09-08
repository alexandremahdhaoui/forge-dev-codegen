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

package tuirust_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/tuirust"
)

const demoDir = "../../demo/tui-rust"

const cargoCrateManifest = `[package]
name = "demo-tui-rust"
version = "0.1.0"
edition = "2021"

[workspace]

[dependencies]
crossterm = "0.29.0"
thiserror = "2"
tokio = { version = "1", features = ["macros", "rt-multi-thread"] }

[dev-dependencies]
mockall = "0.15"
`

const cargoCellLib = `pub mod tui;
`

func readDemoFile(t *testing.T, rel string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(demoDir, rel))
	if err != nil {
		t.Fatalf("reading the demo file %s: %v", rel, err)
	}

	return string(data)
}

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

func standUpTheCell(t *testing.T) (string, string) {
	t.Helper()

	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Skip("cargo is not on PATH")
	}

	spec := readDemoFile(t, "src/tui/tui.yaml")
	controller := readDemoFile(t, "src/tui/controller/board_controller.rs")

	files, err := tuirust.Generate([]byte(spec), tuirust.Options{Service: "demo-tui-rust"})
	if err != nil {
		t.Fatalf("generating the cell: %v", err)
	}

	root := t.TempDir()
	write := writeUnder(t, root)

	write("Cargo.toml", cargoCrateManifest)
	write("src/lib.rs", cargoCellLib)

	for _, f := range files {
		if strings.HasSuffix(f.Path, ".yaml") {
			continue
		}

		write(filepath.Join("src", "tui", f.Path), f.Content)
	}

	write("src/tui/controller/board_controller.rs", controller)

	return cargo, root
}

func runCargo(t *testing.T, cargo, root string, args ...string) {
	t.Helper()

	cmd := exec.Command(cargo, args...)
	cmd.Dir = root

	out, err := cmd.CombinedOutput()
	if err == nil {
		return
	}

	lower := strings.ToLower(string(out))
	if strings.Contains(lower, "could not resolve host") ||
		strings.Contains(lower, "failed to get") ||
		strings.Contains(lower, "spurious network error") {
		t.Skipf("cargo %v needs network access to crates.io, which this run did not have: %v\n%s", args, err, out)
	}

	t.Fatalf("cargo %v: %v\n%s", args, err, out)
}

func TestTheGeneratedCellPassesCargoCheck(t *testing.T) {
	cargo, root := standUpTheCell(t)

	runCargo(t, cargo, root, "check", "--workspace", "--all-targets")
}

func TestTheGeneratedDriverLoopRunsAgainstMockedScreenAndKeyboardPorts(t *testing.T) {
	cargo, root := standUpTheCell(t)

	runCargo(t, cargo, root, "test", "--workspace")
}

func TestDeletingTheControllerImplFailsTheBuildAndNamesTheFile(t *testing.T) {
	cargo, root := standUpTheCell(t)

	if err := os.Remove(filepath.Join(root, "src", "tui", "controller", "board_controller.rs")); err != nil {
		t.Fatalf("removing the controller impl: %v", err)
	}

	cmd := exec.Command(cargo, "check", "--workspace")
	cmd.Dir = root

	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("the build passed without the controller impl\n%s", out)
	}

	text := string(out)

	if !strings.Contains(text, "E0583") || !strings.Contains(text, "board_controller") {
		t.Fatalf("the build never named the missing file\n%s", text)
	}
}
