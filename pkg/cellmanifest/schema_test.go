package cellmanifest_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/cellmanifest"
	"github.com/google/jsonschema-go/jsonschema"
	"sigs.k8s.io/yaml"
)

func resolvedSchema(t *testing.T) *jsonschema.Resolved {
	t.Helper()

	var s jsonschema.Schema
	if err := json.Unmarshal(cellmanifest.Schema(), &s); err != nil {
		t.Fatalf("unmarshalling the embedded schema returned %v", err)
	}

	resolved, err := s.Resolve(nil)
	if err != nil {
		t.Fatalf("resolving the embedded schema returned %v", err)
	}

	return resolved
}

func instanceOf(t *testing.T, manifestYAML string) any {
	t.Helper()

	asJSON, err := yaml.YAMLToJSON([]byte(manifestYAML))
	if err != nil {
		t.Fatalf("converting the manifest to json returned %v", err)
	}

	var instance any
	if err := json.Unmarshal(asJSON, &instance); err != nil {
		t.Fatalf("unmarshalling the manifest returned %v", err)
	}

	return instance
}

func TestTheEmbeddedSchemaDeclaresDraft2020Twelve(t *testing.T) {
	t.Parallel()

	var s jsonschema.Schema
	if err := json.Unmarshal(cellmanifest.Schema(), &s); err != nil {
		t.Fatalf("unmarshalling the embedded schema returned %v", err)
	}

	want := "https://json-schema.org/draft/2020-12/schema"
	if s.Schema != want {
		t.Fatalf("got %q, want %q", s.Schema, want)
	}
}

func TestSchemaReturnsACopyTheCallerCannotCorrupt(t *testing.T) {
	t.Parallel()

	first := cellmanifest.Schema()
	first[0] = 'x'

	second := cellmanifest.Schema()
	if second[0] != '{' {
		t.Fatalf("got first byte %q, want %q", second[0], '{')
	}
}

func TestTheSchemaAcceptsAManifestTheContractAllows(t *testing.T) {
	t.Parallel()

	resolved := resolvedSchema(t)

	cases := []struct {
		name         string
		manifestYAML string
	}{
		{
			name:         "the grpc cell example validates against the schema",
			manifestYAML: grpcCellExample,
		},
		{
			name:         "a cell that provides nothing validates against the schema",
			manifestYAML: "cell: udp\ngenerator: udp-rust\n",
		},
		{
			name: "a config field with every key validates against the schema",
			manifestYAML: "cell: rest\ngenerator: rest-rust-axum\n" +
				"provides:\n  adapters:\n    - name: greeting_sqlite\n      type: GreetingSqliteStore\n" +
				"      module: adapter::greeting_sqlite\n      implements: GreetingStore\n" +
				"      config:\n        path:\n          type: string\n          required: true\n" +
				"          default: ./greeting.db\n          description: where the store file lives\n" +
				"        timeout:\n          type: duration\n" +
				"        retries:\n          type: integer\n" +
				"        strict:\n          type: boolean\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := resolved.Validate(instanceOf(t, tc.manifestYAML)); err != nil {
				t.Fatalf("validating against the schema returned %v", err)
			}
		})
	}
}

func TestTheSchemaRefusesAManifestTheContractForbids(t *testing.T) {
	t.Parallel()

	resolved := resolvedSchema(t)

	cases := []struct {
		name         string
		manifestYAML string
	}{
		{
			name: "a config field type outside the list fails the schema",
			manifestYAML: "cell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  adapters:\n    - name: hello_grpc_client\n      type: HelloGrpcClient\n" +
				"      implements: HelloClient\n      config:\n        addr: { type: float }\n",
		},
		{
			name:         "a manifest with no cell name fails the schema",
			manifestYAML: "generator: grpc-rust-tonic\n",
		},
		{
			name:         "a cell name that is not snake case fails the schema",
			manifestYAML: "cell: GrpcCell\ngenerator: grpc-rust-tonic\n",
		},
		{
			name: "a driver that requires no controller trait fails the schema",
			manifestYAML: "cell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  drivers:\n    - name: grpc\n      type: HelloGrpcDriver\n      requires: []\n",
		},
		{
			name: "an adapter that implements no port fails the schema",
			manifestYAML: "cell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  adapters:\n    - name: hello_grpc_client\n      type: HelloGrpcClient\n",
		},
		{
			name: "a module that is not a double colon path fails the schema",
			manifestYAML: "cell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  ports:\n    - trait: GreetingStore\n      module: rest/port/greeting_store\n",
		},
		{
			name:         "an unknown top level key fails the schema",
			manifestYAML: "cell: grpc\ngenerator: grpc-rust-tonic\nprovide: {}\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := resolved.Validate(instanceOf(t, tc.manifestYAML)); err == nil {
				t.Fatalf("validating against the schema returned no error")
			}
		})
	}
}

func TestWhatWriteProducesStillValidatesAgainstTheSchema(t *testing.T) {
	t.Parallel()

	written, err := cellmanifest.Marshal(exampleManifest())
	if err != nil {
		t.Fatalf("marshalling the example returned %v", err)
	}

	body := strings.SplitN(string(written), "\n", 2)[1]

	if err := resolvedSchema(t).Validate(instanceOf(t, body)); err != nil {
		t.Fatalf("validating the written manifest returned %v", err)
	}
}
