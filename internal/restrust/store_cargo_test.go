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

const profileSpec = `
openapi: 3.1.0
info:
  title: Profile API
  version: 1.0.0
paths:
  /profiles/{accountId}:
    get:
      operationId: getProfile
      x-controller: profile
      x-ports: [ProfileStore]
      parameters:
        - name: accountId
          in: path
          required: true
          schema:
            type: string
      responses:
        "200":
          description: The profile
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Profile"
components:
  schemas:
    Profile:
      type: object
      x-store:
        key: accountId
        lookups:
          - { by: [alias, tag], answers: one }
        adapters: [sqlite, memory]
      required: [accountId, alias, tag]
      properties:
        accountId:
          type: string
        alias:
          type: string
        tag:
          type: string
`

const profileCrateManifest = `[package]
name = "songe-identity"
version = "0.1.0"
edition = "2021"

[dependencies]
axum = "0.8"
rusqlite = { version = "0.40", features = ["bundled"] }
serde = { version = "1", features = ["derive"] }
serde_json = "1"
thiserror = "2"
tokio = { version = "1", features = ["full"] }

[dev-dependencies]
mockall = "0.15"
`

const profileCrateLib = `pub mod rest;
`

const profileControllerImpl = `use crate::rest::controller::{
    ProfileController, ProfileControllerError, ProfileControllerImpl,
};
use crate::rest::port::profile_store::ProfileStore;
use crate::rest::types::profile::Profile;

impl ProfileController for ProfileControllerImpl {
    fn get_profile(&self, account_id: &str) -> Result<Profile, ProfileControllerError> {
        self.profile_store
            .get(account_id)
            .map_err(|source| ProfileControllerError::ProfileStore {
                id: account_id.to_string(),
                source,
            })?
            .ok_or(ProfileControllerError::NotFound {
                id: account_id.to_string(),
            })
    }
}
`

