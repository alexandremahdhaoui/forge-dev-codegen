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

func TestNamingAHelloRpcEmitsTheGateThePeerTableTheBroadcastThePushTheAdmissionAndTheTickDriver(t *testing.T) {
	files, err := udprust.Generate([]byte(sessionProto), sessionOptions())
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	wantPaths := []string{
		"adapter/mod.rs",
		"adapter/zz_generated_hello_datagram_udp_broadcast.rs",
		"adapter/zz_generated_hello_datagram_udp_client.rs",
		"adapter/zz_generated_hello_datagram_udp_peer_table.rs",
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
		"port/zz_generated_hello_datagram_peer_table.rs",
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

func TestTheSessionManifestHandsTheGateAndThePeerTableToTheDriversTheBroadcastToTheControllerAndThePeerTableToTheBroadcastAdapter(t *testing.T) {
	files := generateSession(t)

	m, err := cellmanifest.Parse([]byte(files[cellmanifest.FileName].Content))
	if err != nil {
		t.Fatalf("parsing the manifest: %v", err)
	}

	if len(m.Provides.Drivers) != 2 {
		t.Fatalf("drivers = %+v", m.Provides.Drivers)
	}

	udp, tick := m.Provides.Drivers[0], m.Provides.Drivers[1]

	if udp.Name != "udp" || !reflect.DeepEqual(udp.Ports, []string{"HelloDatagramSessionGate", "HelloDatagramPeerTable"}) {
		t.Errorf("udp driver = %+v", udp)
	}

	if tick.Name != "tick" || tick.Type != "HelloDatagramTickDriver" || !reflect.DeepEqual(tick.Ports, []string{"HelloDatagramPeerTable"}) {
		t.Errorf("tick driver = %+v", tick)
	}

	if !reflect.DeepEqual(tick.Requires, []string{"HelloDatagramController"}) || fmt.Sprint(tick.Config["interval_ms"].Default) != "1000" {
		t.Errorf("tick driver = %+v", tick)
	}

	if !reflect.DeepEqual(m.Provides.Controllers[0].Ports, []string{"HelloDatagramBroadcast"}) {
		t.Errorf("controller = %+v", m.Provides.Controllers[0])
	}

	if len(m.Provides.Adapters) != 3 {
		t.Fatalf("adapters = %+v", m.Provides.Adapters)
	}

	peerTable, broadcast := m.Provides.Adapters[1], m.Provides.Adapters[2]

	if peerTable.Name != "udp_peer_table" || peerTable.Implements != "HelloDatagramPeerTable" || !peerTable.Fallible {
		t.Errorf("peer table adapter = %+v", peerTable)
	}

	if fmt.Sprint(peerTable.Config["max_sessions"].Default) != "64" {
		t.Errorf("peer table adapter config = %+v", peerTable.Config)
	}

	if broadcast.Name != "udp_broadcast" || broadcast.Implements != "HelloDatagramBroadcast" || broadcast.Fallible {
		t.Errorf("broadcast adapter = %+v", broadcast)
	}

	if !reflect.DeepEqual(broadcast.Ports, []string{"HelloDatagramPeerTable"}) || len(broadcast.Config) != 0 {
		t.Errorf("broadcast adapter = %+v", broadcast)
	}

	traits := []string{}
	for _, p := range m.Provides.Ports {
		traits = append(traits, p.Trait)
	}

	if !reflect.DeepEqual(traits, []string{"HelloDatagramClient", "HelloDatagramSessionGate", "HelloDatagramBroadcast", "HelloDatagramPeerTable"}) {
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

func TestTheDriverGatesTheHelloAdmitsThePeerAfterTheControllerAnsweredFollowsAKnownSessionAndDropsAnUnknownOne(t *testing.T) {
	files := generateSession(t)

	driver := files["driver/zz_generated_hello_datagram_udp_driver.rs"].Content

	for _, want := range []string{
		"session_gate: Arc<dyn HelloDatagramSessionGate + Send + Sync>,",
		"peer_table: Arc<dyn HelloDatagramPeerTable + Send + Sync>,",
		".attach(Box::new(move |datagram, peer| sender.try_send_to(datagram, peer)))",
		"codec::HelloDatagramRequest::Hello(request) => {\n                    if !self.gate(&session_id, &request, peer, &mut told_about_a_refusal) {",
		"match self.controller.hello(request, &context) {\n                        Ok(reply) => {\n                            if !self.admit(&session_id, peer, &mut told_about_a_full_table) {",
		"codec::HelloDatagramRequest::Echo(request) => {\n                    if !self.follow(&session_id, peer, &mut told_about_an_unknown_session) {",
		"Ok(Admission::Refused { reason }) => {",
		"session {session_id:?} was never admitted",
		"names a server to client rpc",
	} {
		if !strings.Contains(driver, want) {
			t.Errorf("the driver lacks %q\n%s", want, driver)
		}
	}

	if strings.Contains(driver, "HelloDatagramRequest::Counter") || strings.Contains(driver, "Broadcast") {
		t.Errorf("the driver routes a push rpc inbound or reaches the broadcast\n%s", driver)
	}
}

func TestAFullPeerTableRefusesByNamingMaxSessionsAndNeverAnswersABareFalse(t *testing.T) {
	files := generateSession(t)

	port := files["port/zz_generated_hello_datagram_peer_table.rs"].Content
	adapter := files["adapter/zz_generated_hello_datagram_udp_peer_table.rs"].Content
	driver := files["driver/zz_generated_hello_datagram_udp_driver.rs"].Content

	for _, want := range []string{
		"the hello_datagram peer table holds its max_sessions of {max_sessions} and this session is a new one",
		"Full {\n        max_sessions: usize,\n        session_id: [u8; 16],\n        peer: std::net::SocketAddr,\n    },",
		"    ) -> Result<(), HelloDatagramPeerTableError>;",
	} {
		if !strings.Contains(port, want) {
			t.Errorf("the peer table port lacks %q\n%s", want, port)
		}
	}

	if !strings.Contains(adapter, "return Err(HelloDatagramPeerTableError::Full {") {
		t.Errorf("the peer table adapter answers a bare false when full\n%s", adapter)
	}

	if !strings.Contains(driver, "match self.peer_table.admit_peer(session_id, peer) {\n            Ok(()) => true,") {
		t.Errorf("the driver still reads an admission as a bool\n%s", driver)
	}

	if !strings.Contains(driver, "if full.first_time(peer) {\n                    eprintln!(\"dropping a hello from {peer}: {}\", error_chain(&error));") {
		t.Errorf("the driver never renders the refusal the peer table names\n%s", driver)
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
		"fn on_tick(&self) -> Result<(), HelloDatagramControllerError>;",
		"pub(crate) hello_datagram_broadcast: Arc<dyn HelloDatagramBroadcast + Send + Sync>,",
		"pub fn new(\n        hello_datagram_broadcast: Arc<dyn HelloDatagramBroadcast + Send + Sync>,\n    ) -> Self {",
		"Broadcast {\n        kind: String,\n        #[source]\n        source: HelloDatagramBroadcastError,\n    },",
	} {
		if !strings.Contains(controller, want) {
			t.Errorf("the controller lacks %q\n%s", want, controller)
		}
	}

	for _, want := range []string{
		"use crate::udp::types::hello_datagram_messages::{Hello, Welcome, Echo, Counter};",
		"pub const COUNTER_HASH: u8 =",
		"COUNTER_HASH => {\n            return Err(HelloDatagramCodecError::Outbound {",
		"pub fn encode_push(",
		"HelloDatagramPush::Counter(message) => seal(COUNTER_METHOD, session_id, COUNTER_HASH, message),",
		"pub fn decode_push(",
		"COUNTER_HASH => Counter::decode(framed.payload)\n            .map(HelloDatagramPush::Counter)",
	} {
		if !strings.Contains(codec, want) {
			t.Errorf("the codec lacks %q\n%s", want, codec)
		}
	}

	if strings.Contains(codec, "encode_counter_request") || strings.Count(codec[strings.Index(codec, "pub fn decode_push("):], "unframe(datagram)") != 1 {
		t.Errorf("the codec emits a request encoder for a push rpc or unframes a push twice\n%s", codec)
	}

	if !strings.Contains(push, "pub enum HelloDatagramPush {\n    Counter(Counter),\n}") {
		t.Errorf("the push enum lacks the counter variant\n%s", push)
	}
}

func TestTheBroadcastPortOnlySendsAndThePeerTablePortHoldsTheSocketAndThePeers(t *testing.T) {
	files := generateSession(t)

	broadcast := files["port/zz_generated_hello_datagram_broadcast.rs"].Content
	peerTable := files["port/zz_generated_hello_datagram_peer_table.rs"].Content

	for _, want := range []string{
		"#[cfg_attr(test, mockall::automock)]",
		"fn send_to(&self, session_id: &[u8; 16], push: HelloDatagramPush) -> Result<(), HelloDatagramBroadcastError>;",
		"fn send_all(&self, push: HelloDatagramPush) -> Result<usize, HelloDatagramBroadcastError>;",
	} {
		if !strings.Contains(broadcast, want) {
			t.Errorf("the broadcast port lacks %q\n%s", want, broadcast)
		}
	}

	for _, unwanted := range []string{"fn attach(", "fn admit_peer(", "fn follow_peer("} {
		if strings.Contains(broadcast, unwanted) {
			t.Errorf("the broadcast port carries %q, the controller must not see it\n%s", unwanted, broadcast)
		}
	}

	for _, want := range []string{
		"#[cfg_attr(test, mockall::automock)]",
		"fn attach(&self, sender: HelloDatagramSender) -> Result<(), HelloDatagramPeerTableError>;",
		"fn attached(&self) -> bool;",
		"fn admit_peer(",
		"fn follow_peer(",
		"fn peer_of(",
		"fn peers(&self) -> Result<Vec<([u8; 16], std::net::SocketAddr)>, HelloDatagramPeerTableError>;",
		"fn deliver(&self, datagram: &[u8], peer: std::net::SocketAddr) -> Result<(), HelloDatagramPeerTableError>;",
	} {
		if !strings.Contains(peerTable, want) {
			t.Errorf("the peer table port lacks %q\n%s", want, peerTable)
		}
	}
}

func TestOneAdapterHoldsThePeerTableAndAnotherEncodesThePushThroughIt(t *testing.T) {
	files := generateSession(t)

	peerTable := files["adapter/zz_generated_hello_datagram_udp_peer_table.rs"].Content
	broadcast := files["adapter/zz_generated_hello_datagram_udp_broadcast.rs"].Content

	for _, want := range []string{
		"pub struct HelloDatagramUdpPeerTable {",
		"peers: Mutex<Peers>,",
		"sender: OnceLock<HelloDatagramSender>,",
		"if peers.len() >= self.max_sessions && !peers.contains_key(session_id) {",
		"impl HelloDatagramPeerTable for HelloDatagramUdpPeerTable {",
	} {
		if !strings.Contains(peerTable, want) {
			t.Errorf("the peer table adapter lacks %q\n%s", want, peerTable)
		}
	}

	for _, want := range []string{
		"pub struct HelloDatagramUdpBroadcastConfig {}",
		"peer_table: Arc<dyn HelloDatagramPeerTable + Send + Sync>,",
		"pub fn new(config: HelloDatagramUdpBroadcastConfig, peer_table: Arc<dyn HelloDatagramPeerTable + Send + Sync>) -> Self {",
		"impl HelloDatagramBroadcast for HelloDatagramUdpBroadcast {",
		"codec::encode_push(session_id, push)",
		".deliver(&datagram, peer)",
	} {
		if !strings.Contains(broadcast, want) {
			t.Errorf("the broadcast adapter lacks %q\n%s", want, broadcast)
		}
	}
}

func TestTheTickDriverTakesThePeerTableRefusesToServeUnattachedAndCallsOnTickWithNoArgument(t *testing.T) {
	files := generateSession(t)

	tick := files["driver/zz_generated_hello_datagram_tick_driver.rs"].Content

	for _, want := range []string{
		"pub struct HelloDatagramTickDriverConfig {\n    pub interval_ms: i64,\n}",
		"peer_table: Arc<dyn HelloDatagramPeerTable + Send + Sync>,",
		"pub async fn bind(&mut self) -> Result<(), HelloDatagramTickDriverError> {",
		"println!(\"TICKING {}\", self.interval_ms()?);",
		"pub async fn serve(self) -> Result<(), HelloDatagramTickDriverError> {",
		"const ATTACH_WAIT_INTERVALS: u32 = 10;",
		"self.wait_for_the_socket(interval).await?;",
		"for _ in 0..ATTACH_WAIT_INTERVALS {\n            if self.peer_table.attached() {\n                return Ok(());",
		"Err(HelloDatagramTickDriverError::NotAttached {\n            waited: ATTACH_WAIT_INTERVALS,",
		"enable driver_udp so the udp driver attaches one",
		"if let Err(error) = self.controller.on_tick() {\n                eprintln!(\n                    \"skipping a tick of the hello_datagram tick driver: {}\",\n                    error_chain(&error)",
	} {
		if !strings.Contains(tick, want) {
			t.Errorf("the tick driver lacks %q\n%s", want, tick)
		}
	}

	for _, unwanted := range []string{"on_tick(tick)", "wrapping_add", "OnTick {"} {
		if strings.Contains(tick, unwanted) {
			t.Errorf("the tick driver still carries %q\n%s", unwanted, tick)
		}
	}
}

func counterPort() udprust.PortSpec {
	adapters := []string{udprust.CounterAdapterMemory}

	return udprust.PortSpec{Name: "TickCounter", Kind: udprust.CounterPortKind, Adapters: &adapters}
}

func TestACounterPortKindIsEmittedAsATraitHeldByTheControllerWithItsMemoryAdapter(t *testing.T) {
	opts := sessionOptions()
	opts.Ports = []udprust.PortSpec{counterPort()}

	files, err := udprust.Generate([]byte(sessionProto), opts)
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	byPath := map[string]string{}
	for _, f := range files {
		byPath[f.Path] = f.Content
	}

	stub := byPath["port/zz_generated_tick_counter.rs"]
	if !strings.Contains(stub, "#[cfg_attr(test, mockall::automock)]\npub trait TickCounter: Send + Sync {\n    fn next(&self) -> u64;\n}") {
		t.Errorf("the port stub is not the trait the layout named\n%s", stub)
	}

	if !strings.Contains(byPath["port/mod.rs"], "pub use zz_generated_tick_counter as tick_counter;") {
		t.Errorf("the port layer never aliased the stub\n%s", byPath["port/mod.rs"])
	}

	controller := byPath["controller/zz_generated_hello_datagram_controller.rs"]

	for _, want := range []string{
		"use crate::udp::port::tick_counter::TickCounter;",
		"pub(crate) hello_datagram_broadcast: Arc<dyn HelloDatagramBroadcast + Send + Sync>,\n    pub(crate) tick_counter: Arc<dyn TickCounter + Send + Sync>,",
		"hello_datagram_broadcast: Arc<dyn HelloDatagramBroadcast + Send + Sync>,\n        tick_counter: Arc<dyn TickCounter + Send + Sync>,\n    ) -> Self {",
	} {
		if !strings.Contains(controller, want) {
			t.Errorf("the controller lacks %q\n%s", want, controller)
		}
	}

	m, err := cellmanifest.Parse([]byte(byPath[cellmanifest.FileName]))
	if err != nil {
		t.Fatalf("parsing the manifest: %v", err)
	}

	if !reflect.DeepEqual(m.Provides.Controllers[0].Ports, []string{"HelloDatagramBroadcast", "TickCounter"}) {
		t.Errorf("controller ports = %q", m.Provides.Controllers[0].Ports)
	}

	for _, required := range m.Requires.Ports {
		if required == "TickCounter" {
			t.Error("the cell provides the counter adapter and still requires the port")
		}
	}

	counter := byPath["adapter/zz_generated_tick_counter_memory.rs"]

	for _, want := range []string{"AtomicU64", "fn next(&self) -> u64 {", "fetch_add(1, Ordering::Relaxed) + 1"} {
		if !strings.Contains(counter, want) {
			t.Errorf("the memory counter lacks %q\n%s", want, counter)
		}
	}

	declared := false
	for _, port := range m.Provides.Ports {
		declared = declared || (port.Trait == "TickCounter" && port.Module == "udp::port::tick_counter")
	}

	if !declared {
		t.Errorf("the manifest never declared the port: %+v", m.Provides.Ports)
	}
}

func TestANamedControllerPortWithoutMethodsMountsTheUsersOwnPortFile(t *testing.T) {
	opts := udprust.Options{Service: "songe-hello", Ports: []udprust.PortSpec{{Name: "Clock"}}}

	files, err := udprust.Generate([]byte(sessionProto), opts)
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	byPath := map[string]string{}
	for _, f := range files {
		byPath[f.Path] = f.Content
	}

	if _, emitted := byPath["port/zz_generated_clock.rs"]; emitted {
		t.Errorf("a port with no methods got a stub")
	}

	if !strings.Contains(byPath["port/mod.rs"], "\npub mod clock;") {
		t.Errorf("the port layer never mounted the user's file\n%s", byPath["port/mod.rs"])
	}

	controller := byPath["controller/zz_generated_hello_datagram_controller.rs"]
	if !strings.Contains(controller, "pub(crate) clock: Arc<dyn Clock + Send + Sync>,") || strings.Contains(controller, "impl Default") {
		t.Errorf("the controller lacks the port or still derives Default\n%s", controller)
	}
}

func TestAControllerPortThatIsNotAPascalIdentOrAMethodThatIsNotASignatureIsRefused(t *testing.T) {
	for _, tc := range []struct {
		name string
		spec udprust.PortSpec
		want string
	}{
		{name: "a snake name", spec: udprust.PortSpec{Name: "tick_counter"}, want: `"tick_counter" is not a Pascal case Rust ident`},
		{name: "an unknown kind", spec: udprust.PortSpec{Name: "TickCounter", Kind: "dice"}, want: `is of kind "dice", the declared kinds are counter`},
		{name: "a kind with no adapters", spec: udprust.PortSpec{Name: "TickCounter", Kind: udprust.CounterPortKind}, want: "names no adapters"},
		{name: "a kind with an empty adapters list", spec: udprust.PortSpec{Name: "TickCounter", Kind: udprust.CounterPortKind, Adapters: &[]string{}}, want: "names an empty adapters list"},
		{name: "an unknown adapter kind", spec: udprust.PortSpec{Name: "TickCounter", Kind: udprust.CounterPortKind, Adapters: &[]string{"clock"}}, want: `adapter kind "clock", a counter adapter is one of memory`},
		{name: "adapters with no kind", spec: udprust.PortSpec{Name: "TickCounter", Adapters: &[]string{"memory"}}, want: "names adapters and no kind"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := udprust.Generate([]byte(sessionProto), udprust.Options{Service: "songe-hello", Ports: []udprust.PortSpec{tc.spec}})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("generating reported %v, want %q", err, tc.want)
			}
		})
	}
}

func TestWithoutAHelloNothingOfTheSessionIsEmitted(t *testing.T) {
	files := generate(t, udprust.Options{Service: "songe-hello"})

	for path := range files {
		for _, word := range []string{"gate", "broadcast", "tick", "push", "admission", "peer_table"} {
			if strings.Contains(path, word) {
				t.Errorf("a cell with no hello emitted %s", path)
			}
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

func secretGate() udprust.GateSpec {
	adapters := []string{udprust.GateAdapterSecret}

	return udprust.GateSpec{Field: "secret", Adapters: &adapters}
}

func TestASecretGateAdapterComparesTheDeclaredFieldAndIsProvidedByTheCell(t *testing.T) {
	opts := sessionOptions()
	opts.Gate = secretGate()

	files, err := udprust.Generate([]byte(sessionProto), opts)
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	byPath := map[string]string{}
	for _, f := range files {
		byPath[f.Path] = f.Content
	}

	gate := byPath["adapter/zz_generated_hello_datagram_session_gate_secret.rs"]

	for _, want := range []string{
		"impl HelloDatagramSessionGate for HelloDatagramSessionGateSecret {",
		"if request.secret == self.secret {",
		"return Ok(Admission::Admitted);",
		"Ok(Admission::Refused {",
	} {
		if !strings.Contains(gate, want) {
			t.Errorf("the secret gate lacks %q\n%s", want, gate)
		}
	}

	m, err := cellmanifest.Parse([]byte(byPath[cellmanifest.FileName]))
	if err != nil {
		t.Fatalf("parsing the manifest: %v", err)
	}

	provided := false
	for _, adapter := range m.Provides.Adapters {
		provided = provided || (adapter.Name == "hello_datagram_session_gate_secret" && adapter.Implements == "HelloDatagramSessionGate")
	}

	if !provided {
		t.Errorf("the manifest never provided the secret gate: %+v", m.Provides.Adapters)
	}
}

func TestAGateDeclarationIsRefusedWhenItBreaksTheContract(t *testing.T) {
	empty := []string{}
	unknown := []string{"oauth"}
	secret := []string{udprust.GateAdapterSecret}

	for _, tc := range []struct {
		name string
		gate udprust.GateSpec
		want string
	}{
		{name: "a field with no adapters", gate: udprust.GateSpec{Field: "secret"}, want: "names field \"secret\" and no adapters"},
		{name: "an empty adapters list", gate: udprust.GateSpec{Adapters: &empty}, want: "the adapters list is empty"},
		{name: "an unknown adapter kind", gate: udprust.GateSpec{Field: "secret", Adapters: &unknown}, want: `adapter kind "oauth", a gate adapter is one of secret`},
		{name: "no field", gate: udprust.GateSpec{Adapters: &secret}, want: "names no field"},
		{name: "a field the hello never declares", gate: udprust.GateSpec{Field: "password", Adapters: &secret}, want: `compares field "password", which Hello does not declare, it declares secret`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := sessionOptions()
			opts.Gate = tc.gate

			_, err := udprust.Generate([]byte(sessionProto), opts)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("wanted a refusal naming %q, got %v", tc.want, err)
			}
		})
	}
}
