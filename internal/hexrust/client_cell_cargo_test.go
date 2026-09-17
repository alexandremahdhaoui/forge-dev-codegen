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

const authzProto = `syntax = "proto3";

package songe.authz.v1;

service Authz {
  rpc Relate(RelateRequest) returns (RelateReply);
}

message RelateRequest {
  string subject = 1;
  string relation = 2;
  string object = 3;
}

message RelateReply {
  bool written = 1;
}
`

const clientCellWiring = `binary: songe-hello-node
ports:
  GreetingStore:
    default: sqlite
drivers:
  rest: { enabled: true }
  grpc: { enabled: true }
`

const clientCellControllerImpl = `use crate::authz_client::types::authz_messages::RelateRequest;
use crate::identity_client::types::identity_messages::ResolveAliasRequest;
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

        self.authz_client
            .relate(RelateRequest {
                subject: resolved.account_id.clone(),
                relation: "greeter".to_string(),
                object: body.id.clone(),
            })
            .map_err(|source| GreetingControllerError::AuthzClient {
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
    pub authz_client: String,
    pub authz_client_authz_client_client_endpoint: String,
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
            identity_client: "identity_client_client".to_string(),
            identity_client_identity_client_client_endpoint: "http://127.0.0.1:50051".to_string(),
            authz_client: "authz_client_client".to_string(),
            authz_client_authz_client_client_endpoint: "http://127.0.0.1:50052".to_string(),
            driver_grpc: true,
            driver_rest: true,
            grpc_addr: "127.0.0.1:0".to_string(),
            rest_addr: "127.0.0.1:0".to_string(),
        })
    }
}
`

func writeGrpcCell(t *testing.T, root, cell, proto, side string) {
	t.Helper()

	write := writeUnder(t, root)

	files, err := grpcrust.Generate([]byte(proto), grpcrust.Options{Service: "songe-hello", Cell: cell, Side: side})
	if err != nil {
		t.Fatalf("generating the %s cell: %v", cell, err)
	}

	write(filepath.Join("src", cell, hexrust.CellConfigFile), "name: songe-hello\nkind: grpc\n")

	for _, f := range files {
		write(filepath.Join("src", cell, f.Path), f.Content)
	}
}

func TestARestControllerCallingTwoClientCellsBesideTheCratesOwnGrpcServerCellCompilesAndPassesClippy(t *testing.T) {
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Skip("cargo is not on PATH")
	}

	root := t.TempDir()
	write := writeUnder(t, root)

	foreignSpec := strings.Replace(helloSpec, "x-ports: [GreetingStore]", "x-ports: [GreetingStore, IdentityClient, AuthzClient]", 1)
	writeRestCell(t, root, "rest", foreignSpec, restrust.SideServer, true)
	writeGrpcCell(t, root, "identity_client", identityProto, grpcrust.SideClient)
	writeGrpcCell(t, root, "authz_client", authzProto, grpcrust.SideClient)
	writeGrpcCell(t, root, "grpc", helloGrpcProto, grpcrust.SideServer)

	generated := generateHello(t, root, clientCellWiring, "authz_client", "grpc", "identity_client", "rest")

	configSpec := generated["zz_generated_config_spec.yaml"]
	for _, key := range []string{"identity_client_identity_client_client_endpoint", "authz_client_authz_client_client_endpoint"} {
		if !strings.Contains(configSpec, key) {
			t.Fatalf("the config spec never carried %q:\n%s", key, configSpec)
		}
	}

	build := generated["zz_generated_build.rs"]
	for _, call := range []string{"build_authz_client()", "build_grpc()", "build_identity_client()"} {
		if !strings.Contains(build, call) {
			t.Fatalf("the crate root build script never calls %s:\n%s", call, build)
		}
	}

	for path, content := range generated {
		if strings.HasSuffix(path, ".rs") {
			write(path, content)
		}
	}

	write("Cargo.toml", strings.ReplaceAll(nodeCrateManifest, "SONGE_COMMON_DIR", songeCommonDir(t)))
	write("src/config/zz_generated_config.rs", clientCellConfigLoader)
	write("src/rest/controller/greeting_controller.rs", clientCellControllerImpl)
	write("src/grpc/controller/hello_controller.rs", nodeHelloControllerImpl)

	out, err := runCargo(t, cargo, root, "clippy", "--workspace", "--all-targets", "--", "-D", "warnings")
	if err != nil {
		skipOnNetwork(t, out, err)
		t.Fatalf("cargo clippy: %v\n%s", err, out)
	}
}