const profileStoreTest = `use songe_identity::rest::adapter::profile_memory::{
    ProfileMemoryStore, ProfileMemoryStoreConfig,
};
use songe_identity::rest::adapter::profile_sqlite::{ProfileSqliteStore, ProfileSqliteStoreConfig};
use songe_identity::rest::port::profile_store::ProfileStore;
use songe_identity::rest::types::profile::Profile;

fn profile(account_id: &str, alias: &str, tag: &str) -> Profile {
    Profile {
        account_id: account_id.to_string(),
        alias: alias.to_string(),
        tag: tag.to_string(),
    }
}

fn sqlite_path(name: &str) -> std::path::PathBuf {
    let path = std::path::Path::new(env!("CARGO_TARGET_TMPDIR")).join(format!("{name}.db"));
    let _ = std::fs::remove_file(&path);

    path
}

fn sqlite_store_at(path: &std::path::Path) -> ProfileSqliteStore {
    ProfileSqliteStore::new(ProfileSqliteStoreConfig {
        path: path.display().to_string(),
    })
    .unwrap()
}

fn sqlite_store(name: &str) -> ProfileSqliteStore {
    sqlite_store_at(&sqlite_path(name))
}

fn memory_store() -> ProfileMemoryStore {
    ProfileMemoryStore::new(ProfileMemoryStoreConfig { capacity: 8 }).unwrap()
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

fn audit_rows(path: &std::path::Path, op: &str) -> Vec<(String, Option<String>, Option<String>)> {
    let connection = rusqlite::Connection::open(path).unwrap();
    let mut statement = connection
        .prepare("SELECT key, before, after FROM audit WHERE op = ?1")
        .unwrap();

    statement
        .query_map([op], |row| Ok((row.get(0)?, row.get(1)?, row.get(2)?)))
        .unwrap()
        .collect::<Result<Vec<_>, _>>()
        .unwrap()
}

#[test]
fn both_stores_delete_a_held_row_and_a_later_get_answers_none() {
    let sqlite = sqlite_store("delete_held");
    let memory = memory_store();
    let stores: [&dyn ProfileStore; 2] = [&sqlite, &memory];

    for store in stores {
        store.put(profile("account-1", "kay", "0001")).unwrap();
        store.delete("account-1").unwrap();

        assert_eq!(store.get("account-1").unwrap(), None);
    }
}

#[test]
fn both_stores_refuse_to_delete_a_key_that_holds_no_row_and_name_the_missing_key() {
    let sqlite = sqlite_store("delete_missing");

    assert_eq!(
        error_chain(&sqlite.delete("account-9").unwrap_err()),
        "deleting profile \"account-9\": the profile sqlite store holds no row under accountId \"account-9\""
    );

    let memory = memory_store();

    assert_eq!(
        error_chain(&memory.delete("account-9").unwrap_err()),
        "deleting profile \"account-9\": the profile memory store holds no row under accountId \"account-9\""
    );
}

#[test]
fn the_sqlite_store_writes_one_audit_row_with_op_delete_holding_the_deleted_row_as_before() {
    let path = sqlite_path("delete_audit");
    let store = sqlite_store_at(&path);
    let held = profile("account-1", "kay", "0001");

    store.put(held.clone()).unwrap();
    store.delete("account-1").unwrap();

    assert_eq!(
        audit_rows(&path, "delete"),
        vec![(
            "account-1".to_string(),
            Some(serde_json::to_string(&held).unwrap()),
            None
        )]
    );
}

#[test]
fn a_lookup_that_answers_one_no_longer_answers_a_deleted_row_in_either_store() {
    let sqlite = sqlite_store("delete_lookup");
    let memory = memory_store();
    let stores: [&dyn ProfileStore; 2] = [&sqlite, &memory];

    for store in stores {
        store.put(profile("account-1", "kay", "0001")).unwrap();
        store.delete("account-1").unwrap();

        assert_eq!(store.get_by_alias_and_tag("kay", "0001").unwrap(), None);
    }
}

#[test]
fn a_sqlite_store_keyed_on_a_property_other_than_id_reads_back_the_row_it_wrote() {
    let store = sqlite_store("keyed");

    store.put(profile("account-1", "kay", "0001")).unwrap();

    assert_eq!(
        store.get("account-1").unwrap(),
        Some(profile("account-1", "kay", "0001"))
    );
}

#[test]
fn a_sqlite_store_finds_a_row_by_every_field_of_a_composite_lookup() {
    let store = sqlite_store("composite");

    store.put(profile("account-1", "kay", "0001")).unwrap();
    store.put(profile("account-2", "kay", "0002")).unwrap();

    assert_eq!(
        store.get_by_alias_and_tag("kay", "0002").unwrap(),
        Some(profile("account-2", "kay", "0002"))
    );
    assert_eq!(store.get_by_alias_and_tag("kay", "0003").unwrap(), None);
}

#[test]
fn both_stores_refuse_a_second_key_holding_the_values_of_a_lookup_that_answers_one() {
    let sqlite = sqlite_store("duplicate");

    sqlite.put(profile("account-1", "kay", "0001")).unwrap();

    assert!(sqlite.put(profile("account-2", "kay", "0001")).is_err());

    let memory = ProfileMemoryStore::new(ProfileMemoryStoreConfig { capacity: 8 }).unwrap();

    memory.put(profile("account-1", "kay", "0001")).unwrap();

    assert!(memory.put(profile("account-2", "kay", "0001")).is_err());
}

#[test]
fn both_stores_accept_a_row_that_overwrites_itself_under_its_own_key() {
    let sqlite = sqlite_store("overwrite");

    sqlite.put(profile("account-1", "kay", "0001")).unwrap();
    sqlite.put(profile("account-1", "kay", "0001")).unwrap();

    let memory = ProfileMemoryStore::new(ProfileMemoryStoreConfig { capacity: 8 }).unwrap();

    memory.put(profile("account-1", "kay", "0001")).unwrap();
    memory.put(profile("account-1", "kay", "0001")).unwrap();
}
`

func TestBothStoreAdaptersHoldTheirContractAgainstARealSqliteFile(t *testing.T) {
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Skip("cargo is not on PATH")
	}

	files, err := restrust.Generate([]byte(profileSpec), restrust.Options{Service: "songe-identity"})
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

	write("Cargo.toml", profileCrateManifest)
	write("src/lib.rs", profileCrateLib)

	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".rs") {
			continue
		}

		write(filepath.Join("src", "rest", f.Path), f.Content)
	}

	write("src/rest/controller/profile_controller.rs", profileControllerImpl)
	write("tests/store.rs", profileStoreTest)

	runCargo(t, cargo, root, "test", "--test", "store")
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
		t.Skipf("cargo %s needs network access to crates.io, which this run did not have: %v\n%s", args[0], err, out)
	}

	t.Fatalf("cargo %s: %v\n%s", strings.Join(args, " "), err, out)
}
