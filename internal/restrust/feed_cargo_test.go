package restrust_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/restrust"
)

const feedCrateManifest = `[workspace]

[package]
name = "songe-social"
version = "0.1.0"
edition = "2021"

[dependencies]
axum = "0.8.9"
http-body-util = { version = "0.1.5", features = ["channel"] }
rusqlite = { version = "0.40.2", features = ["bundled"] }
serde = { version = "1.0.229", features = ["derive"] }
serde_json = "1.0.151"
thiserror = "2.0.20"
tokio = { version = "1.53.1", features = ["full"] }

[dev-dependencies]
mockall = "0.15.0"
`

const chatControllerImpl = `use std::sync::mpsc::Receiver;

use crate::rest::controller::{ChatController, ChatControllerError, ChatControllerImpl};
use crate::rest::types::chat_line::ChatLine;

impl ChatController for ChatControllerImpl {
    fn stream_chat_lines(&self, chat_id: &str) -> Result<Receiver<ChatLine>, ChatControllerError> {
        self.chat_line_subscribe
            .subscribe(chat_id)
            .map_err(|source| ChatControllerError::ChatLineSubscribe {
                id: chat_id.to_string(),
                source,
            })
    }
}
`

const feedRustTest = `use std::sync::mpsc::TryRecvError;
use std::sync::Arc;
use std::time::Duration;

use songe_social::rest::adapter::chat_line_memory::{ChatLineMemoryStore, ChatLineMemoryStoreConfig};
use songe_social::rest::adapter::chat_line_memory_feed::{ChatLineMemoryFeed, ChatLineMemoryFeedConfig};
use songe_social::rest::adapter::chat_line_sqlite::{ChatLineSqliteStore, ChatLineSqliteStoreConfig};
use songe_social::rest::port::chat_line_store::ChatLineStore;
use songe_social::rest::port::chat_line_subscribe::ChatLineSubscribe;
use songe_social::rest::types::chat_line::ChatLine;

fn line() -> ChatLine {
    ChatLine {
        chat_id: "chat-1".to_string(),
        line_id: "line-1".to_string(),
        text: "hello".to_string(),
    }
}

fn feed() -> Arc<ChatLineMemoryFeed> {
    Arc::new(ChatLineMemoryFeed::new(ChatLineMemoryFeedConfig::default()))
}

fn a_put_reaches_the_chat_subscriber_and_never_the_line_subscriber(store: &dyn ChatLineStore, feed: &ChatLineMemoryFeed) {
    let by_chat = feed.subscribe("chat-1").expect("subscribing under the chat id");
    let by_line = feed.subscribe("line-1").expect("subscribing under the line id");

    store.put(line()).expect("putting the line");

    assert_eq!(by_chat.recv_timeout(Duration::from_secs(1)), Ok(line()));
    assert_eq!(by_line.try_recv(), Err(TryRecvError::Empty));
}

#[test]
fn the_sqlite_store_publishes_a_put_line_under_its_chat_id_and_never_under_its_line_id() {
    let feed = feed();
    let path = std::path::Path::new(env!("CARGO_TARGET_TMPDIR")).join("feed.db");
    let _ = std::fs::remove_file(&path);
    let store = ChatLineSqliteStore::new(
        ChatLineSqliteStoreConfig {
            path: path.display().to_string(),
        },
        feed.clone(),
    )
    .expect("opening the sqlite store");

    a_put_reaches_the_chat_subscriber_and_never_the_line_subscriber(&store, &feed);
}

#[test]
fn the_memory_store_publishes_a_put_line_under_its_chat_id_and_never_under_its_line_id() {
    let feed = feed();
    let store = ChatLineMemoryStore::new(ChatLineMemoryStoreConfig { capacity: 8 }, feed.clone())
        .expect("sizing the memory store");

    a_put_reaches_the_chat_subscriber_and_never_the_line_subscriber(&store, &feed);
}
`

const feedRustTestCount = "2 passed"

func TestAChatLineStoredByLineIdIsPublishedUnderItsChatIdThroughBothStoreAdapters(t *testing.T) {
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Fatalf("finding cargo, which this contract runs through and never skips: %v", err)
	}

	files, err := restrust.Generate([]byte(chatLineFeedSpec(t)), restrust.Options{Service: "songe-social"})
	if err != nil {
		t.Fatalf("generating: %v", err)
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

	write("Cargo.toml", feedCrateManifest)
	write("src/lib.rs", "pub mod rest;\n")

	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".rs") {
			continue
		}

		write(filepath.Join("src", "rest", f.Path), f.Content)
	}

	write("src/rest/controller/chat_controller.rs", chatControllerImpl)
	write("tests/feed.rs", feedRustTest)

	mustCargo(t, cargo, root, "clippy", "--all-targets", "--", "-D", "warnings")

	out := mustCargo(t, cargo, root, "test", "--test", "feed")
	if !strings.Contains(out, feedRustTestCount) {
		t.Fatalf("the feed contract never reported %q, so some of its tests did not run:\n%s", feedRustTestCount, out)
	}
}
