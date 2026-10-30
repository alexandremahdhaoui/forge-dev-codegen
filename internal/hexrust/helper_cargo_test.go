package hexrust_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/grpcrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/hexrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/layoutports"
	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/udprust"
)

const sharedHelper = `use crate::port::greeting_store::{GreetingStore, GreetingStoreError};
use crate::rest::types::greeting::Greeting;

pub fn stored_greeting(store: &dyn GreetingStore, id: &str) -> Result<Option<Greeting>, GreetingStoreError> {
    store.get(id)
}
`

const helperRestControllerImpl = `use crate::controller::greeting_lookup::stored_greeting;
use crate::rest::controller::{
    GreetingController, GreetingControllerError, GreetingControllerImpl,
};
use crate::rest::types::greeting::Greeting;

impl GreetingController for GreetingControllerImpl {
    fn create_greeting(&self, body: Greeting) -> Result<Greeting, GreetingControllerError> {
        let held = stored_greeting(&*self.greeting_store, &body.id)?;

        Ok(held.unwrap_or(body))
    }
}
`

const helperGrpcControllerImpl = `use crate::controller::greeting_lookup::stored_greeting;
use crate::grpc::controller::{HelloController, HelloControllerError, HelloControllerImpl};
use crate::grpc::types::hello_messages::{PingReply, PingRequest};

impl HelloController for HelloControllerImpl {
    fn ping(&self, request: PingRequest) -> Result<PingReply, HelloControllerError> {
        let held = stored_greeting(&*self.greeting_store, &request.message)?;

        Ok(PingReply {
            message: held.map_or(request.message, |greeting| greeting.name),
        })
    }
}
`

const helperUdpControllerImpl = `use crate::controller::greeting_lookup::stored_greeting;
use crate::udp::controller::{
    HelloDatagramController, HelloDatagramControllerError, HelloDatagramControllerImpl,
};
use crate::udp::types::context::Context;
use crate::udp::types::hello_datagram_messages::Echo;

impl HelloDatagramController for HelloDatagramControllerImpl {
    fn echo(&self, request: Echo, context: &Context) -> Result<Echo, HelloDatagramControllerError> {
        let _ = context;

        let held = stored_greeting(&*self.greeting_store, &request.payload)?;

        Ok(Echo {
            payload: held.map_or(request.payload, |greeting| greeting.name),
            count: request.count + 1,
        })
    }
}
`

const helperConfigLoader = `#[derive(Debug)]
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
    pub driver_grpc: bool,
    pub driver_rest: bool,
    pub driver_udp: bool,
    pub grpc_addr: String,
    pub rest_addr: String,
    pub udp_addr: String,
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
            driver_grpc: true,
            driver_rest: true,
            driver_udp: true,
            grpc_addr: "127.0.0.1:0".to_string(),
            rest_addr: "127.0.0.1:0".to_string(),
            udp_addr: "127.0.0.1:0".to_string(),
        })
    }
}
`

func TestAHelperReturningAPortErrorIsCalledWithQuestionMarkFromARestAGrpcAndAUdpControllerAndPassesClippy(t *testing.T) {
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Fatalf("this proof runs cargo clippy and never skips, and cargo is not on PATH: %v", err)
	}

	root := t.TempDir()
	write := writeUnder(t, root)

	writeRestCell(t, root, "rest", helloSpec, "", true)

	grpcFiles, err := grpcrust.Generate([]byte(helloGrpcProto), grpcrust.Options{
		Service: "songe-hello",
		Ports:   []layoutports.Spec{{Name: "GreetingStore"}},
	})
	if err != nil {
		t.Fatalf("generating the grpc cell: %v", err)
	}

	write(filepath.Join("src", "grpc", hexrust.CellConfigFile), "name: songe-hello\nkind: grpc\n")

	for _, f := range grpcFiles {
		write(filepath.Join("src", "grpc", f.Path), f.Content)
	}

	udpFiles, err := udprust.Generate([]byte(helloUdpProto), udprust.Options{
		Service: "songe-hello",
		Ports:   []udprust.PortSpec{{Name: "GreetingStore"}},
	})
	if err != nil {
		t.Fatalf("generating the udp cell: %v", err)
	}

	write(filepath.Join("src", "udp", hexrust.CellConfigFile), "name: songe-hello\nkind: udp\n")

	for _, f := range udpFiles {
		write(filepath.Join("src", "udp", f.Path), f.Content)
	}

	files, err := hexrust.Generate(hexrust.Options{
		Service: "songe-hello",
		SrcDir:  root,
		Cells:   []string{"grpc", "rest", "udp"},
		Wiring:  []byte(helloWiring),
		Helpers: []string{"greeting_lookup"},
	})
	if err != nil {
		t.Fatalf("generating the crate root: %v", err)
	}

	for _, f := range files {
		if strings.HasSuffix(f.Path, ".rs") {
			write(f.Path, f.Content)
		}
	}

	write("Cargo.toml", strings.ReplaceAll(nodeCrateManifest, "SONGE_COMMON_DIR", songeCommonDir(t)))
	write("src/config/zz_generated_config.rs", helperConfigLoader)
	write("src/controller/greeting_lookup.rs", sharedHelper)
	write("src/rest/controller/greeting_controller.rs", helperRestControllerImpl)
	write("src/grpc/controller/hello_controller.rs", helperGrpcControllerImpl)
	write("src/udp/controller/hello_datagram_controller.rs", helperUdpControllerImpl)

	out, err := runCargo(t, cargo, root, "clippy", "--workspace", "--all-targets", "--", "-D", "warnings")
	if err != nil {
		t.Fatalf("cargo clippy refused the crate, and a crates.io network failure fails this proof too, it never skips: %v\n%s", err, out)
	}
}
