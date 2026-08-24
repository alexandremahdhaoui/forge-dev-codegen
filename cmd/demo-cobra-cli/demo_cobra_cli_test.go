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

package main_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildDemo builds the real binary: the dispatcher is generated, so
// `go run` proves nothing about exit codes; the built binary does.
func buildDemo(t *testing.T) string {
	t.Helper()

	bin := filepath.Join(t.TempDir(), "demo-cobra-cli")

	cmd := exec.Command("go", "build", "-o", bin, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building demo-cobra-cli: %v\n%s", err, out)
	}

	return bin
}

func run(t *testing.T, bin string, args ...string) (string, int) {
	t.Helper()

	out, err := exec.Command(bin, args...).CombinedOutput()

	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatalf("running demo-cobra-cli: %v", err)
	}

	return string(out), code
}

// TestTheCobraCellKeepsTheBuiltinContract is the drop-in guarantee: the
// same handlers file, the same behavior, a different dispatcher library.
func TestTheCobraCellKeepsTheBuiltinContract(t *testing.T) {
	bin := buildDemo(t)

	t.Run("a handler gets its args and its output through", func(t *testing.T) {
		out, code := run(t, bin, "greet", "world")
		if code != 0 || !strings.Contains(out, "hello [world]") {
			t.Errorf("greet answered %q with code %d", out, code)
		}
	})

	t.Run("exit codes pass through", func(t *testing.T) {
		if _, code := run(t, bin, "fail", "4"); code != 4 {
			t.Errorf("fail must propagate the exit code, got %d", code)
		}
	})

	t.Run("an unknown command fails loud with exit 2", func(t *testing.T) {
		out, code := run(t, bin, "nope")
		if code != 2 || !strings.Contains(out, `unknown command "nope"`) {
			t.Errorf("an unknown command must exit 2 naming itself, got %q with code %d", out, code)
		}
	})

	t.Run("flags reach the handler unparsed", func(t *testing.T) {
		out, code := run(t, bin, "greet", "--loud", "world")
		if code != 0 || !strings.Contains(out, "hello [--loud world]") {
			t.Errorf("the handler owns its args, got %q with code %d", out, code)
		}
	})
}
