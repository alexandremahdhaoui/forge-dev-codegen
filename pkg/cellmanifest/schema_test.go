package cellmanifest_test

import (
	"encoding/json"
	"os"
	"path/filepath"
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
			manifestYAML: "version: \"1\"\ncell: udp\ngenerator: udp-rust\n",
		},
		{
			name: "a config field with every key validates against the schema",
			manifestYAML: "version: \"1\"\ncell: rest\ngenerator: rest-rust-axum\n" +
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
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  adapters:\n    - name: hello_grpc_client\n      type: HelloGrpcClient\n" +
				"      module: grpc::adapter::hello_grpc_client\n" +
				"      implements: HelloClient\n      config:\n        addr: { type: float }\n",
		},
		{
			name:         "a manifest with no cell name fails the schema",
			manifestYAML: "version: \"1\"\ngenerator: grpc-rust-tonic\n",
		},
		{
			name:         "a manifest with no version fails the schema",
			manifestYAML: "cell: grpc\ngenerator: grpc-rust-tonic\n",
		},
		{
			name:         "a version other than one fails the schema",
			manifestYAML: "version: \"2\"\ncell: grpc\ngenerator: grpc-rust-tonic\n",
		},
		{
			name:         "a cell name that is not snake case fails the schema",
			manifestYAML: "version: \"1\"\ncell: GrpcCell\ngenerator: grpc-rust-tonic\n",
		},
		{
			name: "a driver that requires no controller trait fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  drivers:\n    - name: grpc\n      type: HelloGrpcDriver\n" +
				"      module: grpc::driver::hello_grpc_driver\n      requires: []\n",
		},
		{
			name: "an adapter that implements no port fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  adapters:\n    - name: hello_grpc_client\n      type: HelloGrpcClient\n" +
				"      module: grpc::adapter::hello_grpc_client\n",
		},
		{
			name: "a driver with no module fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  drivers:\n    - name: grpc\n      type: HelloGrpcDriver\n" +
				"      requires: [HelloController]\n",
		},
		{
			name: "an adapter with no module fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  adapters:\n    - name: hello_grpc_client\n      type: HelloGrpcClient\n" +
				"      implements: HelloClient\n",
		},
		{
			name: "a controller with no module fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  controllers:\n    - trait: HelloController\n      impl: HelloControllerImpl\n",
		},
		{
			name: "a port with no module fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  ports:\n    - trait: GreetingStore\n",
		},
		{
			name: "a module that is not a double colon path fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  ports:\n    - trait: GreetingStore\n      module: rest/port/greeting_store\n",
		},
		{
			name:         "an unknown top level key fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\nprovide: {}\n",
		},
		{
			name: "a driver name that is not snake case fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  drivers:\n    - name: GrpcDriver\n      type: HelloGrpcDriver\n" +
				"      module: grpc::driver::hello_grpc_driver\n      requires: [HelloController]\n",
		},
		{
			name: "a controller trait a driver requires that is not a Rust ident fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  drivers:\n    - name: grpc\n      type: HelloGrpcDriver\n" +
				"      module: grpc::driver::hello_grpc_driver\n      requires: [hello-controller]\n",
		},
		{
			name: "an adapter name that is not snake case fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  adapters:\n    - name: HelloGrpcClient\n      type: HelloGrpcClient\n" +
				"      module: grpc::adapter::hello_grpc_client\n      implements: HelloClient\n",
		},
		{
			name: "a port an adapter implements that is not a Rust ident fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  adapters:\n    - name: hello_grpc_client\n      type: HelloGrpcClient\n" +
				"      module: grpc::adapter::hello_grpc_client\n      implements: hello-client\n",
		},
		{
			name: "a controller trait that is not a Rust ident fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  controllers:\n    - trait: hello-controller\n" +
				"      impl: HelloControllerImpl\n      module: grpc::controller::hello_controller\n",
		},
		{
			name: "a port a controller consumes that is not a Rust ident fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  controllers:\n    - trait: HelloController\n" +
				"      impl: HelloControllerImpl\n      module: grpc::controller::hello_controller\n" +
				"      ports: [greeting-store]\n",
		},
		{
			name: "a port a cell requires that is not a Rust ident fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"requires:\n  ports: [greeting-store]\n",
		},
		{
			name: "a config field name that is not snake case fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  adapters:\n    - name: hello_grpc_client\n      type: HelloGrpcClient\n" +
				"      module: grpc::adapter::hello_grpc_client\n      implements: HelloClient\n" +
				"      config:\n        Addr: { type: string }\n",
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

func writtenManifest(t *testing.T, m cellmanifest.Manifest) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), cellmanifest.FileName)

	if err := cellmanifest.Write(path, m); err != nil {
		t.Fatalf("writing the manifest returned %v", err)
	}

	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the written manifest returned %v", err)
	}

	return string(written)
}

func TestWhatWriteProducesStillValidatesAgainstTheSchema(t *testing.T) {
	t.Parallel()

	written := writtenManifest(t, exampleManifest())

	body := strings.SplitN(written, "\n", 2)[1]

	if err := resolvedSchema(t).Validate(instanceOf(t, body)); err != nil {
		t.Fatalf("validating the written manifest returned %v", err)
	}
}

