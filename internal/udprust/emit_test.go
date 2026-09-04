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
	"reflect"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/grpcrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/udprust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/cellmanifest"
)

const helloProto = `syntax = "proto3";

package songe.hello.udp.v1;

service HelloDatagram {
  rpc Echo(Echo) returns (Echo);
  rpc Note(Note) returns (Nothing);
}

message Echo {
  string payload = 1;
  uint64 count = 2;
}

message Note {
  string text = 1;
}

message Nothing {}
`

func generate(t *testing.T, opts udprust.Options) map[string]udprust.File {
	t.Helper()

	files, err := udprust.Generate([]byte(helloProto), opts)
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	byPath := map[string]udprust.File{}
	for _, f := range files {
		byPath[f.Path] = f
	}

	return byPath
}

func TestGeneratingTheUdpProtoEmitsTheWholeFileSet(t *testing.T) {
	files, err := udprust.Generate([]byte(helloProto), udprust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	wantPaths := []string{
		"adapter/mod.rs",
		"adapter/zz_generated_hello_datagram_udp_client.rs",
		"controller/mod.rs",
		"controller/zz_generated_hello_datagram_codec.rs",
		"controller/zz_generated_hello_datagram_controller.rs",
		"driver/mod.rs",
		"driver/zz_generated_hello_datagram_udp_driver.rs",
		"mod.rs",
		"port/mod.rs",
		"port/zz_generated_hello_datagram_client.rs",
		"types/mod.rs",
		"types/zz_generated_context.rs",
		"types/zz_generated_hello_datagram_messages.rs",
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

func TestTheCellHoldsNoHandDirectoryAndNoHandCall(t *testing.T) {
	files := generate(t, udprust.Options{Service: "songe-hello"})

	for path, file := range files {
		if strings.Contains(path, "hand") {
			t.Fatalf("the cell emitted %q", path)
		}

		if strings.Contains(file.Content, "::hand::") {
			t.Fatalf("%s still reaches for a hand module:\n%s", path, file.Content)
		}
	}
}

func TestTheCellManifestNamesTheDriverTheAdapterTheControllerAndThePort(t *testing.T) {
	files := generate(t, udprust.Options{Service: "songe-hello"})

	body := files[cellmanifest.FileName].Content
	if body == "" {
		t.Fatal("no cell manifest was emitted")
	}

	m, err := cellmanifest.Parse([]byte(body))
	if err != nil {
		t.Fatalf("parsing the manifest: %v\n%s", err, body)
	}

	if m.Cell != "udp" {
		t.Errorf("cell = %q, want udp", m.Cell)
	}

	if len(m.Provides.Drivers) != 1 {
		t.Fatalf("drivers = %+v", m.Provides.Drivers)
	}

	driver := m.Provides.Drivers[0]
	if driver.Name != "udp" || driver.Type != "HelloDatagramUdpDriver" || driver.Module != "udp::driver::hello_datagram_udp_driver" {
		t.Errorf("driver = %+v", driver)
	}

	if !reflect.DeepEqual(driver.Requires, []string{"HelloDatagramController"}) {
		t.Errorf("driver requires = %+v", driver.Requires)
	}

	if len(m.Provides.Adapters) != 1 {
		t.Fatalf("adapters = %+v", m.Provides.Adapters)
	}

	adapter := m.Provides.Adapters[0]
	if adapter.Name != "udp_client" || adapter.Implements != "HelloDatagramClient" {
		t.Errorf("adapter = %+v", adapter)
	}

	if adapter.Config["timeout_ms"].Type != cellmanifest.FieldTypeDuration {
		t.Errorf("adapter config = %+v", adapter.Config)
	}

	if len(m.Provides.Controllers) != 1 {
		t.Fatalf("controllers = %+v", m.Provides.Controllers)
	}

	controller := m.Provides.Controllers[0]
	if controller.Trait != "HelloDatagramController" || controller.Impl != "HelloDatagramControllerImpl" || controller.Module != "udp::controller" {
		t.Errorf("controller = %+v", controller)
	}
}

func TestNoOneofEnvelopeIsEmittedAnywhere(t *testing.T) {
	files := generate(t, udprust.Options{Service: "songe-hello"})

	for path, file := range files {
		if strings.Contains(file.Content, "Envelope") || strings.Contains(file.Content, "oneof") {
			t.Fatalf("the cell reached for an envelope in %s:\n%s", path, file.Content)
		}
	}
}

func TestTheSchemaVersionIsTheNumberInThePackageVersionSuffix(t *testing.T) {
	files := generate(t, udprust.Options{Service: "songe-hello"})

	codec := files["controller/zz_generated_hello_datagram_codec.rs"].Content

	if !strings.Contains(codec, "pub const SCHEMA_VERSION: u8 = 1;") {
		t.Fatalf("the codec never read the version out of the package:\n%s", codec)
	}
}

func TestAPackageWithNoVersionSuffixIsRefused(t *testing.T) {
	const unversioned = `syntax = "proto3";

package songe.hello.udp;

service HelloDatagram {
  rpc Echo(Echo) returns (Echo);
}

message Echo {
  string payload = 1;
}
`

	_, err := udprust.Generate([]byte(unversioned), udprust.Options{Service: "songe-hello"})
	if err == nil {
		t.Fatal("a package with no version suffix was accepted")
	}

	if !strings.Contains(err.Error(), "the last segment must be a version like v1") {
		t.Fatalf("generating reported %q", err)
	}
}

func TestTheFunctionHashIsFnv1aOverTheFullMethodNameFoldedToEightBits(t *testing.T) {
	if got := udprust.FunctionHash("songe.hello.udp.v1.HelloDatagram/Echo"); got != 223 {
		t.Fatalf("the echo method folds to %d, want 223", got)
	}

	if got := udprust.FunctionHash("songe.hello.udp.v1.HelloDatagram/Note"); got != 128 {
		t.Fatalf("the note method folds to %d, want 128", got)
	}

	files := generate(t, udprust.Options{Service: "songe-hello"})

	codec := files["controller/zz_generated_hello_datagram_codec.rs"].Content

	for _, want := range []string{
		"pub const ECHO_METHOD: &str = \"songe.hello.udp.v1.HelloDatagram/Echo\";",
		"pub const ECHO_HASH: u8 = 223;",
		"pub const NOTE_METHOD: &str = \"songe.hello.udp.v1.HelloDatagram/Note\";",
		"pub const NOTE_HASH: u8 = 128;",
	} {
		if !strings.Contains(codec, want) {
			t.Fatalf("the codec never carried %q:\n%s", want, codec)
		}
	}
}

func TestTwoMethodsThatFoldToTheSameFunctionHashAreRefusedByName(t *testing.T) {
	const first = "songe.collision.v1.Service/Alpha"
	const collides = "Baqt"

	if udprust.FunctionHash(first) != udprust.FunctionHash("songe.collision.v1.Service/"+collides) {
		t.Fatalf("%s and %s no longer fold to the same byte", first, collides)
	}

	doc := "syntax = \"proto3\";\n\npackage songe.collision.v1;\n\nservice Service {\n  rpc Alpha(Ping) returns (Ping);\n  rpc " + collides + "(Ping) returns (Ping);\n}\n\nmessage Ping {\n  string text = 1;\n}\n"

	_, err := udprust.Generate([]byte(doc), udprust.Options{Service: "songe-hello"})
	if err == nil {
		t.Fatal("two colliding methods were accepted")
	}

	for _, want := range []string{"songe.collision.v1.Service/Alpha", "songe.collision.v1.Service/" + collides, "fold to the function hash"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the refusal %q never named %q", err, want)
		}
	}
}

func TestTwoServicesInOneProtoFileWhoseRpcsFoldToTheSameFunctionHashAreRefusedByName(t *testing.T) {
	const first = "songe.collision.v1.Alpha/Ping"
	const second = "songe.collision.v1.Beta/AabE"

	if udprust.FunctionHash(first) != udprust.FunctionHash(second) {
		t.Fatalf("%s and %s no longer fold to the same byte", first, second)
	}

	doc := "syntax = \"proto3\";\n\npackage songe.collision.v1;\n\nservice Alpha {\n  rpc Ping(Ping) returns (Ping);\n}\n\nservice Beta {\n  rpc AabE(Ping) returns (Ping);\n}\n\nmessage Ping {\n  string text = 1;\n}\n"

	_, err := udprust.Generate([]byte(doc), udprust.Options{Service: "songe-hello"})
	if err == nil {
		t.Fatal("two services whose rpcs collide were accepted")
	}

	for _, want := range []string{first, second, "fold to the function hash"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the refusal %q never named %q", err, want)
		}
	}
}

func TestTheCodecRefusesAMissingMagicAShortDatagramAnOversizeOneAWrongVersionAndAnUnknownMethod(t *testing.T) {
	files := generate(t, udprust.Options{Service: "songe-hello"})

	codec := files["controller/zz_generated_hello_datagram_codec.rs"].Content

	for _, want := range []string{
		"it does not open with the udplb magic",
		"a datagram opens with a magic, a session id, a schema version and a function hash",
		"a datagram carries at most {MAX_DATAGRAM_LEN} bytes",
		"it speaks schema version {got}, this build speaks {SCHEMA_VERSION}",
		"function hash {hash} names no rpc of HelloDatagram",
		"pub const MAX_DATAGRAM_LEN: usize = 508;",
		"pub const MAX_PAYLOAD_LEN: usize = MAX_DATAGRAM_LEN - HEADER_LEN;",
	} {
		if !strings.Contains(codec, want) {
			t.Fatalf("the codec never carried %q:\n%s", want, codec)
		}
	}
}

func TestAControllerMethodTakesTheRequestAndTheSessionContext(t *testing.T) {
	files := generate(t, udprust.Options{Service: "songe-hello"})

	controller := files["controller/zz_generated_hello_datagram_controller.rs"].Content
	context := files["types/zz_generated_context.rs"].Content

	for _, want := range []string{
		"pub trait HelloDatagramController: Send + Sync {",
		"request: Echo,\n        context: &Context,\n    ) -> Result<Echo, HelloDatagramControllerError>;",
		"request: Note,\n        context: &Context,\n    ) -> Result<Nothing, HelloDatagramControllerError>;",
		"pub struct HelloDatagramControllerImpl;",
	} {
		if !strings.Contains(controller, want) {
			t.Fatalf("the controller never carried %q:\n%s", want, controller)
		}
	}

	for _, want := range []string{
		"pub session_id: [u8; 16],",
		"pub peer: std::net::SocketAddr,",
	} {
		if !strings.Contains(context, want) {
			t.Fatalf("the context never carried %q:\n%s", want, context)
		}
	}
}

func TestAnRpcReplyingNothingIsNeverAnsweredOnTheWire(t *testing.T) {
	files := generate(t, udprust.Options{Service: "songe-hello"})

	driver := files["driver/zz_generated_hello_datagram_udp_driver.rs"].Content
	adapter := files["adapter/zz_generated_hello_datagram_udp_client.rs"].Content

	if !strings.Contains(driver, "codec::HelloDatagramRequest::Note(request) => {\n                    match self.controller.note(request, &context) {\n                        Ok(_) => None,") {
		t.Fatalf("the driver answered a Nothing rpc:\n%s", driver)
	}

	if !strings.Contains(driver, "Ok(reply) => match codec::encode_echo_reply(&session_id, &reply) {") {
		t.Fatalf("the driver never answered the echo rpc:\n%s", driver)
	}

	if !strings.Contains(adapter, "Ok(Nothing::default())") {
		t.Fatalf("the client waited for an answer to a Nothing rpc:\n%s", adapter)
	}
}

func TestTheGeneratedClientTakesItsConfigConnectsItsSocketAndRefusesAnotherSessionId(t *testing.T) {
	files := generate(t, udprust.Options{Service: "songe-hello"})

	adapter := files["adapter/zz_generated_hello_datagram_udp_client.rs"].Content
	codec := files["controller/zz_generated_hello_datagram_codec.rs"].Content

	for _, want := range []string{
		"pub struct HelloDatagramUdpClientConfig {",
		"    pub address: String,",
		"    pub session_id: String,",
		"    pub timeout_ms: i64,",
		"    pub fn new(config: HelloDatagramUdpClientConfig) -> Self {",
		".connect(&self.address)",
		"socket.send(&datagram).await.map_err(failing_echo)?;",
		"codec::decode_echo_reply(&self.session_id, &buffer[..read])",
	} {
		if !strings.Contains(adapter, want) {
			t.Fatalf("the client never carried %q:\n%s", want, adapter)
		}
	}

	for _, want := range []string{
		"UnknownSession {",
		"if framed.session_id != *session_id {",
		"if payload.len() > MAX_PAYLOAD_LEN {",
		"pub fn session_id_from(text: &str) -> [u8; SESSION_ID_LEN] {",
	} {
		if !strings.Contains(codec, want) {
			t.Fatalf("the codec never carried %q:\n%s", want, codec)
		}
	}
}

func TestTheDriverAnswersTheHealthProbeBeforeItDecodesAnything(t *testing.T) {
	files := generate(t, udprust.Options{Service: "songe-hello"})

	driver := files["driver/zz_generated_hello_datagram_udp_driver.rs"].Content

	probe := strings.Index(driver, "codec::is_health_probe(datagram)")
	decode := strings.Index(driver, "codec::decode_request(datagram)")

	if probe < 0 || decode < 0 || probe > decode {
		t.Fatalf("the driver decodes before it answers the health probe:\n%s", driver)
	}

	for _, want := range []string{
		"const RECV_ERROR_PAUSE: Duration = Duration::from_millis(50);",
		"const MAX_CONSECUTIVE_RECV_ERRORS: usize = 100;",
		"const MAX_PEERS_TOLD_ABOUT_THE_VERSION: usize = 256;",
		"pub struct HelloDatagramUdpDriverConfig {",
		"pub struct HelloDatagramUdpDriver {",
		"    controller: Arc<dyn HelloDatagramController + Send + Sync>,",
		"    pub async fn bind(&mut self) -> Result<(), HelloDatagramUdpDriverError> {",
		"if peers_told_about_the_version.len() >= MAX_PEERS_TOLD_ABOUT_THE_VERSION {",
		"println!(\"LISTENING_UDP {}\", self.local_port()?);",
		"if peers_told_about_the_version.insert(peer) {",
		"dropping a datagram from {peer}: function hash {hash} names no rpc",
	} {
		if !strings.Contains(driver, want) {
			t.Fatalf("the driver never carried %q:\n%s", want, driver)
		}
	}
}

func TestTheParserIsTheOneGrpcRustTonicUses(t *testing.T) {
	const repeated = `syntax = "proto3";

package songe.hello.udp.v1;

service HelloDatagram {
  rpc Echo(Echo) returns (Echo);
}

message Echo {
  repeated string payload = 1;
}
`

	_, grpcErr := grpcrust.Parse([]byte(repeated))
	if grpcErr == nil {
		t.Fatal("the shared parser accepted a repeated field")
	}

	_, udpErr := udprust.Generate([]byte(repeated), udprust.Options{Service: "songe-hello"})
	if udpErr == nil {
		t.Fatal("udp-rust accepted a repeated field")
	}

	if udpErr.Error() != grpcErr.Error() {
		t.Fatalf("udp-rust reported %q, the shared parser reports %q", udpErr, grpcErr)
	}
}

func TestGeneratingRefusesAnInputThatNamesNoServiceAndNoCell(t *testing.T) {
	cases := []struct {
		name string
		doc  string
		opts udprust.Options
		want string
	}{
		{
			name: "no service name",
			doc:  helloProto,
			opts: udprust.Options{},
			want: "the service name is required",
		},
		{
			name: "a cell rust cannot spell",
			doc:  helloProto,
			opts: udprust.Options{Service: "songe-hello", Cell: "Udp Cell"},
			want: "is not a name Rust can spell as a module",
		},
		{
			name: "a cell that is a rust keyword",
			doc:  helloProto,
			opts: udprust.Options{Service: "songe-hello", Cell: "type"},
			want: "is not a name Rust can spell as a module",
		},
		{
			name: "a proto with no service block",
			doc:  "syntax = \"proto3\";\n\npackage songe.hello.udp.v1;\n\nmessage Echo {\n  string payload = 1;\n}\n",
			opts: udprust.Options{Service: "songe-hello"},
			want: "the proto document declares no service",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := udprust.Generate([]byte(tc.doc), tc.opts)
			if err == nil {
				t.Fatal("generating was accepted")
			}

			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("generating reported %q, want it to name %q", err, tc.want)
			}
		})
	}
}

