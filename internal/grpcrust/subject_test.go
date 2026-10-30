package grpcrust_test

import (
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/grpcrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/layoutports"
	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/taxonomy"
)

func generatedByPath(t *testing.T, opts grpcrust.Options) map[string]string {
	t.Helper()

	files, err := grpcrust.Generate([]byte(helloProto), opts)
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	byPath := map[string]string{}
	for _, f := range files {
		byPath[f.Path] = f.Content
	}

	return byPath
}

func carries(t *testing.T, byPath map[string]string, path string, wants ...string) {
	t.Helper()

	content, ok := byPath[path]
	if !ok {
		t.Fatalf("no file at %s", path)
	}

	for _, want := range wants {
		if !strings.Contains(content, want) {
			t.Errorf("%s lacks %q:\n%s", path, want, content)
		}
	}
}

func TestEveryPortErrorAnswersTheIdentifierItCarriesThroughSubject(t *testing.T) {
	t.Run("client", func(t *testing.T) {
		arms := []string{"Self::" + taxonomy.Runtime("hello").Variant + " { operation, .. }"}

		for _, member := range taxonomy.Detailed("hello") {
			arms = append(arms, "Self::"+member.Variant+" { operation, .. }")
		}

		byPath := generatedByPath(t, grpcrust.Options{Service: "songe-hello"})

		carries(t, byPath, "port/zz_generated_hello_client.rs",
			"impl HelloClientError {\n    pub fn subject(&self) -> String {\n        match self {\n            "+strings.Join(arms, " | ")+" => operation.clone(),\n        }\n    }\n}",
		)
	})
}

func TestAGrpcControllerErrorCarriesOneArmPerHeldPortAndTheDriverAnswersItAsInternal(t *testing.T) {
	byPath := generatedByPath(t, grpcrust.Options{
		Service: "songe-hello",
		Ports:   []layoutports.Spec{{Name: "TicketStore"}, {Name: "GreetingClock"}},
	})

	internal := taxonomy.Runtime("hello").GrpcCode

	for _, port := range []struct{ name, snake string }{{"TicketStore", "ticket_store"}, {"GreetingClock", "greeting_clock"}} {
		carries(t, byPath, "controller/zz_generated_hello_controller.rs",
			"    #[error(\"calling the "+port.snake+" port for {id:?}\")]\n    "+port.name+" {\n        id: String,\n        #[source]\n        source: "+port.name+"Error,\n    },",
			"impl From<"+port.name+"Error> for HelloControllerError {\n    fn from(source: "+port.name+"Error) -> Self {\n        Self::"+port.name+" {\n            id: source.subject(),\n            source,\n        }\n    }\n}",
		)

		carries(t, byPath, "driver/zz_generated_hello_grpc_driver.rs",
			"HelloControllerError::"+port.name+" { .. } => {\n            eprintln!(\"{error:?}\");\n            tonic::Status::new(tonic::Code::"+internal+", \"internal error\")\n        }",
		)
	}
}
