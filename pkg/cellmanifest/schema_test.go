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
			name: "a driver that consumes ports validates against the schema",
			manifestYAML: "version: \"1\"\ncell: udp\ngenerator: udp-rust\n" +
				"provides:\n  drivers:\n    - name: udp\n      type: HelloDatagramUdpDriver\n" +
				"      module: udp::driver::hello_datagram_udp_driver\n      requires: [HelloDatagramController]\n" +
				"      ports: [HelloDatagramSessionGate, HelloDatagramBroadcast]\n",
		},
		{
			name: "a driver with protocol TCP validates against the schema",
			manifestYAML: "version: \"1\"\ncell: rest\ngenerator: rest-rust\n" +
				"provides:\n  drivers:\n    - name: rest\n      type: HttpDriver\n" +
				"      module: rest::driver::http_driver\n      protocol: TCP\n      requires: [GreetingController]\n",
		},
		{
			name: "a driver with protocol UDP validates against the schema",
			manifestYAML: "version: \"1\"\ncell: udp\ngenerator: udp-rust\n" +
				"provides:\n  drivers:\n    - name: udp\n      type: HelloDatagramUdpDriver\n" +
				"      module: udp::driver::hello_datagram_udp_driver\n      protocol: UDP\n      requires: [HelloDatagramController]\n",
		},
		{
			name: "a build script under a directory of the cell validates against the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"buildScript: build/zz_generated_build.rs\n",
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
		{
			name:         "a config field marked secret true validates against the schema",
			manifestYAML: secretFieldManifest("true"),
		},
		{
			name:         "a config field marked secret false validates against the schema",
			manifestYAML: secretFieldManifest("false"),
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

func secretFieldManifest(secret string) string {
	return "version: \"1\"\ncell: rest\ngenerator: rest-rust\n" +
		"provides:\n  adapters:\n    - name: quiet\n      type: QuietStore\n" +
		"      module: adapter::quiet\n      implements: GreetingStore\n" +
		"      config:\n        token:\n          type: string\n          secret: " + secret + "\n"
}

func TestTheSchemaRefusesAManifestTheContractForbids(t *testing.T) {
	t.Parallel()

	resolved := resolvedSchema(t)

	cases := []struct {
		name         string
		manifestYAML string
		named        string
	}{
		{
			name:         "a config field marked secret with the string yes fails the schema naming secret",
			manifestYAML: secretFieldManifest(`"yes"`),
			named:        "secret",
		},
		{
			name:         "a config field marked secret with the integer one fails the schema naming secret",
			manifestYAML: secretFieldManifest("1"),
			named:        "secret",
		},
		{
			name: "a config field type outside the list fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  adapters:\n    - name: hello_grpc_client\n      type: HelloGrpcClient\n" +
				"      module: grpc::adapter::hello_grpc_client\n" +
				"      implements: HelloClient\n      config:\n        addr: { type: float }\n",
			named: `/$defs/configField/properties/type: enum: float does not equal any of`,
		},
		{
			name: "a driver with protocol http fails the schema",
			manifestYAML: "version: \"1\"\ncell: rest\ngenerator: rest-rust\n" +
				"provides:\n  drivers:\n    - name: rest\n      type: HttpDriver\n" +
				"      module: rest::driver::http_driver\n      protocol: http\n      requires: [GreetingController]\n",
			named: `/$defs/driver/properties/protocol: enum: http does not equal any of`,
		},
		{
			name:         "a manifest with no cell name fails the schema",
			manifestYAML: "version: \"1\"\ngenerator: grpc-rust-tonic\n",
			named:        `required: missing properties: ["cell"]`,
		},
		{
			name:         "a manifest with no version fails the schema",
			manifestYAML: "cell: grpc\ngenerator: grpc-rust-tonic\n",
			named:        `required: missing properties: ["version"]`,
		},
		{
			name:         "a version other than one fails the schema",
			manifestYAML: "version: \"2\"\ncell: grpc\ngenerator: grpc-rust-tonic\n",
			named:        `/properties/version: const: 2 does not equal 1`,
		},
		{
			name:         "a build script that leaves the cell fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\nbuildScript: ../zz_generated_build.rs\n",
			named:        `/$defs/buildScript: pattern: "../zz_generated_build.rs" does not match`,
		},
		{
			name:         "a build script whose name is not generated fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\nbuildScript: build.rs\n",
			named:        `/$defs/buildScript: pattern: "build.rs" does not match`,
		},
		{
			name:         "a cell name that is not snake case fails the schema",
			manifestYAML: "version: \"1\"\ncell: GrpcCell\ngenerator: grpc-rust-tonic\n",
			named:        `/properties/cell: validating /$defs/snakeName: pattern: "GrpcCell"`,
		},
		{
			name: "a driver that requires no controller trait fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  drivers:\n    - name: grpc\n      type: HelloGrpcDriver\n" +
				"      module: grpc::driver::hello_grpc_driver\n      requires: []\n",
			named: `/$defs/driver/properties/requires/allOf/1: minItems: array length 0 is less than 1`,
		},
		{
			name: "an adapter that implements no port fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  adapters:\n    - name: hello_grpc_client\n      type: HelloGrpcClient\n" +
				"      module: grpc::adapter::hello_grpc_client\n",
			named: `/$defs/adapter: required: missing properties: ["implements"]`,
		},
		{
			name: "a driver with no module fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  drivers:\n    - name: grpc\n      type: HelloGrpcDriver\n" +
				"      requires: [HelloController]\n",
			named: `/$defs/driver: required: missing properties: ["module"]`,
		},
		{
			name: "an adapter with no module fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  adapters:\n    - name: hello_grpc_client\n      type: HelloGrpcClient\n" +
				"      implements: HelloClient\n",
			named: `/$defs/adapter: required: missing properties: ["module"]`,
		},
		{
			name: "a controller with no module fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  controllers:\n    - trait: HelloController\n      impl: HelloControllerImpl\n",
			named: `/$defs/controller: required: missing properties: ["module"]`,
		},
		{
			name: "a port with no module fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  ports:\n    - trait: GreetingStore\n",
			named: `/$defs/port: required: missing properties: ["module"]`,
		},
		{
			name: "a module that is not a double colon path fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  ports:\n    - trait: GreetingStore\n      module: rest/port/greeting_store\n",
			named: `/$defs/port/properties/module: validating /$defs/modulePath: pattern: "rest/port/greeting_store"`,
		},
		{
			name:         "an unknown top level key fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\nprovide: {}\n",
			named:        `unexpected additional properties ["provide"]`,
		},
		{
			name: "a driver name that is not snake case fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  drivers:\n    - name: GrpcDriver\n      type: HelloGrpcDriver\n" +
				"      module: grpc::driver::hello_grpc_driver\n      requires: [HelloController]\n",
			named: `/$defs/driver/properties/name: validating /$defs/snakeName: pattern: "GrpcDriver"`,
		},
		{
			name: "a controller trait a driver requires that is not a Rust ident fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  drivers:\n    - name: grpc\n      type: HelloGrpcDriver\n" +
				"      module: grpc::driver::hello_grpc_driver\n      requires: [hello-controller]\n",
			named: `/$defs/driver/properties/requires/allOf/0: validating /$defs/traitList: ` +
				`validating /$defs/traitList/items: validating /$defs/rustIdent: pattern: "hello-controller"`,
		},
		{
			name: "an adapter name that is not snake case fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  adapters:\n    - name: HelloGrpcClient\n      type: HelloGrpcClient\n" +
				"      module: grpc::adapter::hello_grpc_client\n      implements: HelloClient\n",
			named: `/$defs/adapter/properties/name: validating /$defs/snakeName: pattern: "HelloGrpcClient"`,
		},
		{
			name: "a port an adapter implements that is not a Rust ident fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  adapters:\n    - name: hello_grpc_client\n      type: HelloGrpcClient\n" +
				"      module: grpc::adapter::hello_grpc_client\n      implements: hello-client\n",
			named: `/$defs/adapter/properties/implements: validating /$defs/rustIdent: pattern: "hello-client"`,
		},
		{
			name: "a driver type that is not a Rust ident fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  drivers:\n    - name: grpc\n      type: hello-grpc-driver\n" +
				"      module: grpc::driver::hello_grpc_driver\n      requires: [HelloController]\n",
			named: `/$defs/driver/properties/type: validating /$defs/rustIdent: pattern: "hello-grpc-driver"`,
		},
		{
			name: "an adapter type that is not a Rust ident fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  adapters:\n    - name: hello_grpc_client\n      type: hello-grpc-client\n" +
				"      module: grpc::adapter::hello_grpc_client\n      implements: HelloClient\n",
			named: `/$defs/adapter/properties/type: validating /$defs/rustIdent: pattern: "hello-grpc-client"`,
		},
		{
			name: "a controller impl that is not a Rust ident fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  controllers:\n    - trait: HelloController\n" +
				"      impl: hello-controller-impl\n      module: grpc::controller::hello_controller\n",
			named: `/$defs/controller/properties/impl: validating /$defs/rustIdent: pattern: "hello-controller-impl"`,
		},
		{
			name: "a port trait that is not a Rust ident fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  ports:\n    - trait: greeting-store\n      module: rest::port::greeting_store\n",
			named: `/$defs/port/properties/trait: validating /$defs/rustIdent: pattern: "greeting-store"`,
		},
		{
			name: "a controller trait that is not a Rust ident fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  controllers:\n    - trait: hello-controller\n" +
				"      impl: HelloControllerImpl\n      module: grpc::controller::hello_controller\n",
			named: `/$defs/controller/properties/trait: validating /$defs/rustIdent: pattern: "hello-controller"`,
		},
		{
			name: "a port a controller consumes that is not a Rust ident fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  controllers:\n    - trait: HelloController\n" +
				"      impl: HelloControllerImpl\n      module: grpc::controller::hello_controller\n" +
				"      ports: [greeting-store]\n",
			named: `/$defs/controller/properties/ports: validating /$defs/traitList: ` +
				`validating /$defs/traitList/items: validating /$defs/rustIdent: pattern: "greeting-store"`,
		},
		{
			name: "a port a cell requires that is not a Rust ident fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"requires:\n  ports: [greeting-store]\n",
			named: `/$defs/requires/properties/ports: validating /$defs/traitList: ` +
				`validating /$defs/traitList/items: validating /$defs/rustIdent: pattern: "greeting-store"`,
		},
		{
			name: "a config field name that is not snake case fails the schema",
			manifestYAML: "version: \"1\"\ncell: grpc\ngenerator: grpc-rust-tonic\n" +
				"provides:\n  adapters:\n    - name: hello_grpc_client\n      type: HelloGrpcClient\n" +
				"      module: grpc::adapter::hello_grpc_client\n      implements: HelloClient\n" +
				"      config:\n        Addr: { type: string }\n",
			named: `/$defs/config/propertyNames: validating /$defs/snakeName: pattern: "Addr"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := resolved.Validate(instanceOf(t, tc.manifestYAML))
			if err == nil {
				t.Fatalf("validating against the schema returned no error")
			}

			if tc.named == "" {
				t.Fatalf("the case names no text the refusal must carry, the schema returned %q", err.Error())
			}

			if !strings.Contains(err.Error(), tc.named) {
				t.Fatalf("got %q, want it to name %q", err.Error(), tc.named)
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
			name: "a driver config field required key with no value is refused",
			manifestYAML: nullCaseHeader +
				"provides:\n  drivers:\n    - name: grpc\n      type: HelloGrpcDriver\n" +
				"      module: grpc::driver::hello_grpc_driver\n      requires: [HelloController]\n" +
				"      config:\n        addr:\n          type: string\n          required:\n",
			message: `key "provides.drivers[0].config.addr.required" is null, write a value`,
		},
		{
			name: "an adapter config field description key with no value is refused",
			manifestYAML: nullCaseHeader +
				"provides:\n  adapters:\n    - name: hello_grpc_client\n      type: HelloGrpcClient\n" +
				"      module: grpc::adapter::hello_grpc_client\n      implements: HelloClient\n" +
				"      config:\n        addr:\n          type: string\n          description:\n",
			message: `key "provides.adapters[0].config.addr.description" is null, write a value`,
		},
		{
			name:         "a cell key with no value is refused",
			manifestYAML: "version: \"1\"\ncell:\ngenerator: grpc-rust-tonic\n",
			message:      `key "cell" is null, write a value`,
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

func TestParseAndTheSchemaBothAcceptAConfigFieldDefaultWithNoValue(t *testing.T) {
	t.Parallel()

	manifestYAML := nullCaseHeader +
		"provides:\n  adapters:\n    - name: hello_grpc_client\n      type: HelloGrpcClient\n" +
		"      module: grpc::adapter::hello_grpc_client\n      implements: HelloClient\n" +
		"      config:\n        addr:\n          type: string\n          default:\n"

	if _, err := cellmanifest.Parse([]byte(manifestYAML)); err != nil {
		t.Fatalf("parsing a config field with a null default returned %v", err)
	}

	if err := resolvedSchema(t).Validate(instanceOf(t, manifestYAML)); err != nil {
		t.Fatalf("validating a config field with a null default returned %v", err)
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