func TestTheCellDefaultsToUdpAndTheMountPointsFollowIt(t *testing.T) {
	files := generate(t, udprust.Options{Service: "songe-hello"})

	if !strings.Contains(files["controller/zz_generated_hello_datagram_controller.rs"].Content, "use crate::udp::types::context::Context;") {
		t.Fatal("the controller never reached the context through the default cell")
	}

	named, err := udprust.Generate([]byte(helloProto), udprust.Options{Service: "songe-hello", Cell: "datagram"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	byPath := map[string]udprust.File{}
	for _, f := range named {
		byPath[f.Path] = f
	}

	if !strings.Contains(byPath["controller/zz_generated_hello_datagram_controller.rs"].Content, "use crate::datagram::types::context::Context;") {
		t.Fatal("the controller ignored the named cell")
	}
}

func TestEveryLayerAndTheCellCarryAGeneratedModFile(t *testing.T) {
	files := generate(t, udprust.Options{Service: "songe-hello"})

	if !strings.Contains(files["mod.rs"].Content, "pub mod adapter;\npub mod controller;\npub mod driver;\npub mod port;\npub mod types;") {
		t.Fatalf("the cell mod file lists the wrong layers:\n%s", files["mod.rs"].Content)
	}

	if !strings.Contains(files["controller/mod.rs"].Content, "pub use zz_generated_hello_datagram_codec as hello_datagram_codec;") {
		t.Fatalf("the controller mod file never aliased the codec:\n%s", files["controller/mod.rs"].Content)
	}

	if !strings.Contains(files["controller/mod.rs"].Content, "mod hello_datagram_controller;") {
		t.Fatalf("the controller mod file never mounts the user impl file:\n%s", files["controller/mod.rs"].Content)
	}

	if !strings.Contains(files["driver/mod.rs"].Content, "#![allow(clippy::disallowed_methods, clippy::disallowed_types)]") {
		t.Fatalf("the driver mod file never allows the io lint table:\n%s", files["driver/mod.rs"].Content)
	}
}
