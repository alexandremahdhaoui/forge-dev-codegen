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
	"net"
	"strconv"
	"strings"
	"testing"
)

func TestAllocatePortsGivesEveryDeclaredNameAFreeAndDistinctPort(t *testing.T) {
	ports, err := AllocatePorts([]string{"grpc", "http"})
	if err != nil {
		t.Fatal(err)
	}

	if len(ports) != 2 || ports["grpc"] == 0 || ports["http"] == 0 {
		t.Fatalf("ports: %v", ports)
	}

	if ports["grpc"] == ports["http"] {
		t.Errorf("two names must not share a port, got %v", ports)
	}

	for name, port := range ports {
		listener, err := net.Listen("tcp", LoopbackHost+":"+strconv.Itoa(port))
		if err != nil {
			t.Errorf("port %s is not free after allocation: %v", name, err)

			continue
		}

		_ = listener.Close()
	}
}

func TestAllocatePortsRefusesAnEmptyNameAndANameDeclaredTwice(t *testing.T) {
	if _, err := AllocatePorts([]string{""}); err == nil || !strings.Contains(err.Error(), "the name is empty") {
		t.Errorf("got %v", err)
	}

	_, err := AllocatePorts([]string{"grpc", "grpc"})
	if err == nil || !strings.Contains(err.Error(), `allocating port "grpc": the name is declared twice`) {
		t.Errorf("got %v", err)
	}
}

func TestAllocatePortsOfAServiceThatDeclaresNoneAnswersAnEmptyMap(t *testing.T) {
	ports, err := AllocatePorts(nil)
	if err != nil || len(ports) != 0 {
		t.Errorf("got %v %v", ports, err)
	}
}

func TestSubstitutePutsEveryDeclaredPortAndTheTmpDirIntoTheText(t *testing.T) {
	placeholders := Placeholders{Ports: map[string]int{"grpc": 41, "http": 42}, TmpDir: "/tmp/stack"}

	got, err := placeholders.Substitute("--grpc 127.0.0.1:@port.grpc@ --http 127.0.0.1:@port.http@ --db @tmpDir@/service.db")
	if err != nil {
		t.Fatal(err)
	}

	if got != "--grpc 127.0.0.1:41 --http 127.0.0.1:42 --db /tmp/stack/service.db" {
		t.Errorf("got %q", got)
	}

	if got, err := placeholders.Substitute("no placeholder here"); err != nil || got != "no placeholder here" {
		t.Errorf("got %q %v", got, err)
	}
}

func TestSubstituteRefusesAPortTheServiceDidNotDeclareAndNamesTheOnesItDid(t *testing.T) {
	placeholders := Placeholders{Ports: map[string]int{"grpc": 41, "http": 42}}

	_, err := placeholders.Substitute("--admin @port.admin@")
	if err == nil || !strings.Contains(err.Error(), `port "admin" is not declared by this service; the declared ports are grpc, http`) {
		t.Errorf("got %v", err)
	}
}

func TestSubstituteRefusesAPlaceholderThatIsNeitherAPortNorTheTmpDir(t *testing.T) {
	placeholders := Placeholders{Ports: map[string]int{"grpc": 41}}

	_, err := placeholders.Substitute("@ports.grpc@")
	if err == nil || !strings.Contains(err.Error(), `placeholder "ports.grpc" names nothing; the placeholders are @tmpDir@ and @port.<name>@ over grpc`) {
		t.Errorf("got %v", err)
	}

	_, err = Placeholders{}.Substitute("@port.grpc@")
	if err == nil || !strings.Contains(err.Error(), "no port at all") {
		t.Errorf("a service with no port must be told so, got %v", err)
	}
}

func TestSubstituteRefusesAnOpeningDelimiterWithNoClosingDelimiter(t *testing.T) {
	_, err := Placeholders{}.Substitute("--db @tmpDir")
	if err == nil || !strings.Contains(err.Error(), "an opening @ has no closing @") {
		t.Errorf("got %v", err)
	}
}

func TestSubstituteLeavesTheDoubleBraceOfTheOrchestratorAlone(t *testing.T) {
	got, err := Placeholders{}.Substitute("{{allocateOpenPort \"127.0.0.1\" \"demo\"}}")
	if err != nil || got != "{{allocateOpenPort \"127.0.0.1\" \"demo\"}}" {
		t.Errorf("forge expands its own templates before the engine sees them, got %q %v", got, err)
	}
}

func TestSubstituteAllAndSubstituteMapNameTheItemAndTheKeyThatFailed(t *testing.T) {
	placeholders := Placeholders{Ports: map[string]int{"grpc": 41}}

	args, err := placeholders.SubstituteAll([]string{"-p", "@port.grpc@"})
	if err != nil || len(args) != 2 || args[1] != "41" {
		t.Fatalf("got %v %v", args, err)
	}

	if _, err := placeholders.SubstituteAll([]string{"ok", "@port.nope@"}); err == nil || !strings.Contains(err.Error(), "reading item 1") {
		t.Errorf("got %v", err)
	}

	env, err := placeholders.SubstituteMap(map[string]string{"ADDR": "127.0.0.1:@port.grpc@"})
	if err != nil || env["ADDR"] != "127.0.0.1:41" {
		t.Fatalf("got %v %v", env, err)
	}

	if _, err := placeholders.SubstituteMap(map[string]string{"ADDR": "@port.nope@"}); err == nil || !strings.Contains(err.Error(), `reading key "ADDR"`) {
		t.Errorf("got %v", err)
	}

	if args, err := placeholders.SubstituteAll(nil); args != nil || err != nil {
		t.Errorf("an absent list stays absent, got %v %v", args, err)
	}

	if env, err := placeholders.SubstituteMap(nil); env != nil || err != nil {
		t.Errorf("an absent map stays absent, got %v %v", env, err)
	}
}

func TestPortAddressesExportOneBareAddressPerDeclaredPortUnderTheAddrEnv(t *testing.T) {
	env := PortAddresses("SONGE_OPENFGA_ADDR", map[string]int{"grpc": 41, "http": 42})

	want := map[string]string{
		"SONGE_OPENFGA_ADDR_GRPC": "127.0.0.1:41",
		"SONGE_OPENFGA_ADDR_HTTP": "127.0.0.1:42",
	}

	for key, value := range want {
		if env[key] != value {
			t.Errorf("%s is %q, want %q", key, env[key], value)
		}
	}

	if len(env) != len(want) {
		t.Errorf("env: %v", env)
	}
}
