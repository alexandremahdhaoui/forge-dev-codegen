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
echo "addr=$HELLO_URL store=$SONGE_STORE_GREETING_PATH mode=$MODE"
echo "LISTENING 4321"
sleep 60
`

const nodeService = `#!/bin/sh
echo "LISTENING 4321"
echo "LISTENING_GRPC 4322"
echo "LISTENING_UDP 4323"
sleep 60
`

const portedService = `#!/bin/sh
echo "args=$@"
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

func TestCreateExportsOneAddressPerPortTheServiceAnnounces(t *testing.T) {
	root := t.TempDir()
	writeBinary(t, root, "hello", nodeService)
	tmpDir := t.TempDir()

	spec := &Spec{Services: []Service{{Name: "hello", Binary: "hello", AddrEnv: "HELLO_URL", ReadyTimeoutSeconds: 5}}}

	artifact, err := Create(context.Background(), engineframework.CreateInput{TestID: "t2", TmpDir: tmpDir, RootDir: root}, spec)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = Delete(context.Background(), engineframework.DeleteInput{TestID: "t2", Metadata: artifact.Metadata}, nil)
	}()

	want := map[string]string{
		"HELLO_URL":      "http://127.0.0.1:4321",
		"HELLO_URL_GRPC": "http://127.0.0.1:4322",
		"HELLO_URL_UDP":  "127.0.0.1:4323",
	}

	for key, value := range want {
		if artifact.Env[key] != value {
			t.Errorf("%s is %q, want %q", key, artifact.Env[key], value)
		}
	}
}

func TestAnAbsentReadyBlockReadsAsTheStdoutKeywordSoAServiceWrittenBeforePortsIsUntouched(t *testing.T) {
	if got := toReady(Ready{}); got.Kind != testenvstack.ReadyStdout {
		t.Errorf("got %+v", got)
	}

	got := toReady(Ready{Kind: "http", Port: "http", Path: "/healthz", Status: 200})
	if got != (testenvstack.Ready{Kind: "http", Port: "http", Path: "/healthz", Status: 200}) {
		t.Errorf("got %+v", got)
	}
}

func TestAnAfterCommandWithAPathResolvesAgainstTheRootAndABareNameStaysOnThePath(t *testing.T) {
	if got := resolveCommand("/root", "seed"); got != "seed" {
		t.Errorf("got %q", got)
	}

	if got := resolveCommand("/root", "./build/bin/demo-stack"); got != "/root/build/bin/demo-stack" {
		t.Errorf("got %q", got)
	}

	if got := resolveCommand("/root", "/usr/bin/seed"); got != "/usr/bin/seed" {
		t.Errorf("got %q", got)
	}
}

func TestTheSpecParsesArgsPortsReadyAndAfter(t *testing.T) {
	spec, err := FromMap(map[string]any{"services": []any{map[string]any{
		"name": "node", "binary": "/bin/sh", "addrEnv": "NODE_ADDR",
		"args":  []any{"-c", "run --grpc @port.grpc@"},
		"ports": []any{"grpc", "http"},
		"ready": map[string]any{"kind": "http", "port": "http", "path": "/healthz", "status": float64(200)},
		"after": []any{map[string]any{
			"command": "seed", "args": []any{"store", "create"}, "export": "STORE_ID", "jsonPath": "store.id",
		}},
	}}})
	if err != nil {
		t.Fatal(err)
	}

	svc := spec.Services[0]
	if len(svc.Args) != 2 || svc.Args[1] != "run --grpc @port.grpc@" {
		t.Errorf("args: %v", svc.Args)
	}

	if len(svc.Ports) != 2 || svc.Ports[0] != "grpc" || svc.Ports[1] != "http" {
		t.Errorf("ports: %v", svc.Ports)
	}

	if svc.Ready.Kind != "http" || svc.Ready.Port != "http" || svc.Ready.Path != "/healthz" || svc.Ready.Status != 200 {
		t.Errorf("ready: %+v", svc.Ready)
	}

	if len(svc.After) != 1 || svc.After[0].Export != "STORE_ID" || svc.After[0].JsonPath != "store.id" {
		t.Errorf("after: %+v", svc.After)
	}

	if out := ValidateMap(map[string]any{"services": []any{map[string]any{
		"name": "one", "binary": "/bin/sh", "addrEnv": "ONE_ADDR",
		"ports": []any{"grpc"},
	}}}); !out.Valid {
		t.Errorf("a service that declares ports and no ready block is valid, got %+v", out.Errors)
	}

	if out := ValidateMap(map[string]any{"services": []any{map[string]any{
		"name": "one", "binary": "/bin/sh", "addrEnv": "ONE_ADDR",
		"after": []any{map[string]any{"command": "seed"}},
	}}}); out.Valid {
		t.Error("an after command without export must be invalid")
	}
}

