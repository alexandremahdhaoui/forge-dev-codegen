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

package vectorsrust_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/restrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/udprust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/vectorsrust"
)

const sessionProto = `syntax = "proto3";

package songe.hello.udp.v1;

service HelloDatagram {
  rpc Hello(Hello) returns (Welcome);
  rpc Echo(Echo) returns (Echo);
  rpc Counter(Counter) returns (Nothing);
}

message Hello {
  string secret = 1;
}

message Welcome {
  string greeting = 1;
}

message Echo {
  string payload = 1;
}

message Counter {
  uint64 tick = 1;
}

message Nothing {}
`

const sessionUdpCases = `
    {
      "case": "udp_echo_after_a_hello_comes_back",
      "operation": "udp_echo",
      "input": { "sessionId": "0123456789abcdef", "payload": "songe" },
      "hello": { "secret": "open" },
      "controllerReply": { "payload": "songe" },
      "expectedBody": { "sessionId": "0123456789abcdef", "payload": "songe" }
    },
    {
      "case": "udp_hello_with_a_wrong_secret_is_dropped",
      "operation": "udp_hello",
      "input": { "sessionId": "0123456789abcdef", "secret": "wrong" },
      "gate": "refuse",
      "expectDropped": true
    },
    {
      "case": "udp_hello_with_the_secret_is_welcomed",
      "operation": "udp_hello",
      "input": { "sessionId": "0123456789abcdef", "secret": "open" },
      "gate": "admit",
      "controllerReply": { "greeting": "welcome" },
      "expectedBody": { "sessionId": "0123456789abcdef", "greeting": "welcome" }
    },
    {
      "case": "udp_echo_from_an_unknown_session_is_dropped",
      "operation": "udp_echo",
      "input": { "sessionId": "fedcba9876543210", "payload": "songe" },
      "session": "unknown",
      "expectDropped": true
    },
    {
      "case": "udp_hello_again_from_a_new_address_replaces_the_peer",
      "operation": "udp_hello",
      "input": { "sessionId": "0123456789abcdef", "secret": "open" },
      "reconnect": true,
      "controllerReply": { "greeting": "welcome" },
      "expectedBody": { "sessionId": "0123456789abcdef", "greeting": "welcome" }
    },
    {
      "case": "udp_counter_reaches_two_sessions_on_a_tick",
      "operation": "udp_counter",
      "input": null,
      "hello": { "secret": "open" },
      "expectPush": {
        "rpc": "Counter",
        "sessionIds": ["0123456789abcdef", "fedcba9876543210"],
        "payload": { "tick": 3 }
      }
    }`

const sessionCases = `{
  "cases": [` + sessionUdpCases + `
  ]
}`

func sessionOptions() vectorsrust.Options {
	return vectorsrust.Options{
		Service: "songe-hello",
		Proto:   []byte(sessionProto),
		Hello:   "Hello",
		Push:    []string{"Counter"},
	}
}

func generateSession(t *testing.T, cases string) string {
	t.Helper()

	files, err := vectorsrust.Generate([]byte(helloSpec), []byte(cases), sessionOptions())
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	return files[0].Content
}

