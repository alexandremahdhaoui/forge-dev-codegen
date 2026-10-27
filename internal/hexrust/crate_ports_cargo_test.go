package hexrust_test

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

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
    pub sqlite_path: String,
    pub ticket_verifier: String,
    pub ticket_verifier_secret_secret: String,
    pub ticket_verifier_secret_id: String,
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
            sqlite_path: ":memory:".to_string(),
            ticket_verifier: "ticket_verifier_secret".to_string(),
            ticket_verifier_secret_secret: "open".to_string(),
            ticket_verifier_secret_id: "friend".to_string(),
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

func standUpTwoRestCellCrate(t *testing.T) (string, string) {
	t.Helper()

	root := t.TempDir()
	write := writeUnder(t, root)

	writeRestCell(t, root, "rest", guardedHelloSpec, restrust.SideBoth, true)
	writeRestCell(t, root, "account", accountSpec, restrust.SideClient, true)

	files, err := hexrust.Generate(hexrust.Options{
		Service: "songe-hello",
		SrcDir:  root,
		Cells:   []string{"account", "rest"},
		Ports:   map[string][]string{"TicketVerifier": {"secret"}},
		Wiring: []byte(`binary: songe-hello-node
drivers:
  rest: { enabled: true }
`),
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	spec := ""

	for _, f := range files {
		if f.Path == hexrust.ConfigSpecFile {
			spec = f.Content
		}

		if strings.HasSuffix(f.Path, ".rs") {
			write(f.Path, f.Content)
		}
	}

	write("Cargo.toml", strings.ReplaceAll(guardedCrateManifest, "SONGE_COMMON_DIR", songeCommonDir(t)))
	write("src/rest/controller/greeting_controller.rs", guardedControllerImpl)

	return root, spec
}

func TestTwoRestCellsShareTheCrateRootPortsAndTheCrateCompiles(t *testing.T) {
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Skip("cargo is not on PATH")
	}

	root, _ := standUpTwoRestCellCrate(t)
	writeUnder(t, root)("src/config/zz_generated_config.rs", guardedConfigLoader)

	out, err := runCargo(t, cargo, root, "check", "--workspace", "--all-targets")
	if err != nil {
		skipOnNetwork(t, out, err)
		t.Fatalf("cargo check: %v\n%s", err, out)
	}
}

const configgenGen = "github.com/alexandremahdhaoui/golden-configgen/cmd/configgen-gen"

func generateTheConfigLoader(t *testing.T, spec string) string {
	t.Helper()

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "hexrust-test", Version: "0.0.0"}, nil)

	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: exec.Command("go", "run", configgenGen, "--mcp")}, nil)
	if err != nil {
		t.Fatalf("starting %s the way forge-dev starts a config generator: %v", configgenGen, err)
	}
	defer func() { _ = session.Close() }()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "generate",
		Arguments: map[string]any{
			"name":        "songe-hello",
			"kind":        "hexagonal",
			"language":    "rust",
			"packageName": "main",
			"openapiSpec": spec,
		},
	})
	if err != nil {
		t.Fatalf("calling generate on %s: %v", configgenGen, err)
	}

	if result.IsError {
		t.Fatalf("%s refused the config spec: %v\n%s", configgenGen, result.Content, spec)
	}

	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("encoding the answer of %s: %v", configgenGen, err)
	}

	var answer struct {
		Files []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"files"`
	}

	if err := json.Unmarshal(encoded, &answer); err != nil {
		t.Fatalf("decoding the answer of %s: %v\n%s", configgenGen, err, encoded)
	}

	if len(answer.Files) != 1 || answer.Files[0].Path != "zz_generated_config.rs" {
		t.Fatalf("%s answered %s, want the one file zz_generated_config.rs", configgenGen, encoded)
	}

	return answer.Files[0].Content
}

const requiredWhenTest = `use songe_hello::config::SongeHelloConfig;

fn flags(ticket_verifier: &str) -> Vec<String> {
    vec!["--ticket-verifier".to_string(), ticket_verifier.to_string()]
}

#[test]
fn the_loader_refuses_a_waiting_key_by_name_while_the_choice_names_its_adapter() {
    let error = SongeHelloConfig::load(&flags("ticket_verifier_secret")).unwrap_err();

    assert_eq!(
        error.to_string(),
        "resolving ticket_verifier_secret_id when ticket_verifier is ticket_verifier_secret"
    );
}

#[test]
fn the_loader_loads_with_every_waiting_key_unset_while_the_choice_names_another_adapter() {
    let config = SongeHelloConfig::load(&flags("ticket_verifier_jwks")).unwrap();

    assert_eq!(config.ticket_verifier, "ticket_verifier_jwks");
    assert_eq!(config.ticket_verifier_secret_id, "");
    assert_eq!(config.ticket_verifier_secret_secret, "");
}
`

func TestTheGeneratedRustLoaderRefusesAWaitingKeyOnlyWhileItsPortChoiceNamesItsAdapter(t *testing.T) {
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Fatalf("this proof runs cargo and never skips, and cargo is not on PATH: %v", err)
	}

	root, spec := standUpTwoRestCellCrate(t)
	write := writeUnder(t, root)

	write("src/config/zz_generated_config.rs", generateTheConfigLoader(t, spec))
	write("tests/required_when.rs", requiredWhenTest)

	out, err := runCargo(t, cargo, root, "test", "--test", "required_when")
	if err != nil {
		t.Fatalf("cargo test: %v\n%s", err, out)
	}

	if !strings.Contains(out, "test result: ok. 2 passed") {
		t.Fatalf("cargo test did not run both loader proofs\n%s", out)
	}
}
