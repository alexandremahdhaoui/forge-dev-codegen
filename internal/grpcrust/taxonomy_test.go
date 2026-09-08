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
		"HelloControllerError::Runtime { .. } => {\n            eprintln!(\"{error:?}\");\n            tonic::Status::internal(\"internal error\")\n        }",
		"HelloControllerError::Authentication { .. } => tonic::Status::unauthenticated(error.to_string()),",
		"HelloControllerError::Authorization { .. } => tonic::Status::permission_denied(error.to_string()),",
		"HelloControllerError::NotFound { .. } => tonic::Status::not_found(error.to_string()),",
		"HelloControllerError::Invalid { .. } => tonic::Status::invalid_argument(error.to_string()),",
		"HelloControllerError::Semantic { .. } => tonic::Status::failed_precondition(error.to_string()),",
		"HelloControllerError::RateLimited { .. } => tonic::Status::resource_exhausted(error.to_string()),",
		"HelloControllerError::NotImplemented { .. } => tonic::Status::unimplemented(error.to_string()),",
		".map_err(status)?;",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("the driver lacks %q:\n%s", want, content)
		}
	}

	if strings.Contains(content, "tonic::Status::internal(source.to_string())") {
		t.Fatal("the driver still folds every controller error into the internal status")
	}
}
