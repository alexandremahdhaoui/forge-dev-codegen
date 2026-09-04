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

package main

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alexandremahdhaoui/forge/pkg/engineframework"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/testenvstack"
)

const fakeService = `#!/bin/sh
echo "addr=$HELLO_ADDR store=$SONGE_STORE_GREETING_PATH mode=$MODE"
echo "LISTENING 4321"
sleep 60
`

const exitingService = `#!/bin/sh
exit 1
`

func writeBinary(t *testing.T, root string, name string, script string) string {
	t.Helper()

	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	return path
}

func TestTheSpecParsesServicesFromAForgeYamlSpecBlockAndRefusesAnEmptyList(t *testing.T) {
	spec, err := FromMap(map[string]any{"services": []any{map[string]any{
		"name": "hello", "binary": "./build/bin/hello", "addrEnv": "HELLO_URL",
		"env": map[string]any{"MODE": "test"}, "readyTimeoutSeconds": float64(3),
	}}})
	if err != nil {
		t.Fatal(err)
	}

	if len(spec.Services) != 1 {
		t.Fatalf("spec: %+v", spec)
	}

	svc := spec.Services[0]
	if svc.Name != "hello" || svc.Binary != "./build/bin/hello" || svc.AddrEnv != "HELLO_URL" || svc.Env["MODE"] != "test" || svc.ReadyTimeoutSeconds != 3 {
		t.Errorf("service: %+v", svc)
	}

	if out := ValidateMap(map[string]any{}); out.Valid {
		t.Error("a spec without services must be invalid")
	}

	if out := ValidateMap(map[string]any{"services": []any{map[string]any{"name": "hello"}}}); out.Valid {
		t.Error("a service without binary and addrEnv must be invalid")
	}
}

func TestToServiceResolvesTheBinaryAgainstTheRootAndDefaultsTheTimeout(t *testing.T) {
	svc := toService("/root", Service{Name: "hello", Binary: "build/hello", AddrEnv: "HELLO_URL"})

	if svc.Binary != "/root/build/hello" || svc.ReadyTimeout != 0 {
		t.Errorf("service: %+v", svc)
	}

	svc = toService("/root", Service{Name: "hello", Binary: "/abs/hello", ReadyTimeoutSeconds: 2})

	if svc.Binary != "/abs/hello" || svc.ReadyTimeout != 2*time.Second {
		t.Errorf("service: %+v", svc)
	}
}

func TestCreateStartsEveryServiceExportsItsAddressAndDeleteStopsIt(t *testing.T) {
	root := t.TempDir()
	writeBinary(t, root, "hello", fakeService)
	tmpDir := t.TempDir()

	input := engineframework.CreateInput{
		TestID:  "t1",
		Stage:   "integration",
		TmpDir:  tmpDir,
		RootDir: root,
		Env:     map[string]string{"SONGE_STORE_GREETING_PATH": "/db/greeting.db"},
	}

	spec := &Spec{Services: []Service{{Name: "hello", Binary: "hello", AddrEnv: "HELLO_URL", Env: map[string]string{"MODE": "test"}, ReadyTimeoutSeconds: 5}}}

	artifact, err := Create(context.Background(), input, spec)
	if err != nil {
		t.Fatal(err)
	}

	if artifact.Env["HELLO_URL"] != "http://127.0.0.1:4321" {
		t.Errorf("env: %v", artifact.Env)
	}

	if artifact.Files["stack.hello.log"] != "hello.log" || artifact.Files["stack.pids"] != "stack.pids" {
		t.Errorf("files: %v", artifact.Files)
	}

	pid, err := strconv.Atoi(artifact.Metadata["testenv-stack.hello.pid"])
	if err != nil || pid <= 0 {
		t.Fatalf("metadata: %v", artifact.Metadata)
	}

	if artifact.Metadata[pidsPathKey] != filepath.Join(tmpDir, "stack.pids") {
		t.Errorf("metadata: %v", artifact.Metadata)
	}

	log, _ := os.ReadFile(filepath.Join(tmpDir, "hello.log"))
	if !strings.Contains(string(log), "addr=127.0.0.1:0 store=/db/greeting.db mode=test") {
		t.Errorf("log: %s", log)
	}

	if !testenvstack.Alive(pid) {
		t.Fatal("the service must outlive Create")
	}

	if err := Delete(context.Background(), engineframework.DeleteInput{TestID: "t1", Metadata: artifact.Metadata}, nil); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for testenvstack.Alive(pid) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}

	if testenvstack.Alive(pid) {
		t.Error("the service must be gone after Delete")
	}
}

func TestCreateStopsTheServicesAlreadyStartedWhenALaterOneFails(t *testing.T) {
	root := t.TempDir()
	writeBinary(t, root, "hello", fakeService)
	writeBinary(t, root, "broken", exitingService)

	input := engineframework.CreateInput{TestID: "t1", Stage: "s", TmpDir: t.TempDir(), RootDir: root}
	spec := &Spec{Services: []Service{
		{Name: "hello", Binary: "hello", AddrEnv: "HELLO_URL", ReadyTimeoutSeconds: 5},
		{Name: "broken", Binary: "broken", AddrEnv: "BROKEN_URL", ReadyTimeoutSeconds: 5},
	}}

	if _, err := Create(context.Background(), input, spec); err == nil {
		t.Fatal("a failing service must fail create")
	}

	pids, err := testenvstack.ReadPids(filepath.Join(input.TmpDir, "stack.pids"))
	if err == nil && len(pids) > 0 {
		t.Errorf("no pid file must survive a failed create: %v", pids)
	}
}

func TestCreateReportsAPidFileThatCannotBeWritten(t *testing.T) {
	root := t.TempDir()
	writeBinary(t, root, "hello", fakeService)

	tmpDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmpDir, "stack.pids"), 0o755); err != nil {
		t.Fatal(err)
	}

	input := engineframework.CreateInput{TestID: "t1", Stage: "s", TmpDir: tmpDir, RootDir: root}
	spec := &Spec{Services: []Service{{Name: "hello", Binary: "hello", AddrEnv: "HELLO_URL", ReadyTimeoutSeconds: 5}}}

	if _, err := Create(context.Background(), input, spec); err == nil {
		t.Error("an unwritable pid file must fail create")
	}
}

func TestDeleteWithoutAPidFileDoesNothing(t *testing.T) {
	if err := Delete(context.Background(), engineframework.DeleteInput{TestID: "t1"}, nil); err != nil {
		t.Error(err)
	}

	gone := filepath.Join(t.TempDir(), "stack.pids")
	if err := Delete(context.Background(), engineframework.DeleteInput{TestID: "t1", Metadata: map[string]string{pidsPathKey: gone}}, nil); err != nil {
		t.Error(err)
	}
}

func TestDeleteReportsAPidFileItCannotRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stack.pids")
	if err := os.WriteFile(path, []byte("hello notanumber\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Delete(context.Background(), engineframework.DeleteInput{TestID: "t1", Metadata: map[string]string{pidsPathKey: path}}, nil); err == nil {
		t.Error("a broken pid file must be reported")
	}
}
