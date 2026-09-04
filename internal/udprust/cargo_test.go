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

const cargoCrateManifest = `[package]
name = "songe-hello"
version = "0.1.0"
edition = "2021"

[dependencies]
prost = "0.14"
thiserror = "2"
tokio = { version = "1", features = ["macros", "net", "rt-multi-thread", "time"] }

[dev-dependencies]
mockall = "0.15"
`

const cargoCellLib = `pub mod udp;
`

const cargoControllerImpl = `use crate::udp::controller::{
    HelloDatagramController, HelloDatagramControllerError, HelloDatagramControllerImpl,
};
use crate::udp::types::context::Context;
use crate::udp::types::hello_datagram_messages::{Echo, Note, Nothing};

impl HelloDatagramController for HelloDatagramControllerImpl {
    fn echo(&self, request: Echo, context: &Context) -> Result<Echo, HelloDatagramControllerError> {
        let _ = context;

        Ok(Echo {
            payload: request.payload,
            count: request.count + 1,
        })
    }

    fn note(&self, request: Note, context: &Context) -> Result<Nothing, HelloDatagramControllerError> {
        let _ = request;
        let _ = context;

        Ok(Nothing {})
    }
}
`

const cargoRoundTripTest = `use std::sync::Arc;

use songe_hello::udp::adapter::hello_datagram_udp_client::{
    HelloDatagramUdpClient, HelloDatagramUdpClientConfig,
};
use songe_hello::udp::controller::hello_datagram_codec as codec;
use songe_hello::udp::controller::{HelloDatagramController, HelloDatagramControllerError};
use songe_hello::udp::driver::hello_datagram_udp_driver::{
    HelloDatagramUdpDriver, HelloDatagramUdpDriverConfig,
};
use songe_hello::udp::port::hello_datagram_client::HelloDatagramClient;
use songe_hello::udp::types::context::Context;
use songe_hello::udp::types::hello_datagram_messages::{Echo, Note, Nothing};

mockall::mock! {
    Controller {}

    impl HelloDatagramController for Controller {
        fn echo(&self, request: Echo, context: &Context) -> Result<Echo, HelloDatagramControllerError>;
        fn note(&self, request: Note, context: &Context) -> Result<Nothing, HelloDatagramControllerError>;
    }
}

const SESSION: &str = "0123456789abcdef";

fn session() -> [u8; codec::SESSION_ID_LEN] {
    codec::session_id_from(SESSION)
}

fn echo() -> Echo {
    Echo {
        payload: "songe".to_string(),
        count: 7,
    }
}

fn client_config(port: u16) -> HelloDatagramUdpClientConfig {
    HelloDatagramUdpClientConfig {
        address: format!("127.0.0.1:{port}"),
        session_id: SESSION.to_string(),
        timeout_ms: 2000,
    }
}

#[test]
fn a_session_id_shorter_than_sixteen_bytes_is_padded_with_zeros() {
    let mut want = [0u8; codec::SESSION_ID_LEN];
    want[..4].copy_from_slice(b"abcd");

    assert_eq!(codec::session_id_from("abcd"), want);
}

#[test]
fn a_session_id_longer_than_sixteen_bytes_is_cut_to_sixteen() {
    assert_eq!(
        codec::session_id_from("0123456789abcdefghij"),
        *b"0123456789abcdef"
    );
}

#[test]
fn a_framed_echo_request_comes_back_out_of_the_codec_unchanged() {
    let datagram = codec::encode_echo_request(&session(), &echo()).expect("a datagram");

    let (session_id, request) = codec::decode_request(&datagram).expect("a request");

    assert_eq!(session_id, session());
    assert_eq!(request, codec::HelloDatagramRequest::Echo(echo()));
}

#[test]
fn a_framed_reply_comes_back_out_of_the_codec_unchanged() {
    let datagram = codec::encode_echo_reply(&session(), &echo()).expect("a datagram");

    assert_eq!(
        codec::decode_echo_reply(&session(), &datagram).expect("a reply"),
        echo()
    );
}

#[test]
fn a_reply_framed_with_another_session_id_is_refused() {
    let mut datagram = codec::encode_echo_reply(&session(), &echo()).expect("a datagram");
    datagram[codec::MAGIC_LEN] = b'z';

    let error = codec::decode_echo_reply(&session(), &datagram).expect_err("a refusal");

    assert!(matches!(
        error,
        codec::HelloDatagramCodecError::UnknownSession { .. }
    ));
}

#[test]
fn a_datagram_that_does_not_open_with_the_udplb_magic_is_refused() {
    let mut datagram = codec::encode_echo_request(&session(), &echo()).expect("a datagram");
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
    let mut datagram = codec::encode_echo_request(&session(), &echo()).expect("a datagram");
    datagram[codec::MAGIC_LEN + codec::SESSION_ID_LEN + codec::VERSION_LEN] = 0x01;

    let error = codec::decode_request(&datagram).expect_err("a refusal");

    assert!(matches!(
        error,
        codec::HelloDatagramCodecError::UnknownMethod { .. }
    ));
}

#[test]
fn a_datagram_that_speaks_another_schema_version_is_refused() {
    let mut datagram = codec::encode_echo_request(&session(), &echo()).expect("a datagram");
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
    let datagram = codec::encode_echo_request(&session(), &echo()).expect("a datagram");

    assert_eq!(&datagram[..codec::MAGIC_LEN], &codec::MAGIC);
    assert_eq!(
        &datagram[codec::MAGIC_LEN..codec::MAGIC_LEN + codec::SESSION_ID_LEN],
        &session()
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

    let error = codec::encode_echo_request(&session(), &oversize).expect_err("a refusal");

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
        &codec::encode_echo_request(&session(), &echo()).expect("a datagram")
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

    let mut driver = HelloDatagramUdpDriver::new(
        HelloDatagramUdpDriverConfig {
            addr: "127.0.0.1:0".to_string(),
        },
        Arc::new(controller),
    );

    driver.bind().await.expect("a bound socket");

    let port = driver.local_port().expect("a bound port");

    driver.announce().expect("an announced port");

    tokio::spawn(async move {
        let _ = driver.serve().await;
    });

    let client = HelloDatagramUdpClient::new(client_config(port));

    let reply = client.echo(echo()).await.expect("a reply");

    assert_eq!(reply.payload, "songe");
    assert_eq!(reply.count, 8);
}

#[tokio::test]
async fn the_driver_answers_the_health_probe_with_the_zeroed_session_id_it_received() {
    let mut controller = MockController::new();
    controller.expect_echo().never();

    let mut driver = HelloDatagramUdpDriver::new(
        HelloDatagramUdpDriverConfig {
            addr: "127.0.0.1:0".to_string(),
        },
        Arc::new(controller),
    );

    driver.bind().await.expect("a bound socket");

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

#[tokio::test]
async fn a_driver_that_was_never_bound_refuses_to_announce_and_names_its_address() {
    let mut controller = MockController::new();
    controller.expect_echo().never();

    let driver = HelloDatagramUdpDriver::new(
        HelloDatagramUdpDriverConfig {
            addr: "127.0.0.1:0".to_string(),
        },
        Arc::new(controller),
    );

    let error = driver.announce().expect_err("a refusal");

    assert!(error.to_string().contains("it is not bound yet"));
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

	files, err := udprust.Generate([]byte(helloProto), udprust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating the cell: %v", err)
	}

	root := t.TempDir()
	write := writeUnder(t, root)

	write("Cargo.toml", cargoCrateManifest)
	write("src/lib.rs", cargoCellLib)
	write("tests/round_trip.rs", cargoRoundTripTest)

	for _, f := range files {
		if strings.HasSuffix(f.Path, ".yaml") {
			continue
		}

		write(filepath.Join("src", "udp", f.Path), f.Content)
	}

	write("src/udp/controller/hello_datagram_controller.rs", cargoControllerImpl)

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

func TestDeletingTheControllerImplFailsTheBuildAndNamesTheFile(t *testing.T) {
	cargo, root := standUpTheCell(t)

	if err := os.Remove(filepath.Join(root, "src", "udp", "controller", "hello_datagram_controller.rs")); err != nil {
		t.Fatalf("removing the controller impl: %v", err)
	}

	cmd := exec.Command(cargo, "check", "--workspace")
	cmd.Dir = root

	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("the build passed without the controller impl\n%s", out)
	}

	text := string(out)

	if !strings.Contains(text, "E0583") || !strings.Contains(text, "hello_datagram_controller") {
		t.Fatalf("the build never named the missing file\n%s", text)
	}
}