func TestASessionCellGetsAGateMockAStackHelperAndOneTestPerCaseKind(t *testing.T) {
	content := generateSession(t, sessionCases)

	for _, want := range []string{
		"fn on_tick(&self, tick: u64) -> Result<(), HelloDatagramControllerError>;",
		"pub HelloDatagramSessionGate {}",
		"fn admit(&self, session_id: &[u8; 16], request: &Hello, peer: std::net::SocketAddr) -> Result<Admission, HelloDatagramSessionGateError>;",
		"async fn stand_up_hello_datagram(",
		"async fn register_hello_datagram(",
		"async fn receive_hello_datagram_push(",
		"async fn udp_echo_after_a_hello_comes_back() {",
		"let _registered = register_hello_datagram(stack.port, \"0123456789abcdef\", Hello { secret: \"open\".to_string() }).await;",
		"async fn udp_hello_with_a_wrong_secret_is_dropped() {",
		"hello_datagram_gate(false)",
		".expect_err(\"the datagram is dropped, no reply comes back\");",
		"timeout_ms: 300,",
		"async fn udp_hello_with_the_secret_is_welcomed() {",
		"async fn udp_echo_from_an_unknown_session_is_dropped() {",
		"async fn udp_hello_again_from_a_new_address_replaces_the_peer() {",
		".times(2)",
		"Some(second_address),",
		"async fn udp_counter_reaches_two_sessions_on_a_tick() {",
		".send_all(HelloDatagramPush::Counter(Counter { tick: 3 }))",
		"(\"fedcba9876543210\", register_hello_datagram(stack.port, \"fedcba9876543210\", Hello { secret: \"open\".to_string() }).await.0),",
		"let mut tick = HelloDatagramTickDriver::new(",
		"interval_ms: 10,",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("the emitted file lacks %q\n%s", want, content)
		}
	}

	if strings.Contains(content, "fn counter(&self") {
		t.Errorf("the controller mock got a method for the push rpc\n%s", content)
	}
}

func TestASessionFieldOnACellWithNoHelloIsRefused(t *testing.T) {
	_, err := vectorsrust.Generate([]byte(helloSpec), []byte(`{
  "cases": [
    {
      "case": "udp_echo_dropped",
      "operation": "udp_echo",
      "input": { "sessionId": "0123456789abcdef", "payload": "songe" },
      "expectDropped": true
    }
  ]
}`), vectorsrust.Options{Service: "songe-hello", Proto: []byte(datagramProto)})

	want := `reading vector "udp_echo_dropped": it uses a session field and the cell names no layout.hello`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("generating reported %v, want %q", err, want)
	}
}

func TestAPushCaseIsRefusedWhenItCarriesInputNamesAnotherRpcOrNoSession(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "input on a push",
			body: `{"case": "c", "operation": "udp_counter", "input": {"tick": 1}, "expectPush": {"sessionIds": ["0123456789abcdef"]}}`,
			want: `a push case carries no input`,
		},
		{
			name: "no session ids",
			body: `{"case": "c", "operation": "udp_counter", "expectPush": {"sessionIds": []}}`,
			want: `expectPush needs sessionIds`,
		},
		{
			name: "a short session id",
			body: `{"case": "c", "operation": "udp_counter", "expectPush": {"sessionIds": ["short"]}}`,
			want: `must be 16 bytes, got 5`,
		},
		{
			name: "an rpc that disagrees with the operation",
			body: `{"case": "c", "operation": "udp_counter", "expectPush": {"rpc": "Echo", "sessionIds": ["0123456789abcdef"]}}`,
			want: `expectPush names rpc "Echo" and the operation names Counter`,
		},
		{
			name: "a push on an inbound rpc",
			body: `{"case": "c", "operation": "udp_echo", "expectPush": {"sessionIds": ["0123456789abcdef"]}}`,
			want: `expectPush names Echo and the cell does not list it under layout.push`,
		},
		{
			name: "a request on a push rpc",
			body: `{"case": "c", "operation": "udp_counter", "input": {"sessionId": "0123456789abcdef", "tick": 1}, "controllerReply": {}, "expectedBody": {}}`,
			want: `Counter is a push rpc, a push case carries expectPush and no input`,
		},
		{
			name: "a reconnect on an echo",
			body: `{"case": "c", "operation": "udp_echo", "input": {"sessionId": "0123456789abcdef"}, "reconnect": true, "controllerReply": {}, "expectedBody": {}}`,
			want: `reconnect is a hello sent again from a new address, and Echo is not the hello rpc`,
		},
		{
			name: "a gate that is neither admit nor refuse",
			body: `{"case": "c", "operation": "udp_hello", "input": {"sessionId": "0123456789abcdef"}, "gate": "maybe", "controllerReply": {}, "expectedBody": {}}`,
			want: `gate "maybe" is neither admit nor refuse`,
		},
		{
			name: "a dropped case with a reply",
			body: `{"case": "c", "operation": "udp_hello", "input": {"sessionId": "0123456789abcdef"}, "gate": "refuse", "expectDropped": true, "controllerReply": {}}`,
			want: `a dropped case expects no reply`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := vectorsrust.Generate([]byte(helloSpec), []byte(`{"cases": [`+tc.body+`]}`), sessionOptions())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("generating reported %v, want %q", err, tc.want)
			}
		})
	}
}

