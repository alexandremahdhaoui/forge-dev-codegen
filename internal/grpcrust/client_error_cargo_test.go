package grpcrust_test

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/grpcrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/taxonomy"
)

const everyMemberTestName = "a_generated_client_gets_back_the_arm_the_controller_refused_with_for_every_member"

func everyMemberControllerImpl() string {
	var arms strings.Builder

	for _, m := range taxonomy.Detailed("hello") {
		fmt.Fprintf(&arms, "            %q => Err(%s),\n", m.Variant, m.Literal("HelloControllerError", "subject", "reason"))
	}

	return `use crate::grpc::controller::{HelloController, HelloControllerError, HelloControllerImpl};
use crate::grpc::types::hello_messages::{PingReply, PingRequest};

impl HelloController for HelloControllerImpl {
    fn ping(&self, request: PingRequest) -> Result<PingReply, HelloControllerError> {
        match request.message.as_str() {
            "Runtime" => Err(HelloControllerError::Runtime {
                operation: "ping".to_string(),
                source: Box::new(std::io::Error::other("the disk is gone")),
            }),
` + arms.String() + `            _ => Ok(PingReply {
                message: request.message,
                count: request.count,
            }),
        }
    }
}
`
}

func everyMemberRustTest() string {
	var checks strings.Builder

	runtime := taxonomy.Runtime("hello")

	fmt.Fprintf(&checks, `
    let error = refuse(%q);
    assert!(
        matches!(error, HelloClientError::%s { ref operation, ref message, source: None } if operation == "Ping" && message == "internal error"),
        "the controller failed with %s and the client answered {error:?}"
    );
`, runtime.Variant, runtime.Variant, runtime.Variant)

	for _, m := range taxonomy.Detailed("hello") {
		fmt.Fprintf(&checks, `
    let error = refuse(%q);
    assert!(
        matches!(error, HelloClientError::%s { ref operation, .. } if operation == "Ping"),
        "the controller refused with %s and the client answered {error:?}"
    );
`, m.Variant, m.Variant, m.Variant)
	}

	return `use std::sync::Arc;

use songe_hello::grpc::adapter::hello_grpc_client::{HelloGrpcClient, HelloGrpcClientConfig};
use songe_hello::grpc::controller::{HelloControllerImpl, HelloControllerPorts};
use songe_hello::grpc::driver::hello_grpc_driver::{HelloGrpcDriver, HelloGrpcDriverConfig};
use songe_hello::grpc::port::hello_client::{HelloClient, HelloClientError};
use songe_hello::grpc::types::hello_messages::PingRequest;

#[tokio::test(flavor = "multi_thread")]
async fn ` + everyMemberTestName + `() {
    let mut driver = HelloGrpcDriver::new(
        HelloGrpcDriverConfig {
            addr: "127.0.0.1:0".to_string(),
        },
        Arc::new(HelloControllerImpl::new(HelloControllerPorts {})),
    );

    driver.bind().await.expect("a bound grpc socket");

    let port = driver.local_port().expect("a bound grpc port");

    tokio::spawn(async move {
        if let Err(error) = driver.serve().await {
            eprintln!("serving HelloGrpcDriver: {error}");
        }
    });

    let client = HelloGrpcClient::new(HelloGrpcClientConfig {
        endpoint: format!("http://127.0.0.1:{port}"),
    })
    .expect("a grpc client");

    let refuse = |member: &str| {
        client
            .ping(PingRequest {
                message: member.to_string(),
                count: 0,
            })
            .expect_err("a refused call")
    };
` + checks.String() + `}
`
}

func TestAGeneratedClientCallingAGeneratedDriverGetsBackTheArmTheControllerRefusedWithForEveryMember(t *testing.T) {
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Fatalf("this proof runs cargo and never skips, and cargo is not on PATH: %v", err)
	}

	files, err := grpcrust.Generate([]byte(helloProto), grpcrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating the cell: %v", err)
	}

	root := t.TempDir()
	write := writerUnder(t, root)

	write("Cargo.toml", cargoCheckCrateManifest)
	write("src/lib.rs", cargoCheckCellLib)
	write("build.rs", cargoCheckBuildScript)

	for _, f := range files {
		if strings.HasSuffix(f.Path, ".yaml") {
			continue
		}

		write(filepath.Join("src", "grpc", f.Path), f.Content)
	}

	write("src/grpc/controller/hello_controller.rs", everyMemberControllerImpl())
	write("tests/every_member.rs", everyMemberRustTest())

	cmd := exec.Command(cargo, "test", "--test", "every_member")
	cmd.Dir = root

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cargo test: %v\n%s", err, out)
	}

	if !strings.Contains(string(out), everyMemberTestName+" ... ok") {
		t.Fatalf("cargo test never reported %s passing\n%s", everyMemberTestName, out)
	}
}
