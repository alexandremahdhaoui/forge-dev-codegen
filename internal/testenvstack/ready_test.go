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

package testenvstack

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

const echoingService = `#!/bin/sh
echo "args=$@ addr=$SERVICE_ADDR"
echo "LISTENING 4321"
sleep 60
`

func TestReadyValidateAcceptsTheThreeKindsAndRefusesEveryOtherByName(t *testing.T) {
	ports := map[string]int{"grpc": 41}

	for _, ready := range []Ready{
		{Kind: ReadyStdout},
		{Kind: ReadyTCP, Port: "grpc"},
		{Kind: ReadyHTTP, Port: "grpc", Path: "/healthz", Status: 200},
	} {
		if err := ready.Validate(ports); err != nil {
			t.Errorf("%+v: %v", ready, err)
		}
	}

	err := Ready{Kind: "socket"}.Validate(ports)
	if err == nil || !strings.Contains(err.Error(), `readiness kind "socket": not a kind; the kinds are stdout, tcp, http`) {
		t.Errorf("got %v", err)
	}

	err = Ready{}.Validate(ports)
	if err == nil || !strings.Contains(err.Error(), `readiness kind "": not a kind`) {
		t.Errorf("an empty kind is refused here because the engine reads the default off the declaration, got %v", err)
	}
}

func TestReadyValidateRefusesAFieldTheKindDoesNotRead(t *testing.T) {
	ports := map[string]int{"grpc": 41}

	tests := []struct {
		ready  Ready
		reason string
	}{
		{ready: Ready{Kind: ReadyStdout, Port: "grpc"}, reason: "readiness kind stdout: port takes no effect and is refused"},
		{ready: Ready{Kind: ReadyStdout, Path: "/healthz"}, reason: "readiness kind stdout: path takes no effect and is refused"},
		{ready: Ready{Kind: ReadyStdout, Status: 200}, reason: "readiness kind stdout: status takes no effect and is refused"},
		{ready: Ready{Kind: ReadyTCP, Port: "grpc", Path: "/healthz"}, reason: "readiness kind tcp: path takes no effect and is refused"},
		{ready: Ready{Kind: ReadyTCP, Port: "grpc", Status: 200}, reason: "readiness kind tcp: status takes no effect and is refused"},
	}

	for _, tt := range tests {
		err := tt.ready.Validate(ports)
		if err == nil || !strings.Contains(err.Error(), tt.reason) {
			t.Errorf("%+v: got %v", tt.ready, err)
		}
	}
}

func TestReadyValidateRefusesAMissingPortAPortNobodyDeclaredAndAnIncompleteHttpProbe(t *testing.T) {
	ports := map[string]int{"grpc": 41}

	tests := []struct {
		ready  Ready
		reason string
	}{
		{ready: Ready{Kind: ReadyTCP}, reason: "readiness kind tcp: port is required and names one of the declared ports grpc"},
		{ready: Ready{Kind: ReadyTCP, Port: "http"}, reason: `readiness kind tcp: port "http" is not declared by this service; the declared ports are grpc`},
		{ready: Ready{Kind: ReadyHTTP, Port: "grpc", Status: 200}, reason: "readiness kind http: path is required"},
		{ready: Ready{Kind: ReadyHTTP, Port: "grpc", Path: "/healthz"}, reason: "readiness kind http: status is required"},
	}

	for _, tt := range tests {
		err := tt.ready.Validate(ports)
		if err == nil || !strings.Contains(err.Error(), tt.reason) {
			t.Errorf("%+v: got %v", tt.ready, err)
		}
	}
}

func TestAwaitTcpReturnsAsSoonAsThePortAcceptsAndTimesOutWhenNothingListens(t *testing.T) {
	listener, err := net.Listen("tcp", LoopbackHost+":0")
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = listener.Close() }()

	if err := awaitTCP(context.Background(), listener.Addr().String(), 5*time.Second, nil); err != nil {
		t.Errorf("an open port must answer: %v", err)
	}

	free, err := AllocatePorts([]string{"gone"})
	if err != nil {
		t.Fatal(err)
	}

	address := probeAddress(free, "gone")

	err = awaitTCP(context.Background(), address, 300*time.Millisecond, nil)
	if err == nil || !strings.Contains(err.Error(), "no tcp connection to "+address+" within") {
		t.Errorf("got %v", err)
	}
}

func TestAwaitHttpReturnsOnTheDeclaredStatusAndTimesOutOnEveryOther(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	defer server.Close()

	if err := awaitHTTP(context.Background(), server.URL+"/healthz", http.StatusOK, 5*time.Second, nil); err != nil {
		t.Errorf("a matching status must answer: %v", err)
	}

	err := awaitHTTP(context.Background(), server.URL+"/healthz", http.StatusNoContent, 300*time.Millisecond, nil)
	if err == nil || !strings.Contains(err.Error(), "no 204 from "+server.URL+"/healthz within") {
		t.Errorf("got %v", err)
	}
}

