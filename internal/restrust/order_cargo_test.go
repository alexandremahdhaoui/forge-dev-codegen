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

package restrust_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/restrust"
)

const orderCrateManifest = `[workspace]

[package]
name = "songe-social"
version = "0.1.0"
edition = "2021"

[dependencies]
axum = "0.8.9"
rusqlite = { version = "0.40", features = ["bundled"] }
serde = { version = "1.0.229", features = ["derive"] }
serde_json = "1.0.151"
thiserror = "2.0.20"
tokio = { version = "1.53.1", features = ["full"] }

[dev-dependencies]
mockall = "0.15.0"
`

const orderChatControllerImpl = `use crate::rest::controller::{ChatController, ChatControllerError, ChatControllerImpl};
use crate::rest::types::chat_line::ChatLine;

impl ChatController for ChatControllerImpl {
    fn get_chat_line(&self, id: &str) -> Result<ChatLine, ChatControllerError> {
        self.chat_line_store
            .get(id)?
            .ok_or(ChatControllerError::NotFound { id: id.to_string() })
    }
}
`

const orderRustTest = `use songe_social::rest::adapter::chat_line_memory::{ChatLineMemoryStore, ChatLineMemoryStoreConfig};
use songe_social::rest::adapter::chat_line_sqlite::{ChatLineSqliteStore, ChatLineSqliteStoreConfig};
use songe_social::rest::port::chat_line_store::{ChatLineCursor, ChatLineStore, ChatLineStoreError};
use songe_social::rest::types::chat_line::ChatLine;

fn chat_line(id: &str, chat_id: &str, posted_at: &str) -> ChatLine {
    ChatLine {
        id: id.to_string(),
        chat_id: chat_id.to_string(),
        account_id: "account-1".to_string(),
        line: format!("line {id}"),
        posted_at: posted_at.to_string(),
        edits: 0,
    }
}

fn sqlite_store(name: &str) -> ChatLineSqliteStore {
    let path = std::path::Path::new(env!("CARGO_TARGET_TMPDIR")).join(format!("{name}.db"));
    let _ = std::fs::remove_file(&path);

    ChatLineSqliteStore::new(ChatLineSqliteStoreConfig {
        path: path.display().to_string(),
    })
    .unwrap()
}

fn memory_store() -> ChatLineMemoryStore {
    ChatLineMemoryStore::new(ChatLineMemoryStoreConfig { capacity: 64 }).unwrap()
}

fn both_stores(name: &str) -> (ChatLineSqliteStore, ChatLineMemoryStore) {
    (sqlite_store(name), memory_store())
}

fn ids(rows: &[ChatLine]) -> Vec<&str> {
    rows.iter().map(|row| row.id.as_str()).collect()
}

fn error_chain(error: &dyn std::error::Error) -> String {
    let mut text = error.to_string();
    let mut cause = error.source();

    while let Some(next) = cause {
        text.push_str(": ");
        text.push_str(&next.to_string());
        cause = next.source();
    }

    text
}

#[test]
fn both_stores_answer_a_page_in_the_declared_order_and_not_in_key_order() {
    let (sqlite, memory) = both_stores("declared_order");
    let stores: [&dyn ChatLineStore; 2] = [&sqlite, &memory];

    for store in stores {
        store.put(chat_line("line-a", "chat-1", "2026-10-30T12:00:03.000Z")).unwrap();
        store.put(chat_line("line-b", "chat-1", "2026-10-30T12:00:01.000Z")).unwrap();
        store.put(chat_line("line-c", "chat-1", "2026-10-30T12:00:02.000Z")).unwrap();
        store.put(chat_line("line-d", "chat-2", "2026-10-30T12:00:00.000Z")).unwrap();

        let page = store.page_by_chat_id("chat-1", None, None).unwrap();

        assert_eq!(ids(&page), vec!["line-b", "line-c", "line-a"]);
    }
}

#[test]
fn both_stores_break_a_tie_on_the_order_value_by_the_key() {
    let (sqlite, memory) = both_stores("tie");
    let stores: [&dyn ChatLineStore; 2] = [&sqlite, &memory];

    for store in stores {
        store.put(chat_line("line-c", "chat-1", "2026-10-30T12:00:01.000Z")).unwrap();
        store.put(chat_line("line-a", "chat-1", "2026-10-30T12:00:01.000Z")).unwrap();
        store.put(chat_line("line-z", "chat-1", "2026-10-30T12:00:00.000Z")).unwrap();
        store.put(chat_line("line-b", "chat-1", "2026-10-30T12:00:01.000Z")).unwrap();

        let first = store.page_by_chat_id("chat-1", None, Some(2)).unwrap();
        let cursor = ChatLineCursor::from(first.last().unwrap());
        let second = store.page_by_chat_id("chat-1", Some(cursor), Some(2)).unwrap();

        assert_eq!(ids(&first), vec!["line-z", "line-a"]);
        assert_eq!(ids(&second), vec!["line-b", "line-c"]);
    }
}

#[test]
fn both_stores_resume_after_a_cursor_whose_row_was_deleted_between_pages() {
    let (sqlite, memory) = both_stores("deleted_cursor");
    let stores: [&dyn ChatLineStore; 2] = [&sqlite, &memory];

    for store in stores {
        store.put(chat_line("line-d", "chat-1", "2026-10-30T12:00:01.000Z")).unwrap();
        store.put(chat_line("line-c", "chat-1", "2026-10-30T12:00:02.000Z")).unwrap();
        store.put(chat_line("line-b", "chat-1", "2026-10-30T12:00:03.000Z")).unwrap();
        store.put(chat_line("line-a", "chat-1", "2026-10-30T12:00:04.000Z")).unwrap();

        let first = store.page_by_chat_id("chat-1", None, Some(2)).unwrap();
        let cursor = ChatLineCursor::from(first.last().unwrap());

        store.delete("line-c").unwrap();

        let second = store.page_by_chat_id("chat-1", Some(cursor), Some(2)).unwrap();

        assert_eq!(ids(&first), vec!["line-d", "line-c"]);
        assert_eq!(ids(&second), vec!["line-b", "line-a"]);
    }
}

fn every_page(store: &dyn ChatLineStore, account_id: &str, chat_id: &str) -> Vec<Vec<ChatLine>> {
    let mut pages = Vec::new();
    let mut cursor: Option<ChatLineCursor> = None;

    for _ in 0..8 {
        let page = store
            .page_by_account_id_and_chat_id(account_id, chat_id, cursor.clone(), Some(2))
            .unwrap();

        if page.is_empty() {
            return pages;
        }

        cursor = Some(ChatLineCursor::from(page.last().unwrap()));
        pages.push(page);
    }

    panic!("walking the pages of {chat_id} never reached an empty page, so the cursor never moved past a row");
}

#[test]
fn both_stores_answer_the_same_pages_for_the_same_rows_and_cursors() {
    let (sqlite, memory) = both_stores("same_pages");
    let stores: [&dyn ChatLineStore; 2] = [&sqlite, &memory];
    let rows = [
        chat_line("line-f", "chat-1", "2026-10-30T12:00:02.000Z"),
        chat_line("line-e", "chat-1", "2026-10-30T12:00:01.000Z"),
        chat_line("line-d", "chat-1", "2026-10-30T12:00:02.000Z"),
        chat_line("line-c", "chat-1", "2026-10-30T11:59:59.999Z"),
        chat_line("line-b", "chat-1", "2026-10-30T12:00:01.000Z"),
        chat_line("line-a", "chat-1", "2026-10-30T12:00:03.000Z"),
        chat_line("line-g", "chat-2", "2026-10-30T12:00:00.000Z"),
    ];

    for store in stores {
        for row in &rows {
            store.put(row.clone()).unwrap();
        }
    }

    let from_sqlite = every_page(&sqlite, "account-1", "chat-1");
    let from_memory = every_page(&memory, "account-1", "chat-1");
    let walked: Vec<Vec<&str>> = from_sqlite.iter().map(Vec::as_slice).map(ids).collect();

    assert_eq!(from_sqlite, from_memory);
    assert_eq!(
        walked,
        vec![
            vec!["line-c", "line-b"],
            vec!["line-e", "line-d"],
            vec!["line-f", "line-a"],
        ]
    );
}

#[test]
fn a_page_after_the_last_row_answers_an_empty_page_in_both_stores() {
    let (sqlite, memory) = both_stores("after_last");
    let stores: [&dyn ChatLineStore; 2] = [&sqlite, &memory];

    for store in stores {
        store.put(chat_line("line-a", "chat-1", "2026-10-30T12:00:01.000Z")).unwrap();

        let last = chat_line("line-b", "chat-1", "2026-10-30T12:00:02.000Z");

        store.put(last.clone()).unwrap();

        let page = store
            .page_by_chat_id("chat-1", Some(ChatLineCursor::from(&last)), None)
            .unwrap();

        assert_eq!(page, Vec::<ChatLine>::new());
    }
}

#[test]
fn the_cursor_built_from_a_row_carries_its_order_value_and_its_key() {
    let row = chat_line("line-a", "chat-1", "2026-10-30T12:00:01.000Z");

    assert_eq!(
        ChatLineCursor::from(&row),
        ChatLineCursor {
            order: "2026-10-30T12:00:01.000Z".to_string(),
            key: "line-a".to_string(),
        }
    );
}

#[test]
fn both_stores_refuse_a_date_time_order_value_not_in_the_fixed_millisecond_form() {
    let (sqlite, memory) = both_stores("stamp");
    let stores: [(&dyn ChatLineStore, &str); 2] = [(&sqlite, "sqlite"), (&memory, "memory")];

    for (store, kind) in stores {
        for stamp in [
            "2026-10-30T12:00:01Z",
            "2026-10-30T12:00:01.000+00:00",
            "2026-10-30T12:00:01.000000Z",
            "2026-10-30 12:00:01.000Z",
            "2026-10-30T12:00:01.000Z7",
        ] {
            let refused = store.put(chat_line("line-a", "chat-1", stamp)).unwrap_err();

            assert!(matches!(&refused, ChatLineStoreError::Put { id, .. } if id == "line-a"));
            assert_eq!(
                error_chain(&refused),
                format!("putting chat_line \"line-a\": the chat_line {kind} store refuses postedAt \"{stamp}\", a date-time order value is written in the UTC millisecond form YYYY-MM-DDTHH:MM:SS.mmmZ")
            );
        }

        assert_eq!(store.get("line-a").unwrap(), None);
    }
}
`

const orderRustTestCount = "test result: ok. 7 passed"

func TestBothStoreAdaptersReadAPageInTheDeclaredOrderAgainstARealSqliteFile(t *testing.T) {
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Fatalf("finding cargo, which this contract runs through and never skips: %v", err)
	}

	files, err := restrust.Generate([]byte(chatLineSpec), restrust.Options{Service: "songe-social"})
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

	write("Cargo.toml", orderCrateManifest)
	write("src/lib.rs", "pub mod rest;\n")

	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".rs") {
			continue
		}

		write(filepath.Join("src", "rest", f.Path), f.Content)
	}

	write("src/rest/controller/chat_controller.rs", orderChatControllerImpl)
	write("tests/order.rs", orderRustTest)

	mustCargo(t, cargo, root, "clippy", "--all-targets", "--", "-D", "warnings")

	out := mustCargo(t, cargo, root, "test", "--test", "order")
	if !strings.Contains(out, orderRustTestCount) {
		t.Fatalf("the order contract never reported %q, so some of its tests did not run:\n%s", orderRustTestCount, out)
	}
}
