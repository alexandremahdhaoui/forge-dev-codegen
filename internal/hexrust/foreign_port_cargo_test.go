package hexrust_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/hexrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/restrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/cellmanifest"
)

const foreignPortWiring = `binary: songe-hello-node
ports:
  GreetingStore:
    default: sqlite
drivers:
  rest: { enabled: true }
  grpc: { enabled: true }
`

const foreignPortControllerImpl = `use crate::grpc::types::hello_messages::PingRequest;
use crate::rest::controller::{
    GreetingController, GreetingControllerError, GreetingControllerImpl,
};
use crate::rest::types::greeting::Greeting;

impl GreetingController for GreetingControllerImpl {
    fn create_greeting(&self, body: Greeting) -> Result<Greeting, GreetingControllerError> {
        let reply = self
            .hello_client
            .ping(PingRequest {
                message: body.name.clone(),
            })
            .map_err(|source| GreetingControllerError::HelloClient {
                id: body.id.clone(),
                source,
            })?;

        self.greeting_store
            .put(body.clone())
            .map_err(|source| GreetingControllerError::GreetingStore {
                id: body.id.clone(),
                source,
            })?;

        Ok(Greeting {
            id: body.id,
            name: reply.message,
        })
    }
}
`

const foreignPortConfigLoader = `#[derive(Debug)]
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
    pub hello_client: String,
    pub hello_client_grpc_client_endpoint: String,
    pub driver_grpc: bool,
    pub driver_rest: bool,
    pub grpc_addr: String,
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
            hello_client: "grpc_client".to_string(),
            hello_client_grpc_client_endpoint: "http://127.0.0.1:50051".to_string(),
            driver_grpc: true,
            driver_rest: true,
            grpc_addr: "127.0.0.1:0".to_string(),
            rest_addr: "127.0.0.1:0".to_string(),
        })
    }
}
`

func TestARestControllerHoldingTheGrpcCellsClientPortCompilesAndPassesClippy(t *testing.T) {
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Skip("cargo is not on PATH")
	}

	root := t.TempDir()
	write := writeUnder(t, root)

	foreignSpec := strings.Replace(helloSpec, "x-ports: [GreetingStore]", "x-ports: [GreetingStore, HelloClient]", 1)
	writeRestCell(t, root, "rest", foreignSpec, restrust.SideServer, true)

	write(filepath.Join("src", "grpc", hexrust.CellConfigFile), "name: songe-hello\nkind: grpc\n")

	for _, f := range cellFiles(t, "grpc") {
		write(filepath.Join("src", "grpc", f.Path), f.Content)
	}

	raw, err := os.ReadFile(filepath.Join(root, "src", "rest", cellmanifest.FileName))
	if err != nil {
		t.Fatalf("reading the rest manifest: %v", err)
	}

	manifest, err := cellmanifest.Parse(raw)
	if err != nil {
		t.Fatalf("parsing the rest manifest: %v", err)
	}

	if !strings.Contains(strings.Join(manifest.Requires.Ports, ","), "HelloClient") {
		t.Fatalf("the rest cell requires %q, want HelloClient among them", manifest.Requires.Ports)
	}

	for path, content := range generateHello(t, root, foreignPortWiring, "grpc", "rest") {
		if strings.HasSuffix(path, ".rs") {
			write(path, content)
		}
	}

	write("Cargo.toml", strings.ReplaceAll(nodeCrateManifest, "SONGE_COMMON_DIR", songeCommonDir(t)))
	write("src/config/zz_generated_config.rs", foreignPortConfigLoader)
	write("src/rest/controller/greeting_controller.rs", foreignPortControllerImpl)
	write("src/grpc/controller/hello_controller.rs", nodeHelloControllerImpl)

	out, err := runCargo(t, cargo, root, "clippy", "--workspace", "--all-targets", "--", "-D", "warnings")
	if err != nil {
		skipOnNetwork(t, out, err)
		t.Fatalf("cargo clippy: %v\n%s", err, out)
	}
}