func TestAwaitReportsAProcessThatExitedAndACancelledContext(t *testing.T) {
	exited := make(chan error, 1)
	exited <- os.ErrClosed

	err := awaitTCP(context.Background(), "127.0.0.1:1", 5*time.Second, exited)
	if err == nil || !strings.Contains(err.Error(), "exited before answering the probe") {
		t.Errorf("got %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = awaitTCP(ctx, "127.0.0.1:1", 5*time.Second, nil)
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("got %v", err)
	}
}

func TestStartBindsEveryDeclaredPortAndPutsItIntoTheArgsAndTheEnv(t *testing.T) {
	binary := writeBinary(t, echoingService)
	tmpDir := t.TempDir()

	started, err := Start(context.Background(), tmpDir, nil, Service{
		Name:         "hello",
		Binary:       binary,
		Args:         []string{"--grpc", "127.0.0.1:@port.grpc@", "--db", "@tmpDir@/hello.db"},
		AddrEnv:      "SERVICE_ADDR",
		Env:          map[string]string{"SERVICE_ADDR": "127.0.0.1:@port.http@"},
		Ports:        []string{"grpc", "http"},
		Ready:        Ready{Kind: ReadyStdout},
		ReadyTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	defer Stop([]int{started.PID}, 2*time.Second)

	log, err := os.ReadFile(started.LogPath)
	if err != nil {
		t.Fatal(err)
	}

	want := "args=--grpc " + probeAddress(started.Allocated, "grpc") + " --db " + tmpDir + "/hello.db addr=" + probeAddress(started.Allocated, "http")
	if !strings.Contains(string(log), want) {
		t.Errorf("log:\n%s\nwant %q", log, want)
	}
}

func TestStartWithATcpProbeReportsATimeoutWhenNothingEverBinds(t *testing.T) {
	binary := writeBinary(t, silentService)

	_, err := Start(context.Background(), t.TempDir(), nil, Service{
		Name:         "silent",
		Binary:       binary,
		AddrEnv:      "SILENT_ADDR",
		Ports:        []string{"main"},
		Ready:        Ready{Kind: ReadyTCP, Port: "main"},
		ReadyTimeout: 300 * time.Millisecond,
	})
	if err == nil || !strings.Contains(err.Error(), "no tcp connection to 127.0.0.1:") {
		t.Errorf("got %v", err)
	}
}

func TestStartWithAProbeReportsAProcessThatExitsBeforeAnsweringIt(t *testing.T) {
	binary := writeBinary(t, exitingService)

	_, err := Start(context.Background(), t.TempDir(), nil, Service{
		Name:         "dying",
		Binary:       binary,
		AddrEnv:      "DYING_ADDR",
		Ports:        []string{"main"},
		Ready:        Ready{Kind: ReadyTCP, Port: "main"},
		ReadyTimeout: 5 * time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "exited before answering the probe") {
		t.Errorf("got %v", err)
	}
}

func TestStartRefusesAPlaceholderNamingAPortTheServiceDidNotDeclare(t *testing.T) {
	binary := writeBinary(t, silentService)

	_, err := Start(context.Background(), t.TempDir(), nil, Service{
		Name:    "typo",
		Binary:  binary,
		AddrEnv: "TYPO_ADDR",
		Args:    []string{"--grpc", "@port.grpc@"},
		Ports:   []string{"http"},
		Ready:   Ready{Kind: ReadyTCP, Port: "http"},
	})
	if err == nil || !strings.Contains(err.Error(), `port "grpc" is not declared by this service; the declared ports are http`) {
		t.Errorf("got %v", err)
	}
}

func TestStartRefusesAReadinessKindItDoesNotKnowBeforeStartingAnything(t *testing.T) {
	binary := writeBinary(t, silentService)

	_, err := Start(context.Background(), t.TempDir(), nil, Service{Name: "odd", Binary: binary, AddrEnv: "ODD_ADDR", Ready: Ready{Kind: "socket"}})
	if err == nil || !strings.Contains(err.Error(), `readiness kind "socket": not a kind`) {
		t.Errorf("got %v", err)
	}
}

func TestStartRefusesAPortNameDeclaredTwice(t *testing.T) {
	binary := writeBinary(t, silentService)

	_, err := Start(context.Background(), t.TempDir(), nil, Service{
		Name:    "twice",
		Binary:  binary,
		AddrEnv: "TWICE_ADDR",
		Ports:   []string{"http", "http"},
		Ready:   Ready{Kind: ReadyTCP, Port: "http"},
	})
	if err == nil || !strings.Contains(err.Error(), "the name is declared twice") {
		t.Errorf("got %v", err)
	}
}

func TestAServiceThatDeclaresPortsIsNotHandedTheZeroPortOnItsAddrEnv(t *testing.T) {
	env := strings.Join(Environment(nil, Service{Name: "one", AddrEnv: "ONE_ADDR", Ports: []string{"grpc"}}), "\n")
	if strings.Contains(env, "ONE_ADDR=") {
		t.Errorf("a declared port replaces the zero port trick:\n%s", env)
	}

	env = strings.Join(Environment(nil, Service{Name: "two", AddrEnv: "TWO_ADDR"}), "\n")
	if !strings.Contains(env, "TWO_ADDR=127.0.0.1:0") {
		t.Errorf("a service that declares no port keeps the zero port trick:\n%s", env)
	}
}
