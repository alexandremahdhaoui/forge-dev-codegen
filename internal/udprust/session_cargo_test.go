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
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/udprust"
)

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

const cargoSessionTest = `use std::sync::Arc;
use std::time::Duration;

use songe_hello::udp::adapter::hello_datagram_udp_broadcast::{
    HelloDatagramUdpBroadcast, HelloDatagramUdpBroadcastConfig,
};
use songe_hello::udp::adapter::hello_datagram_udp_client::{
    HelloDatagramUdpClient, HelloDatagramUdpClientConfig,
};
use songe_hello::udp::controller::hello_datagram_codec as codec;
use songe_hello::udp::controller::{HelloDatagramController, HelloDatagramControllerImpl};
use songe_hello::udp::driver::hello_datagram_tick_driver::{
    HelloDatagramTickDriver, HelloDatagramTickDriverConfig,
};
use songe_hello::udp::driver::hello_datagram_udp_driver::{
    HelloDatagramUdpDriver, HelloDatagramUdpDriverConfig,
};
use songe_hello::udp::port::hello_datagram_broadcast::HelloDatagramBroadcast;
use songe_hello::udp::port::hello_datagram_client::HelloDatagramClient;
use songe_hello::udp::port::hello_datagram_session_gate::{
    HelloDatagramSessionGate, HelloDatagramSessionGateError,
};
use songe_hello::udp::types::admission::Admission;
use songe_hello::udp::types::hello_datagram_messages::{Counter, Echo, Hello, Welcome};
use songe_hello::udp::types::hello_datagram_push::HelloDatagramPush;

const SECRET: &str = "open";
const SESSION: &str = "0123456789abcdef";
const OTHER: &str = "fedcba9876543210";

struct SecretGate;

impl HelloDatagramSessionGate for SecretGate {
    fn admit(
        &self,
        _session_id: &[u8; 16],
        request: &Hello,
        _peer: std::net::SocketAddr,
    ) -> Result<Admission, HelloDatagramSessionGateError> {
        if request.secret == SECRET {
            return Ok(Admission::Admitted);
        }

        Ok(Admission::Refused {
            reason: "the secret does not match".to_string(),
        })
    }
}

struct Stack {
    port: u16,
    broadcast: Arc<HelloDatagramUdpBroadcast>,
    controller: Arc<dyn HelloDatagramController + Send + Sync>,
}

async fn stand_up(max_sessions: i64) -> Stack {
    let broadcast = Arc::new(
        HelloDatagramUdpBroadcast::new(HelloDatagramUdpBroadcastConfig { max_sessions })
            .expect("a peer table"),
    );

    let controller: Arc<dyn HelloDatagramController + Send + Sync> =
        Arc::new(HelloDatagramControllerImpl::new(broadcast.clone()));

    let mut driver = HelloDatagramUdpDriver::new(
        HelloDatagramUdpDriverConfig {
            addr: "127.0.0.1:0".to_string(),
        },
        controller.clone(),
        Arc::new(SecretGate),
        broadcast.clone(),
    );

    driver.bind().await.expect("a bound socket");

    let port = driver.local_port().expect("a bound port");

    tokio::spawn(async move {
        let _ = driver.serve().await;
    });

    Stack {
        port,
        broadcast,
        controller,
    }
}

fn client(port: u16, session: &str, timeout_ms: i64) -> HelloDatagramUdpClient {
    HelloDatagramUdpClient::new(HelloDatagramUdpClientConfig {
        address: format!("127.0.0.1:{port}"),
        session_id: session.to_string(),
        timeout_ms,
    })
}

fn hello(secret: &str) -> Hello {
    Hello {
        secret: secret.to_string(),
    }
}

async fn raw_hello(port: u16, session: &str) -> tokio::net::UdpSocket {
    let socket = tokio::net::UdpSocket::bind("127.0.0.1:0")
        .await
        .expect("a bound socket");

    let datagram = codec::encode_hello_request(&codec::session_id_from(session), &hello(SECRET))
        .expect("a datagram");

    socket
        .send_to(&datagram, format!("127.0.0.1:{port}"))
        .await
        .expect("a sent hello");

    let mut buffer = [0u8; codec::MAX_DATAGRAM_LEN + 1];

    let read = tokio::time::timeout(Duration::from_secs(2), socket.recv(&mut buffer))
        .await
        .expect("a welcome in time")
        .expect("a welcome");

    let welcome: Welcome =
        codec::decode_hello_reply(&codec::session_id_from(session), &buffer[..read])
            .expect("a decoded welcome");

    assert_eq!(welcome.greeting, "welcome");

    socket
}

async fn recv_push(socket: &tokio::net::UdpSocket, session: &str) -> Option<HelloDatagramPush> {
    let mut buffer = [0u8; codec::MAX_DATAGRAM_LEN + 1];

    let read = tokio::time::timeout(Duration::from_millis(1500), socket.recv(&mut buffer))
        .await
        .ok()?
        .ok()?;

    Some(
        codec::decode_push(&codec::session_id_from(session), &buffer[..read])
            .expect("a decoded push"),
    )
}

#[tokio::test]
async fn a_hello_with_the_secret_is_admitted_and_welcomed() {
    let stack = stand_up(64).await;

    let welcome = client(stack.port, SESSION, 2000)
        .hello(hello(SECRET))
        .await
        .expect("a welcome");

    assert_eq!(welcome.greeting, "welcome");
    assert_eq!(stack.broadcast.sessions().expect("a count"), 1);
}

#[tokio::test]
async fn a_hello_with_a_wrong_secret_is_refused_and_never_answered() {
    let stack = stand_up(64).await;

    let error = client(stack.port, SESSION, 300)
        .hello(hello("wrong"))
        .await
        .expect_err("no welcome");

    assert!(error.to_string().contains("Hello"));
    assert_eq!(stack.broadcast.sessions().expect("a count"), 0);
}

#[tokio::test]
async fn an_echo_from_a_session_never_admitted_is_dropped() {
    let stack = stand_up(64).await;

    client(stack.port, SESSION, 300)
        .echo(Echo {
            payload: "songe".to_string(),
        })
        .await
        .expect_err("no echo");
}

#[tokio::test]
async fn an_echo_after_a_hello_is_answered_and_follows_the_latest_address() {
    let stack = stand_up(64).await;

    raw_hello(stack.port, SESSION).await;

    let echo = client(stack.port, SESSION, 2000)
        .echo(Echo {
            payload: "songe".to_string(),
        })
        .await
        .expect("an echo");

    assert_eq!(echo.payload, "songe");
}

#[tokio::test]
async fn a_hello_again_from_a_new_address_replaces_the_peer() {
    let stack = stand_up(64).await;

    let first = raw_hello(stack.port, SESSION).await;
    let second = raw_hello(stack.port, SESSION).await;

    assert_eq!(
        stack.broadcast.peer_of(&codec::session_id_from(SESSION)).expect("a peer"),
        Some(second.local_addr().expect("an address"))
    );

    stack
        .broadcast
        .send_to(
            &codec::session_id_from(SESSION),
            HelloDatagramPush::Counter(Counter { tick: 7 }),
        )
        .expect("a push");

    assert_eq!(
        recv_push(&second, SESSION).await,
        Some(HelloDatagramPush::Counter(Counter { tick: 7 }))
    );
    assert_eq!(recv_push(&first, SESSION).await, None);
}

#[tokio::test]
async fn a_full_peer_table_refuses_a_new_session_and_keeps_the_known_one() {
    let stack = stand_up(1).await;

    raw_hello(stack.port, SESSION).await;

    client(stack.port, OTHER, 300)
        .hello(hello(SECRET))
        .await
        .expect_err("no welcome");

    assert_eq!(stack.broadcast.sessions().expect("a count"), 1);
}

#[tokio::test]
async fn the_tick_driver_pushes_the_counter_to_every_admitted_session() {
    let stack = stand_up(64).await;

    let first = raw_hello(stack.port, SESSION).await;
    let second = raw_hello(stack.port, OTHER).await;

    let mut tick = HelloDatagramTickDriver::new(
        HelloDatagramTickDriverConfig { interval_ms: 10 },
        stack.controller.clone(),
    );

    tick.bind().await.expect("a bound interval");
    tick.announce().expect("an announced interval");

    tokio::spawn(async move {
        let _ = tick.serve().await;
    });

    assert!(matches!(
        recv_push(&first, SESSION).await,
        Some(HelloDatagramPush::Counter(Counter { tick })) if tick >= 1
    ));
    assert!(matches!(
        recv_push(&second, OTHER).await,
        Some(HelloDatagramPush::Counter(Counter { tick })) if tick >= 1
    ));
}

#[tokio::test]
async fn a_tick_interval_below_one_millisecond_is_refused_at_bind() {
    let stack = stand_up(64).await;

    let mut tick = HelloDatagramTickDriver::new(
        HelloDatagramTickDriverConfig { interval_ms: 0 },
        stack.controller.clone(),
    );

    let error = tick.bind().await.expect_err("a refusal");

    assert_eq!(
        error.to_string(),
        "binding the hello_datagram tick driver: interval_ms 0 is below 1"
    );
}

#[test]
fn a_push_datagram_sent_before_the_driver_is_bound_names_the_unbound_driver() {
    let broadcast =
        HelloDatagramUdpBroadcast::new(HelloDatagramUdpBroadcastConfig { max_sessions: 4 })
            .expect("a peer table");

    let session = codec::session_id_from(SESSION);
    let peer: std::net::SocketAddr = "127.0.0.1:9".parse().expect("an address");

    assert!(broadcast.admit_peer(&session, peer).expect("an admission"));

    let error = broadcast
        .send_to(&session, HelloDatagramPush::Counter(Counter { tick: 1 }))
        .expect_err("a refusal");

    assert_eq!(
        error.to_string(),
        "sending Counter: the hello_datagram udp driver is not bound"
    );
}

#[test]
fn a_peer_table_sized_below_one_is_refused() {
    let error = HelloDatagramUdpBroadcast::new(HelloDatagramUdpBroadcastConfig { max_sessions: 0 })
        .err()
        .expect("a refusal");

    assert_eq!(
        error.to_string(),
        "sizing the hello_datagram peer table: max_sessions 0 is below 1"
    );
}
`

func standUpTheSessionCell(t *testing.T) (string, string) {
	t.Helper()

	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Skip("cargo is not on PATH")
	}

	files, err := udprust.Generate([]byte(sessionProto), sessionOptions())
	if err != nil {
		t.Fatalf("generating the cell: %v", err)
	}

	root := t.TempDir()
	write := writeUnder(t, root)

	write("Cargo.toml", cargoCrateManifest)
	write("src/lib.rs", cargoCellLib)
	write("tests/session.rs", cargoSessionTest)

	for _, f := range files {
		if strings.HasSuffix(f.Path, ".yaml") {
			continue
		}

		write(filepath.Join("src", "udp", f.Path), f.Content)
	}

	write("src/udp/controller/hello_datagram_controller.rs", cargoSessionControllerImpl)

	return cargo, root
}

func TestTheGeneratedSessionCellGatesAHelloFollowsThePeerAndPushesOnTick(t *testing.T) {
	cargo, root := standUpTheSessionCell(t)

	runCargo(t, cargo, root, "test", "--workspace")
}
