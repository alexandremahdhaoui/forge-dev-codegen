package grpcrust_test

import (
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/grpcrust"
)

func generatedGrpcFile(t *testing.T, path string) string {
	t.Helper()

	files, err := grpcrust.Generate([]byte(helloProto), grpcrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	for _, f := range files {
		if f.Path == path {
			return f.Content
		}
	}

	t.Fatalf("no file at %s", path)

	return ""
}

func TestTheGrpcControllerErrorEnumCarriesTheWholeTaxonomy(t *testing.T) {
	content := generatedGrpcFile(t, "controller/zz_generated_hello_controller.rs")

	for _, want := range []string{
		"    Runtime {\n        operation: String,\n        #[source]\n        source: Box<dyn std::error::Error + Send + Sync>,\n    },",
		"    Authentication { subject: String, reason: String },",
		"    Authorization { subject: String, reason: String },",
		`    #[error("finding hello {id:?}: not found")]`,
		"    NotFound { id: String },",
		"    Invalid { field: String, reason: String },",
		"    Semantic { resource: String, reason: String },",
		"    RateLimited { subject: String, reason: String },",
		"    NotImplemented { operation: String },",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("the controller lacks %q:\n%s", want, content)
		}
	}
}

func TestTheGrpcDriverMapsEveryTaxonomyMemberToItsStatusCodeAndHidesARuntimeFailure(t *testing.T) {
	content := generatedGrpcFile(t, "driver/zz_generated_hello_grpc_driver.rs")

	for _, want := range []string{
		"fn status(error: HelloControllerError) -> tonic::Status {",
		"HelloControllerError::Runtime { .. } => {\n            eprintln!(\"{error:?}\");\n            tonic::Status::new(tonic::Code::Internal, \"internal error\")\n        }",
		"HelloControllerError::Authentication { .. } => tonic::Status::new(tonic::Code::Unauthenticated, error.to_string()),",
		"HelloControllerError::Authorization { .. } => tonic::Status::new(tonic::Code::PermissionDenied, error.to_string()),",
		"HelloControllerError::NotFound { .. } => tonic::Status::new(tonic::Code::NotFound, error.to_string()),",
		"HelloControllerError::Invalid { .. } => tonic::Status::new(tonic::Code::InvalidArgument, error.to_string()),",
		"HelloControllerError::Semantic { .. } => tonic::Status::new(tonic::Code::FailedPrecondition, error.to_string()),",
		"HelloControllerError::RateLimited { .. } => tonic::Status::new(tonic::Code::ResourceExhausted, error.to_string()),",
		"HelloControllerError::NotImplemented { .. } => tonic::Status::new(tonic::Code::Unimplemented, error.to_string()),",
		".map_err(status)?;",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("the driver lacks %q:\n%s", want, content)
		}
	}

	for _, gone := range []string{"tonic::Status::internal(", "tonic::Status::not_found(", "tonic::Status::unauthenticated("} {
		if strings.Contains(content, gone) {
			t.Fatalf("the driver still spells a code through the method %q:\n%s", gone, content)
		}
	}
}

func TestTheGrpcClientErrorCarriesOneArmPerTaxonomyMemberAndNoArmPerRpc(t *testing.T) {
	files, err := grpcrust.Generate([]byte(rosterProto), grpcrust.Options{Service: "songe-roster"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	var content string

	for _, f := range files {
		if f.Path == "port/zz_generated_roster_client.rs" {
			content = f.Content
		}
	}

	for _, want := range []string{
		"pub enum RosterClientError {",
		"    #[error(\"calling {operation:?}: {message}\")]\n    Runtime {\n        operation: String,\n        message: String,\n        #[source]\n        source: Option<Box<dyn std::error::Error + Send + Sync>>,\n    },",
		"    #[error(\"calling {operation:?}: authentication refused: {message}\")]\n    Authentication { operation: String, message: String },",
		"    #[error(\"calling {operation:?}: authorization refused: {message}\")]\n    Authorization { operation: String, message: String },",
		"    #[error(\"calling {operation:?}: {message}\")]\n    NotFound { operation: String, message: String },",
		"    #[error(\"calling {operation:?}: validation failed: {message}\")]\n    Invalid { operation: String, message: String },",
		"    #[error(\"calling {operation:?}: {message}\")]\n    Semantic { operation: String, message: String },",
		"    #[error(\"calling {operation:?}: rate limited: {message}\")]\n    RateLimited { operation: String, message: String },",
		"    #[error(\"calling {operation:?}: {message}\")]\n    NotImplemented { operation: String, message: String },",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("the client error lacks %q:\n%s", want, content)
		}
	}

	for _, gone := range []string{"    List {", "    Forget {", "calling list", "calling forget"} {
		if strings.Contains(content, gone) {
			t.Errorf("the client error still carries an arm per rpc through %q:\n%s", gone, content)
		}
	}
}

func TestTheGrpcClientAdapterDecodesACodeOutsideTheTaxonomyAsRuntimeNamingTheCodeAndKeepingTheStatus(t *testing.T) {
	content := generatedGrpcFile(t, "adapter/zz_generated_hello_grpc_client.rs")

	for _, want := range []string{
		".map_err(|status| decode_status(\"Ping\", status))",
		"fn decode_status(operation: &str, status: tonic::Status) -> HelloClientError {",
		"    let message = status.message().to_string();",
		"        tonic::Code::Internal => HelloClientError::Runtime {\n            operation,\n            message,\n            source: None,\n        },",
		"        tonic::Code::NotFound => HelloClientError::NotFound { operation, message },",
		"        code => HelloClientError::Runtime {\n            operation,\n            message: format!(\"{code:?}: {message}\"),\n            source: Some(Box::new(status)),\n        },",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("the client adapter lacks %q:\n%s", want, content)
		}
	}
}
