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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/udprust"
)

const cargoWorkspaceManifest = `[workspace]
resolver = "2"
members = ["core", "app"]
`

const cargoCoreManifest = `[package]
name = "songe-hello-core"
version = "0.1.0"
edition = "2021"

[dependencies]
prost = "0.14"
thiserror = "2"

[dev-dependencies]
mockall = "0.15"
`

const cargoAppManifest = `[package]
name = "songe-hello-app"
version = "0.1.0"
edition = "2021"

[dependencies]
songe-hello-core = { path = "../core" }
prost = "0.14"
thiserror = "2"
tokio = { version = "1", features = ["macros", "net", "rt-multi-thread", "time"] }

[dev-dependencies]
mockall = "0.15"
`

const cargoCellLib = `pub mod udp;
`

const cargoRoundTripTest = `use std::sync::Arc;

use songe_hello_app::udp::adapter::hello_datagram_udp_client::HelloDatagramUdpClient;
use songe_hello_app::udp::driver::hello_datagram_udp_driver::HelloDatagramUdpDriver;
use songe_hello_core::udp::controller::hello_datagram_codec as codec;
use songe_hello_core::udp::controller::hello_datagram_controller::{
    HelloDatagramController, HelloDatagramControllerError,
};
use songe_hello_core::udp::port::hello_datagram_client::HelloDatagramClient;
use songe_hello_core::udp::types::context::Context;
use songe_hello_core::udp::types::hello_datagram_messages::{Echo, Note, Nothing};

mockall::mock! {
    Controller {}

    impl HelloDatagramController for Controller {
        fn echo(&self, request: Echo, context: &Context) -> Result<Echo, HelloDatagramControllerError>;
        fn note(&self, request: Note, context: &Context) -> Result<Nothing, HelloDatagramControllerError>;
    }
}

const SESSION: [u8; codec::SESSION_ID_LEN] = *b"0123456789abcdef";

fn echo() -> Echo {
    Echo {
        payload: "songe".to_string(),
        count: 7,
    }
}

#[test]
fn a_framed_echo_request_comes_back_out_of_the_codec_unchanged() {
    let datagram = codec::encode_echo_request(&SESSION, &echo()).expect("a datagram");

    let (session_id, request) = codec::decode_request(&datagram).expect("a request");

    assert_eq!(session_id, SESSION);
    assert_eq!(request, codec::HelloDatagramRequest::Echo(echo()));
}

#[test]
fn a_framed_reply_comes_back_out_of_the_codec_unchanged() {
    let datagram = codec::encode_echo_reply(&SESSION, &echo()).expect("a datagram");

    assert_eq!(codec::decode_echo_reply(&datagram).expect("a reply"), echo());
}

#[test]
fn a_datagram_that_does_not_open_with_the_udplb_magic_is_refused() {
    let mut datagram = codec::encode_echo_request(&SESSION, &echo()).expect("a datagram");
    datagram[0] = 0x00;

    let error = codec::decode_request(&datagram).expect_err("a refusal");

    assert!(matches!(error, codec::HelloDatagramCodecError::Magic { .. }));
}

#[test]
fn a_datagram_shorter_than_a_magic_a_session_id_and_a_tag_is_refused() {
    let error = codec::decode_request(&[0u8; codec::HEADER_LEN - 1]).expect_err("a refusal");

    assert!(matches!(error, codec::HelloDatagramCodecError::Short { .. }));
}

#[test]
fn a_datagram_over_five_hundred_and_eight_bytes_is_refused() {
    let error =
        codec::decode_request(&vec![0u8; codec::MAX_DATAGRAM_LEN + 1]).expect_err("a refusal");

    assert!(matches!(error, codec::HelloDatagramCodecError::Long { .. }));
}

#[test]
fn a_datagram_whose_function_hash_names_no_rpc_is_refused() {
    let mut datagram = codec::encode_echo_request(&SESSION, &echo()).expect("a datagram");
    datagram[codec::MAGIC_LEN + codec::SESSION_ID_LEN + codec::VERSION_LEN] = 0x01;

    let error = codec::decode_request(&datagram).expect_err("a refusal");

    assert!(matches!(
        error,
        codec::HelloDatagramCodecError::UnknownMethod { .. }
    ));
}

#[test]
fn a_datagram_that_speaks_another_schema_version_is_refused() {
    let mut datagram = codec::encode_echo_request(&SESSION, &echo()).expect("a datagram");
    datagram[codec::MAGIC_LEN + codec::SESSION_ID_LEN] = codec::SCHEMA_VERSION + 1;

    let error = codec::decode_request(&datagram).expect_err("a refusal");

    assert!(matches!(
        error,
        codec::HelloDatagramCodecError::Version { .. }
    ));
}

#[test]
fn a_datagram_carries_the_magic_the_session_id_the_schema_version_and_the_function_hash_in_that_order(
) {
    let datagram = codec::encode_echo_request(&SESSION, &echo()).expect("a datagram");

    assert_eq!(&datagram[..codec::MAGIC_LEN], &codec::MAGIC);
    assert_eq!(
        &datagram[codec::MAGIC_LEN..codec::MAGIC_LEN + codec::SESSION_ID_LEN],
        &SESSION
    );
    assert_eq!(
        datagram[codec::MAGIC_LEN + codec::SESSION_ID_LEN],
        codec::SCHEMA_VERSION
    );
    assert_eq!(
        datagram[codec::MAGIC_LEN + codec::SESSION_ID_LEN + codec::VERSION_LEN],
        codec::ECHO_HASH
    );
    assert_eq!(codec::HEADER_LEN, 22);
    assert_eq!(codec::MAX_PAYLOAD_LEN, 486);
}

#[test]
fn a_payload_that_would_push_a_datagram_over_five_hundred_and_eight_bytes_is_refused() {
    let oversize = Echo {
        payload: "a".repeat(codec::MAX_DATAGRAM_LEN),
        count: 0,
    };

    let error = codec::encode_echo_request(&SESSION, &oversize).expect_err("a refusal");

    assert!(matches!(
        error,
        codec::HelloDatagramCodecError::Oversize { .. }
    ));
}

#[test]
fn a_datagram_whose_session_id_is_sixteen_zero_bytes_is_the_udplb_health_probe() {
    let probe = codec::frame(&[0u8; codec::SESSION_ID_LEN], codec::ECHO_HASH, &[]);

    assert!(codec::is_health_probe(&probe));
    assert!(!codec::is_health_probe(
        &codec::encode_echo_request(&SESSION, &echo()).expect("a datagram")
    ));
}

#[tokio::test]
async fn the_generated_client_round_trips_one_datagram_through_the_generated_driver() {
    let mut controller = MockController::new();
    controller.expect_echo().returning(|request, _| {
        Ok(Echo {
            payload: request.payload,
            count: request.count + 1,
        })
    });

    let socket = tokio::net::UdpSocket::bind("127.0.0.1:0")
        .await
        .expect("a bound socket");

    let driver = HelloDatagramUdpDriver::new(socket, Arc::new(controller));
    let port = driver.local_port().expect("a bound port");

    driver.announce().expect("an announced port");

    tokio::spawn(async move {
        let _ = driver.serve().await;
    });

    let client = HelloDatagramUdpClient::new(format!("127.0.0.1:{port}"), SESSION);

    let reply = client.echo(echo()).await.expect("a reply");

    assert_eq!(reply.payload, "songe");
    assert_eq!(reply.count, 8);
}

#[tokio::test]
async fn the_driver_answers_the_health_probe_with_the_zeroed_session_id_it_received() {
    let mut controller = MockController::new();
    controller.expect_echo().never();

    let socket = tokio::net::UdpSocket::bind("127.0.0.1:0")
        .await
        .expect("a bound socket");

    let driver = HelloDatagramUdpDriver::new(socket, Arc::new(controller));
    let port = driver.local_port().expect("a bound port");

    tokio::spawn(async move {
        let _ = driver.serve().await;
    });

    let probe = codec::frame(&[0u8; codec::SESSION_ID_LEN], codec::ECHO_HASH, &[]);

    let prober = tokio::net::UdpSocket::bind("127.0.0.1:0")
        .await
        .expect("a bound socket");

    prober
        .send_to(&probe, format!("127.0.0.1:{port}"))
        .await
        .expect("a sent probe");

    let mut buffer = [0u8; codec::MAX_DATAGRAM_LEN + 1];

    let read = tokio::time::timeout(
        std::time::Duration::from_secs(2),
        prober.recv(&mut buffer),
    )
    .await
    .expect("an answer in time")
    .expect("an answer");

    assert_eq!(&buffer[..read], probe.as_slice());
}
`

