package hexrust_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/grpcrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/hexrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/restrust"
)

const identityProto = `syntax = "proto3";

package songe.identity.v1;

service Identity {
  rpc ResolveAlias(ResolveAliasRequest) returns (ResolveAliasReply);
}

message ResolveAliasRequest {
  string alias = 1;
}

message ResolveAliasReply {
  string account_id = 1;
}
`

const clientCellWiring = `binary: songe-hello-node
ports:
  GreetingStore:
    default: sqlite
drivers:
  rest: { enabled: true }
`

const clientCellControllerImpl = `use crate::identity_client::types::identity_messages::ResolveAliasRequest;
use crate::rest::controller::{
    GreetingController, GreetingControllerError, GreetingControllerImpl,
};
use crate::rest::types::greeting::Greeting;

impl GreetingController for GreetingControllerImpl {
    fn create_greeting(&self, body: Greeting) -> Result<Greeting, GreetingControllerError> {
        let resolved = self
            .identity_client
            .resolve_alias(ResolveAliasRequest {
                alias: body.name.clone(),
            })
            .map_err(|source| GreetingControllerError::IdentityClient {
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
            id: resolved.account_id,
            name: body.name,
        })
    }
}
`

const clientCellConfigLoader = `#[derive(Debug)]
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
    pub identity_client: String,
    pub identity_client_identity_client_client_endpoint: String,
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
            identity_client: "identity_client_client".to_string(),
            identity_client_identity_client_client_endpoint: "http://127.0.0.1:50051".to_string(),
            driver_rest: true,
            rest_addr: "127.0.0.1:0".to_string(),
        })
    }
}
`

func TestARestControllerCallingAClientCellNamedAfterTheForeignServiceCompilesAndPassesClippy(t *testing.T) {
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Skip("cargo is not on PATH")
	}

	root := t.TempDir()
	write := writeUnder(t, root)

	foreignSpec := strings.Replace(helloSpec, "x-ports: [GreetingStore]", "x-ports: [GreetingStore, IdentityClient]", 1)
	writeRestCell(t, root, "rest", foreignSpec, restrust.SideServer, true)

	clientFiles, err := grpcrust.Generate([]byte(identityProto), grpcrust.Options{
		Service: "songe-hello",
		Cell:    "identity_client",
		Side:    grpcrust.SideClient,
	})
	if err != nil {
		t.Fatalf("generating the identity client cell: %v", err)
	}

	write(filepath.Join("src", "identity_client", hexrust.CellConfigFile), "name: songe-hello\nkind: grpc\n")

	for _, f := range clientFiles {
		write(filepath.Join("src", "identity_client", f.Path), f.Content)
	}

	generated := generateHello(t, root, clientCellWiring, "identity_client", "rest")

	configSpec := generated["zz_generated_config_spec.yaml"]
	if !strings.Contains(configSpec, "identity_client_identity_client_client_endpoint") {
		t.Fatalf("the config spec never carried the endpoint key of the identity_client cell:\n%s", configSpec)
	}

	for path, content := range generated {
		if strings.HasSuffix(path, ".rs") {
			write(path, content)
		}
	}

	write("Cargo.toml", strings.ReplaceAll(nodeCrateManifest, "SONGE_COMMON_DIR", songeCommonDir(t)))
	write("src/config/zz_generated_config.rs", clientCellConfigLoader)
	write("src/rest/controller/greeting_controller.rs", clientCellControllerImpl)

	out, err := runCargo(t, cargo, root, "clippy", "--workspace", "--all-targets", "--", "-D", "warnings")
	if err != nil {
		skipOnNetwork(t, out, err)
		t.Fatalf("cargo clippy: %v\n%s", err, out)
	}
}