func TestCreateBindsTheDeclaredPortsPutsThemInTheArgsExportsThemAndRunsTheAfterCommands(t *testing.T) {
	root := t.TempDir()
	writeBinary(t, root, "hello", portedService)
	tmpDir := t.TempDir()

	spec := &Spec{Services: []Service{{
		Name: "hello", Binary: "hello", AddrEnv: "HELLO_URL",
		Args:  []string{"--grpc", "@port.grpc@"},
		Ports: []string{"grpc", "http"},
		After: []After{
			{Command: "printf", Args: []string{`{"store":{"id":"01STORE"}}`}, Export: "STORE_ID", JsonPath: "store.id"},
			{Command: "sh", Args: []string{"-c", `printf "model=m-%s" "$STORE_ID"`}, Export: "MODEL_ID", Regex: `model=(\S+)`},
		},
		ReadyTimeoutSeconds: 5,
	}}}

	artifact, err := Create(context.Background(), engineframework.CreateInput{TestID: "t3", TmpDir: tmpDir, RootDir: root}, spec)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = Delete(context.Background(), engineframework.DeleteInput{TestID: "t3", Metadata: artifact.Metadata}, nil)
	}()

	grpc := artifact.Env["HELLO_URL_GRPC"]
	if !strings.HasPrefix(grpc, "127.0.0.1:") {
		t.Errorf("HELLO_URL_GRPC is %q", grpc)
	}

	if artifact.Env["HELLO_URL_HTTP"] == "" || artifact.Env["HELLO_URL_HTTP"] == grpc {
		t.Errorf("env: %v", artifact.Env)
	}

	if artifact.Env["STORE_ID"] != "01STORE" || artifact.Env["MODEL_ID"] != "m-01STORE" {
		t.Errorf("env: %v", artifact.Env)
	}

	if artifact.Metadata["testenv-stack.hello.export.STORE_ID"] != "01STORE" || artifact.Metadata["testenv-stack.hello.export.MODEL_ID"] != "m-01STORE" {
		t.Errorf("metadata: %v", artifact.Metadata)
	}

	log, _ := os.ReadFile(filepath.Join(tmpDir, "hello.log"))
	if !strings.Contains(string(log), "args=--grpc "+strings.TrimPrefix(grpc, "127.0.0.1:")) {
		t.Errorf("log: %s", log)
	}
}

func TestCreateStopsTheServiceWhenAnAfterCommandFindsNothingToExport(t *testing.T) {
	root := t.TempDir()
	writeBinary(t, root, "hello", portedService)
	input := engineframework.CreateInput{TestID: "t4", TmpDir: t.TempDir(), RootDir: root}

	spec := &Spec{Services: []Service{{
		Name: "hello", Binary: "hello", AddrEnv: "HELLO_URL", Ports: []string{"grpc"},
		After:               []After{{Command: "printf", Args: []string{"nothing"}, Export: "STORE_ID", Regex: `id=(\S+)`}},
		ReadyTimeoutSeconds: 5,
	}}}

	if _, err := Create(context.Background(), input, spec); err == nil || !strings.Contains(err.Error(), "exporting STORE_ID") {
		t.Fatalf("got %v", err)
	}

	pids, err := testenvstack.ReadPids(filepath.Join(input.TmpDir, "stack.pids"))
	if err == nil && len(pids) > 0 {
		t.Errorf("no pid file must survive a failed create: %v", pids)
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
