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

package udprust_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/udprust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/cellmanifest"
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

func sessionOptions() udprust.Options {
	return udprust.Options{Service: "songe-hello", Hello: "Hello", Push: []string{"Counter"}}
}

func generateSession(t *testing.T) map[string]udprust.File {
	t.Helper()

	files, err := udprust.Generate([]byte(sessionProto), sessionOptions())
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	byPath := map[string]udprust.File{}
	for _, f := range files {
		byPath[f.Path] = f
	}

	return byPath
}

func TestNamingAHelloRpcEmitsTheGateTheBroadcastThePushTheAdmissionAndTheTickDriver(t *testing.T) {
	files, err := udprust.Generate([]byte(sessionProto), sessionOptions())
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	wantPaths := []string{
		"adapter/mod.rs",
		"adapter/zz_generated_hello_datagram_udp_broadcast.rs",
		"adapter/zz_generated_hello_datagram_udp_client.rs",
		"controller/mod.rs",
		"controller/zz_generated_hello_datagram_codec.rs",
		"controller/zz_generated_hello_datagram_controller.rs",
		"driver/mod.rs",
		"driver/zz_generated_hello_datagram_tick_driver.rs",
		"driver/zz_generated_hello_datagram_udp_driver.rs",
		"mod.rs",
		"port/mod.rs",
		"port/zz_generated_hello_datagram_broadcast.rs",
		"port/zz_generated_hello_datagram_client.rs",
		"port/zz_generated_hello_datagram_session_gate.rs",
		"types/mod.rs",
		"types/zz_generated_admission.rs",
		"types/zz_generated_context.rs",
		"types/zz_generated_hello_datagram_messages.rs",
		"types/zz_generated_hello_datagram_push.rs",
		"zz_generated_cell.yaml",
	}

	gotPaths := []string{}
	for _, f := range files {
		gotPaths = append(gotPaths, f.Path)
	}

	if !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Fatalf("emitted paths\n got %q\nwant %q", gotPaths, wantPaths)
	}
}

func TestTheSessionManifestListsTheGateAndTheBroadcastOnTheDriverTheBroadcastOnTheControllerAndTheTickDriver(t *testing.T) {
	files := generateSession(t)

	m, err := cellmanifest.Parse([]byte(files[cellmanifest.FileName].Content))
	if err != nil {
		t.Fatalf("parsing the manifest: %v", err)
	}

	if len(m.Provides.Drivers) != 2 {
		t.Fatalf("drivers = %+v", m.Provides.Drivers)
	}

	udp, tick := m.Provides.Drivers[0], m.Provides.Drivers[1]

	if udp.Name != "udp" || !reflect.DeepEqual(udp.Ports, []string{"HelloDatagramSessionGate", "HelloDatagramBroadcast"}) {
		t.Errorf("udp driver = %+v", udp)
	}

	if tick.Name != "tick" || tick.Type != "HelloDatagramTickDriver" || tick.Module != "udp::driver::hello_datagram_tick_driver" {
		t.Errorf("tick driver = %+v", tick)
	}

	if !reflect.DeepEqual(tick.Requires, []string{"HelloDatagramController"}) || fmt.Sprint(tick.Config["interval_ms"].Default) != "1000" {
		t.Errorf("tick driver = %+v", tick)
	}

	if !reflect.DeepEqual(m.Provides.Controllers[0].Ports, []string{"HelloDatagramBroadcast"}) {
		t.Errorf("controller = %+v", m.Provides.Controllers[0])
	}

	if len(m.Provides.Adapters) != 2 {
		t.Fatalf("adapters = %+v", m.Provides.Adapters)
	}

	broadcast := m.Provides.Adapters[1]
	if broadcast.Name != "udp_broadcast" || broadcast.Implements != "HelloDatagramBroadcast" || !broadcast.Fallible {
		t.Errorf("broadcast adapter = %+v", broadcast)
	}

	if fmt.Sprint(broadcast.Config["max_sessions"].Default) != "64" {
		t.Errorf("broadcast adapter config = %+v", broadcast.Config)
	}

	traits := []string{}
	for _, p := range m.Provides.Ports {
		traits = append(traits, p.Trait)
	}

	if !reflect.DeepEqual(traits, []string{"HelloDatagramClient", "HelloDatagramSessionGate", "HelloDatagramBroadcast"}) {
		t.Errorf("ports = %q", traits)
	}
}