func writeUnder(t *testing.T, root string) func(rel, content string) {
	t.Helper()

	return func(rel, content string) {
		t.Helper()

		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("making %s: %v", filepath.Dir(p), err)
		}

		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", p, err)
		}
	}
}

func standUpTheCell(t *testing.T) (string, string) {
	t.Helper()

	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Skip("cargo is not on PATH")
	}

	coreFiles, err := udprust.Generate([]byte(helloProto), udprust.Options{Service: "songe-hello", Side: "core"})
	if err != nil {
		t.Fatalf("generating the core cell: %v", err)
	}

	appFiles, err := udprust.Generate([]byte(helloProto), udprust.Options{Service: "songe-hello", Side: "app"})
	if err != nil {
		t.Fatalf("generating the app cell: %v", err)
	}

	root := t.TempDir()
	write := writeUnder(t, root)

	write("Cargo.toml", cargoWorkspaceManifest)
	write("core/Cargo.toml", cargoCoreManifest)
	write("core/src/lib.rs", cargoCellLib)
	write("app/Cargo.toml", cargoAppManifest)
	write("app/src/lib.rs", cargoCellLib)
	write("app/tests/round_trip.rs", cargoRoundTripTest)

	for _, f := range coreFiles {
		write(filepath.Join("core", "src", "udp", f.Path), f.Content)
	}

	for _, f := range appFiles {
		write(filepath.Join("app", "src", "udp", f.Path), f.Content)
	}

	return cargo, root
}

func runCargo(t *testing.T, cargo, root string, args ...string) {
	t.Helper()

	cmd := exec.Command(cargo, args...)
	cmd.Dir = root

	out, err := cmd.CombinedOutput()
	if err == nil {
		return
	}

	lower := strings.ToLower(string(out))
	if strings.Contains(lower, "could not resolve host") ||
		strings.Contains(lower, "failed to get") ||
		strings.Contains(lower, "network") ||
		strings.Contains(lower, "spurious network error") {
		t.Skipf("cargo %v needs network access to crates.io, which this run did not have: %v\n%s", args, err, out)
	}

	t.Fatalf("cargo %v: %v\n%s", args, err, out)
}

func TestTheGeneratedCellPassesCargoCheck(t *testing.T) {
	cargo, root := standUpTheCell(t)

	runCargo(t, cargo, root, "check", "--workspace", "--all-targets")
}

func TestTheGeneratedDriverAndClientRoundTripOneDatagramOverALoopbackSocket(t *testing.T) {
	cargo, root := standUpTheCell(t)

	runCargo(t, cargo, root, "test", "--workspace")
}