const cargoSessionControllerImpl = `use crate::udp::controller::{
    HelloDatagramController, HelloDatagramControllerError, HelloDatagramControllerImpl,
};
use crate::udp::types::context::Context;
use crate::udp::types::hello_datagram_messages::{Counter, Echo, Hello, Welcome};
use crate::udp::types::hello_datagram_push::HelloDatagramPush;

impl HelloDatagramController for HelloDatagramControllerImpl {
    fn hello(&self, request: Hello, context: &Context) -> Result<Welcome, HelloDatagramControllerError> {
        let _ = (request, context);

        Ok(Welcome {
            greeting: "welcome".to_string(),
        })
    }

    fn echo(&self, request: Echo, context: &Context) -> Result<Echo, HelloDatagramControllerError> {
        let _ = context;

        Ok(Echo {
            payload: request.payload,
        })
    }

    fn on_tick(&self, tick: u64) -> Result<(), HelloDatagramControllerError> {
        self.hello_datagram_broadcast
            .send_all(HelloDatagramPush::Counter(Counter { tick }))
            .map(|_| ())
            .map_err(|source| HelloDatagramControllerError::Broadcast {
                kind: "Counter".to_string(),
                source,
            })
    }
}
`

func buildSessionCargoWorkspace(t *testing.T) (string, string) {
	t.Helper()

	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Skip("cargo is not on PATH")
	}

	restFiles, err := restrust.Generate([]byte(cargoSpec), restrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating the rest cell: %v", err)
	}

	udpFiles, err := udprust.Generate([]byte(sessionProto), udprust.Options{Service: "songe-hello", Hello: "Hello", Push: []string{"Counter"}})
	if err != nil {
		t.Fatalf("generating the udp cell: %v", err)
	}

	restCases := cargoVectors[:strings.LastIndex(cargoVectors, "}\n  ]")+1]

	vectorFiles, err := vectorsrust.Generate([]byte(cargoSpec), []byte(restCases+","+sessionUdpCases+"\n  ]\n}"), sessionOptions())
	if err != nil {
		t.Fatalf("generating the vectors: %v", err)
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

	write("Cargo.toml", cargoCrateManifest)
	write("src/lib.rs", cargoCrateLib)

	for _, f := range restFiles {
		if strings.HasSuffix(f.Path, ".yaml") {
			continue
		}

		write(filepath.Join("src", "rest", f.Path), f.Content)
	}

	for _, f := range udpFiles {
		if strings.HasSuffix(f.Path, ".yaml") {
			continue
		}

		write(filepath.Join("src", "udp", f.Path), f.Content)
	}

	for _, f := range vectorFiles {
		write(f.Path, f.Content)
	}

	write("src/rest/controller/greeting_controller.rs", cargoGreetingControllerImpl)
	write("src/udp/controller/hello_datagram_controller.rs", cargoSessionControllerImpl)

	return root, cargo
}

func TestTheSessionVectorsPassAgainstTheGeneratedSessionCellAndAMockedGate(t *testing.T) {
	root, cargo := buildSessionCargoWorkspace(t)

	out, err := runCargoTest(t, root, cargo)
	if err != nil {
		skipOnNetworkError(t, err, out)
		t.Fatalf("cargo test: %v\n%s", err, out)
	}

	if !strings.Contains(string(out), "udp_counter_reaches_two_sessions_on_a_tick ... ok") {
		t.Fatalf("the push vector never ran\n%s", out)
	}
}