func TestTheGatePortTakesTheSessionIdTheDecodedHelloAndThePeerAndAnswersAnAdmission(t *testing.T) {
	files := generateSession(t)

	gate := files["port/zz_generated_hello_datagram_session_gate.rs"].Content

	for _, want := range []string{
		"#[cfg_attr(test, mockall::automock)]",
		"pub trait HelloDatagramSessionGate: Send + Sync {",
		"session_id: &[u8; 16],",
		"request: &Hello,",
		"peer: std::net::SocketAddr,",
		") -> Result<Admission, HelloDatagramSessionGateError>;",
	} {
		if !strings.Contains(gate, want) {
			t.Errorf("the gate port lacks %q\n%s", want, gate)
		}
	}

	if strings.Contains(gate, "use std::net") {
		t.Errorf("the gate port names an io path on a use line\n%s", gate)
	}

	admission := files["types/zz_generated_admission.rs"].Content
	if !strings.Contains(admission, "Admitted,\n    Refused { reason: String },") {
		t.Errorf("the admission is not admitted or refused with a reason\n%s", admission)
	}
}

func TestTheDriverGatesTheHelloFollowsAKnownSessionAndDropsAnUnknownOne(t *testing.T) {
	files := generateSession(t)

	driver := files["driver/zz_generated_hello_datagram_udp_driver.rs"].Content

	for _, want := range []string{
		"session_gate: Arc<dyn HelloDatagramSessionGate + Send + Sync>,",
		"broadcast: Arc<dyn HelloDatagramBroadcast + Send + Sync>,",
		".attach(Box::new(move |datagram, peer| sender.try_send_to(datagram, peer)))",
		"codec::HelloDatagramRequest::Hello(request) => {\n                    if !self.admit(&session_id, &request, peer, &mut told_about_a_refusal, &mut told_about_a_full_table) {",
		"codec::HelloDatagramRequest::Echo(request) => {\n                    if !self.follow(&session_id, peer, &mut told_about_an_unknown_session) {",
		"Ok(Admission::Refused { reason }) => {",
		"the peer table is full",
		"session {session_id:?} was never admitted",
		"names a server to client rpc",
	} {
		if !strings.Contains(driver, want) {
			t.Errorf("the driver lacks %q\n%s", want, driver)
		}
	}

	if strings.Contains(driver, "HelloDatagramRequest::Counter") {
		t.Errorf("the driver routes a push rpc inbound\n%s", driver)
	}
}

func TestAPushRpcHasNoInboundMethodAndTheCodecEncodesItThroughThePushEnum(t *testing.T) {
	files := generateSession(t)

	controller := files["controller/zz_generated_hello_datagram_controller.rs"].Content
	codec := files["controller/zz_generated_hello_datagram_codec.rs"].Content
	push := files["types/zz_generated_hello_datagram_push.rs"].Content
	client := files["port/zz_generated_hello_datagram_client.rs"].Content

	if strings.Contains(controller, "fn counter(") || strings.Contains(client, "fn counter(") {
		t.Errorf("a push rpc got an inbound method")
	}

	for _, want := range []string{
		"fn on_tick(&self, tick: u64) -> Result<(), HelloDatagramControllerError>;",
		"pub(crate) hello_datagram_broadcast: Arc<dyn HelloDatagramBroadcast + Send + Sync>,",
		"pub fn new(hello_datagram_broadcast: Arc<dyn HelloDatagramBroadcast + Send + Sync>) -> Self {",
		"Broadcast {\n        kind: String,\n        #[source]\n        source: HelloDatagramBroadcastError,\n    },",
	} {
		if !strings.Contains(controller, want) {
			t.Errorf("the controller lacks %q\n%s", want, controller)
		}
	}

	for _, want := range []string{
		"pub const COUNTER_HASH: u8 =",
		"COUNTER_HASH => {\n            return Err(HelloDatagramCodecError::Outbound {",
		"pub fn encode_push(",
		"HelloDatagramPush::Counter(message) => seal(COUNTER_METHOD, session_id, COUNTER_HASH, message),",
		"pub fn decode_push(",
	} {
		if !strings.Contains(codec, want) {
			t.Errorf("the codec lacks %q\n%s", want, codec)
		}
	}

	if strings.Contains(codec, "encode_counter_request") {
		t.Errorf("the codec emits a request encoder for a push rpc\n%s", codec)
	}

	if !strings.Contains(push, "pub enum HelloDatagramPush {\n    Counter(Counter),\n}") {
		t.Errorf("the push enum lacks the counter variant\n%s", push)
	}
}

