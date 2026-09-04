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

package testenvsqlite

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

const pythonWriter = "import sqlite3, sys\nconnection = sqlite3.connect(sys.argv[1])\nconnection.executescript(sys.stdin.read())\nconnection.commit()\n"

type Writer struct {
	Name    string
	command string
	args    func(path string) []string
}

func DetectWriter() (Writer, error) {
	if sqlite3, err := exec.LookPath("sqlite3"); err == nil {
		return Writer{
			Name:    "sqlite3",
			command: sqlite3,
			args:    func(path string) []string { return []string{path} },
		}, nil
	}

	if python, err := exec.LookPath("python3"); err == nil {
		return Writer{
			Name:    "python3",
			command: python,
			args:    func(path string) []string { return []string{"-c", pythonWriter, path} },
		}, nil
	}

	return Writer{}, fmt.Errorf("finding a sqlite writer: neither sqlite3 nor python3 is on PATH")
}

func (w Writer) Write(ctx context.Context, path string, script string) error {
	cmd := exec.CommandContext(ctx, w.command, w.args(path)...)
	cmd.Stdin = strings.NewReader(script)

	var stderr bytes.Buffer

	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("writing %s through %s: %w: %s", path, w.Name, err, strings.TrimSpace(stderr.String()))
	}

	return nil
}
