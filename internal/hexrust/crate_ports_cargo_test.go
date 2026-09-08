package hexrust_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/hexrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/restrust"
)

const guardedCrateManifest = `[workspace]

[package]
name = "songe-hello"
version = "0.1.0"
edition = "2021"

[[bin]]
name = "songe-hello-node"
path = "src/bin/zz_generated_songe_hello_node.rs"

[dependencies]
anyhow = "1"
axum = "0.8"
http-body-util = { version = "0.1", features = ["channel"] }
reqwest = { version = "0.13", default-features = false, features = ["json", "rustls"] }
rusqlite = { version = "0.40", features = ["bundled"] }
serde = { version = "1", features = ["derive"] }
serde_json = "1"
songe-common = { path = "SONGE_COMMON_DIR" }
thiserror = "2"
tokio = { version = "1", features = ["full"] }

[dev-dependencies]
mockall = "0.15"
`

const guardedConfigLoader = `#[derive(Debug)]
pub struct ConfigError {
    message: String,
}

impl std::fmt::Display for ConfigError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", self.message)
    }
}

impl std::error::Error for ConfigError {}

pub struct SongeHelloConfig {
    pub greeting_store: String,
    pub greeting_store_sqlite_path: String,
    pub ticket_verifier: String,
    pub ticket_verifier_memory_secret: String,
    pub greeting_event_subscribe: String,
    pub driver_rest: bool,
    pub rest_addr: String,
}

impl SongeHelloConfig {
    pub fn load(args: &[String]) -> Result<Self, ConfigError> {
        if let Some(arg) = args.first() {
            return Err(ConfigError {
                message: format!("reading {arg:?}: this loader takes no argument"),
            });
        }

        Ok(Self {
            greeting_store: "sqlite".to_string(),
            greeting_store_sqlite_path: ":memory:".to_string(),
            ticket_verifier: "memory".to_string(),
            ticket_verifier_memory_secret: "open".to_string(),
            greeting_event_subscribe: "memory".to_string(),
            driver_rest: true,
            rest_addr: "127.0.0.1:0".to_string(),
        })
    }
}
`

const guardedControllerImpl = `use crate::rest::controller::{
    GreetingController, GreetingControllerError, GreetingControllerImpl,
};
use crate::rest::types::greeting::Greeting;
use crate::rest::types::greeting_event::GreetingEvent;
use crate::types::subject::Subject;

impl GreetingController for GreetingControllerImpl {
    fn count_greeting(&self, subject: Subject, id: &str) -> Result<Greeting, GreetingControllerError> {
        let greeting = self
            .greeting_store
            .get(id)
            .map_err(|source| GreetingControllerError::GreetingStore {
                id: id.to_string(),
                source,
            })?
            .ok_or(GreetingControllerError::NotFound { id: id.to_string() })?;

        Ok(Greeting {
            id: greeting.id,
            name: subject.id,
        })
    }

    fn stream_greeting_events(
        &self,
        _subject: Subject,
        id: &str,
    ) -> Result<std::sync::mpsc::Receiver<GreetingEvent>, GreetingControllerError> {
        self.greeting_event_subscribe
            .subscribe(id)
            .map_err(|source| GreetingControllerError::GreetingEventSubscribe {
                id: id.to_string(),
                source,
            })
    }
}
`

const guardedTicketMemory = `use crate::adapter::TicketMemoryVerifierConfig;
use crate::port::ticket_verifier::{TicketVerifier, TicketVerifierError};
use crate::types::subject::Subject;

pub struct TicketMemoryVerifier {
    secret: String,
}

impl TicketMemoryVerifier {
    pub fn new(config: TicketMemoryVerifierConfig) -> Self {
        Self {
            secret: config.secret,
        }
    }
}

impl TicketVerifier for TicketMemoryVerifier {
    fn verify(&self, token: &str) -> Result<Subject, TicketVerifierError> {
        if token != self.secret {
            return Err(TicketVerifierError::Refused {
                reason: "the ticket does not match".to_string(),
            });
        }

        Ok(Subject {
            id: "friend".to_string(),
        })
    }
}
`

const guardedEventMemory = `use crate::adapter::GreetingEventMemoryFeedConfig;
use crate::rest::port::greeting_event_subscribe::{
    GreetingEventSubscribe, GreetingEventSubscribeError,
};
use crate::rest::types::greeting_event::GreetingEvent;

pub struct GreetingEventMemoryFeed;

impl GreetingEventMemoryFeed {
    pub fn new(_config: GreetingEventMemoryFeedConfig) -> Self {
        Self
    }
}

impl GreetingEventSubscribe for GreetingEventMemoryFeed {
    fn subscribe(&self, key: &str) -> Result<std::sync::mpsc::Receiver<GreetingEvent>, GreetingEventSubscribeError> {
        let (sender, receiver) = std::sync::mpsc::channel();

        sender
            .send(GreetingEvent { id: key.to_string() })
            .map_err(|source| GreetingEventSubscribeError::Subscribe {
                key: key.to_string(),
                source: Box::new(source),
            })?;

        Ok(receiver)
    }
}
`

func TestTwoRestCellsShareTheCrateRootPortsAndTheCrateCompiles(t *testing.T) {
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Skip("cargo is not on PATH")
	}

	root := t.TempDir()
	write := writeUnder(t, root)

	writeRestCell(t, root, "rest", guardedHelloSpec, restrust.SideBoth, true)
	writeRestCell(t, root, "account", accountSpec, restrust.SideClient, true)

	files, err := hexrust.Generate(hexrust.Options{
		Service: "songe-hello",
		SrcDir:  root,
		Cells:   []string{"account", "rest"},
		Wiring: []byte(`binary: songe-hello-node
ports:
  GreetingStore:
    default: sqlite
    adapters:
      sqlite: {}
  TicketVerifier:
    default: memory
    adapters:
      memory:
        type: TicketMemoryVerifier
        module: adapter::ticket_memory
        config:
          secret: { type: string, default: open }
  GreetingEventSubscribe:
    default: memory
    adapters:
      memory:
        type: GreetingEventMemoryFeed
        module: adapter::greeting_event_memory
drivers:
  rest: { enabled: true }
`),
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	for _, f := range files {
		if strings.HasSuffix(f.Path, ".rs") {
			write(f.Path, f.Content)
		}
	}

	write("Cargo.toml", strings.ReplaceAll(guardedCrateManifest, "SONGE_COMMON_DIR", songeCommonDir(t)))
	write("src/config/zz_generated_config.rs", guardedConfigLoader)
	write("src/adapter/ticket_memory.rs", guardedTicketMemory)
	write("src/adapter/greeting_event_memory.rs", guardedEventMemory)
	write("src/rest/controller/greeting_controller.rs", guardedControllerImpl)

	out, err := runCargo(t, cargo, root, "check", "--workspace", "--all-targets")
	if err != nil {
		skipOnNetwork(t, out, err)
		t.Fatalf("cargo check: %v\n%s", err, out)
	}
}