func TestTheBroadcastPortSendsToOneSessionOrToEveryOneAndTheAdapterHoldsThePeerTable(t *testing.T) {
	files := generateSession(t)

	port := files["port/zz_generated_hello_datagram_broadcast.rs"].Content
	adapter := files["adapter/zz_generated_hello_datagram_udp_broadcast.rs"].Content

	for _, want := range []string{
		"#[cfg_attr(test, mockall::automock)]",
		"fn send_to(&self, session_id: &[u8; 16], push: HelloDatagramPush) -> Result<(), HelloDatagramBroadcastError>;",
		"fn send_all(&self, push: HelloDatagramPush) -> Result<usize, HelloDatagramBroadcastError>;",
		"fn attach(&self, sender: HelloDatagramSender) -> Result<(), HelloDatagramBroadcastError>;",
	} {
		if !strings.Contains(port, want) {
			t.Errorf("the broadcast port lacks %q\n%s", want, port)
		}
	}

	for _, want := range []string{
		"pub struct HelloDatagramUdpBroadcast {",
		"peers: Mutex<Peers>,",
		"sender: OnceLock<HelloDatagramSender>,",
		"if peers.len() >= self.max_sessions && !peers.contains_key(session_id) {",
		"impl HelloDatagramBroadcast for HelloDatagramUdpBroadcast {",
		"codec::encode_push(session_id, push)",
	} {
		if !strings.Contains(adapter, want) {
			t.Errorf("the broadcast adapter lacks %q\n%s", want, adapter)
		}
	}
}

func TestTheTickDriverBindsAnIntervalAnnouncesItAndCallsOnTickWithACount(t *testing.T) {
	files := generateSession(t)

	tick := files["driver/zz_generated_hello_datagram_tick_driver.rs"].Content

	for _, want := range []string{
		"pub struct HelloDatagramTickDriverConfig {\n    pub interval_ms: i64,\n}",
		"pub fn new(config: HelloDatagramTickDriverConfig, controller: Arc<dyn HelloDatagramController + Send + Sync>) -> Self {",
		"pub async fn bind(&mut self) -> Result<(), HelloDatagramTickDriverError> {",
		"println!(\"TICKING {}\", self.interval_ms()?);",
		"pub async fn serve(self) -> Result<(), HelloDatagramTickDriverError> {",
		"if let Err(error) = self.controller.on_tick(tick) {",
	} {
		if !strings.Contains(tick, want) {
			t.Errorf("the tick driver lacks %q\n%s", want, tick)
		}
	}
}

func TestWithoutAHelloNothingOfTheSessionIsEmitted(t *testing.T) {
	files := generate(t, udprust.Options{Service: "songe-hello"})

	for path := range files {
		if strings.Contains(path, "gate") || strings.Contains(path, "broadcast") || strings.Contains(path, "tick") || strings.Contains(path, "push") || strings.Contains(path, "admission") {
			t.Errorf("a cell with no hello emitted %s", path)
		}
	}

	controller := files["controller/zz_generated_hello_datagram_controller.rs"].Content
	if strings.Contains(controller, "on_tick") {
		t.Errorf("a cell with no hello got an on_tick\n%s", controller)
	}
}

func TestAHelloOrAPushThatNamesNoRpcIsRefusedByName(t *testing.T) {
	tests := []struct {
		name string
		opts udprust.Options
		want string
	}{
		{
			name: "a hello that is not an rpc",
			opts: udprust.Options{Service: "songe-hello", Hello: "Greet"},
			want: `naming the hello rpc: "Greet" is not an rpc of package "songe.hello.udp.v1"`,
		},
		{
			name: "a push that is not an rpc",
			opts: udprust.Options{Service: "songe-hello", Hello: "Hello", Push: []string{"State"}},
			want: `naming the push rpcs: "State" is not an rpc of package "songe.hello.udp.v1"`,
		},
		{
			name: "a push with no hello",
			opts: udprust.Options{Service: "songe-hello", Push: []string{"Counter"}},
			want: `naming the push rpcs: Counter pushed and no layout.hello named`,
		},
		{
			name: "the hello pushed",
			opts: udprust.Options{Service: "songe-hello", Hello: "Hello", Push: []string{"Hello"}},
			want: `naming the push rpcs: "Hello" is the hello rpc`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := udprust.Generate([]byte(sessionProto), tc.opts)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("generating reported %v, want %q", err, tc.want)
			}
		})
	}
}