func TestAManifestWithNilSlicesStillValidatesAgainstTheSchema(t *testing.T) {
	t.Parallel()

	m := exampleManifest()
	m.Requires.Ports = nil
	m.Provides.Controllers[0].Ports = nil

	written := writtenManifest(t, m)

	if strings.Contains(written, "null") {
		t.Fatalf("got %q, want no null in the written manifest", written)
	}

	body := strings.SplitN(written, "\n", 2)[1]

	if err := resolvedSchema(t).Validate(instanceOf(t, body)); err != nil {
		t.Fatalf("validating a manifest written from nil slices returned %v", err)
	}

	back, err := cellmanifest.Parse([]byte(body))
	if err != nil {
		t.Fatalf("parsing back a manifest written from nil slices returned %v", err)
	}

	if len(back.Requires.Ports) != 0 || len(back.Provides.Controllers[0].Ports) != 0 {
		t.Fatalf("got %+v, want empty port lists", back)
	}
}

const nullCaseHeader = "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n"

func TestParseAndTheSchemaBothRefuseAnExplicitNull(t *testing.T) {
	t.Parallel()

	resolved := resolvedSchema(t)

	cases := []struct {
		name         string
		manifestYAML string
		message      string
	}{
		{
			name:         "a requires key with no value is refused",
			manifestYAML: nullCaseHeader + "requires:\n",
			message:      `key "requires" is null, write an empty map`,
		},
		{
			name:         "a provides key with no value is refused",
			manifestYAML: nullCaseHeader + "provides:\n",
			message:      `key "provides" is null, write an empty map`,
		},
		{
			name:         "a required ports key with no value is refused",
			manifestYAML: nullCaseHeader + "requires:\n  ports:\n",
			message:      `key "requires.ports" is null, write an empty list`,
		},
		{
			name:         "a provided drivers key with no value is refused",
			manifestYAML: nullCaseHeader + "provides:\n  drivers:\n",
			message:      `key "provides.drivers" is null, write an empty list`,
		},
		{
			name:         "a provided adapters key with no value is refused",
			manifestYAML: nullCaseHeader + "provides:\n  adapters:\n",
			message:      `key "provides.adapters" is null, write an empty list`,
		},
		{
			name:         "a provided controllers key with no value is refused",
			manifestYAML: nullCaseHeader + "provides:\n  controllers:\n",
			message:      `key "provides.controllers" is null, write an empty list`,
		},
		{
			name:         "a provided ports key with no value is refused",
			manifestYAML: nullCaseHeader + "provides:\n  ports:\n",
			message:      `key "provides.ports" is null, write an empty list`,
		},
		{
			name: "a driver requires key with no value is refused",
			manifestYAML: nullCaseHeader +
				"provides:\n  drivers:\n    - name: grpc\n      type: HelloGrpcDriver\n" +
				"      module: grpc::driver::hello_grpc_driver\n      requires:\n",
			message: `key "provides.drivers[0].requires" is null, write an empty list`,
		},
		{
			name: "a driver config key with no value is refused",
			manifestYAML: nullCaseHeader +
				"provides:\n  drivers:\n    - name: grpc\n      type: HelloGrpcDriver\n" +
				"      module: grpc::driver::hello_grpc_driver\n      requires: [HelloController]\n" +
				"      config:\n",
			message: `key "provides.drivers[0].config" is null, write an empty map`,
		},
		{
			name: "a driver config field with no value is refused",
			manifestYAML: nullCaseHeader +
				"provides:\n  drivers:\n    - name: grpc\n      type: HelloGrpcDriver\n" +
				"      module: grpc::driver::hello_grpc_driver\n      requires: [HelloController]\n" +
				"      config:\n        addr:\n",
			message: `key "provides.drivers[0].config.addr" is null, write an empty map`,
		},
		{
			name: "an adapter config key with no value is refused",
			manifestYAML: nullCaseHeader +
				"provides:\n  adapters:\n    - name: hello_grpc_client\n      type: HelloGrpcClient\n" +
				"      module: grpc::adapter::hello_grpc_client\n      implements: HelloClient\n" +
				"      config:\n",
			message: `key "provides.adapters[0].config" is null, write an empty map`,
		},
		{
			name: "a controller ports key with no value is refused",
			manifestYAML: nullCaseHeader +
				"provides:\n  controllers:\n    - trait: HelloController\n" +
				"      impl: HelloControllerImpl\n      module: grpc::controller::hello_controller\n" +
				"      ports:\n",
			message: `key "provides.controllers[0].ports" is null, write an empty list`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := cellmanifest.Parse([]byte(tc.manifestYAML))
			if err == nil {
				t.Fatalf("parsing %q returned no error", tc.manifestYAML)
			}

			want := "parsing cell manifest: " + tc.message
			if err.Error() != want {
				t.Fatalf("got %q, want %q", err.Error(), want)
			}

			if err := resolved.Validate(instanceOf(t, tc.manifestYAML)); err == nil {
				t.Fatalf("validating %q against the schema returned no error", tc.manifestYAML)
			}
		})
	}
}

func TestMarshalLeavesTheManifestItWasGivenAlone(t *testing.T) {
	t.Parallel()

	m := exampleManifest()
	m.Requires.Ports = nil

	if _, err := cellmanifest.Marshal(m); err != nil {
		t.Fatalf("marshalling the example returned %v", err)
	}

	if m.Requires.Ports != nil {
		t.Fatalf("got %v, want the required ports to stay nil", m.Requires.Ports)
	}
}
