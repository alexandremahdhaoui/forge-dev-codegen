package cellmanifest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/cellmanifest"
)

const grpcCellExample = `version: "1"
cell: grpc
generator: grpc-rust-tonic
provides:
  drivers:
    - name: grpc
      type: HelloGrpcDriver
      module: grpc::driver::hello_grpc_driver
      requires: [HelloController]
      config:
        addr: { type: string, default: "127.0.0.1:0" }
  adapters:
    - name: hello_grpc_client
      type: HelloGrpcClient
      module: grpc::adapter::hello_grpc_client
      implements: HelloClient
      config:
        addr: { type: string, required: true }
  controllers:
    - trait: HelloController
      impl: HelloControllerImpl
      module: grpc::controller::hello_controller
      ports: []
  ports:
    - trait: GreetingStore
      module: rest::port::greeting_store
requires:
  ports: []
`

func exampleManifest() cellmanifest.Manifest {
	return cellmanifest.Manifest{
		Version:   cellmanifest.Version,
		Cell:      "grpc",
		Generator: "grpc-rust-tonic",
		Provides: cellmanifest.Provides{
			Drivers: []cellmanifest.Driver{{
				Name:     "grpc",
				Type:     "HelloGrpcDriver",
				Module:   "grpc::driver::hello_grpc_driver",
				Requires: []string{"HelloController"},
				Config: map[string]cellmanifest.ConfigField{
					"addr": {Type: cellmanifest.FieldTypeString, Default: "127.0.0.1:0"},
				},
			}},
			Adapters: []cellmanifest.Adapter{{
				Name:       "hello_grpc_client",
				Type:       "HelloGrpcClient",
				Module:     "grpc::adapter::hello_grpc_client",
				Implements: "HelloClient",
				Config: map[string]cellmanifest.ConfigField{
					"addr": {Type: cellmanifest.FieldTypeString, Required: true},
				},
			}},
			Controllers: []cellmanifest.Controller{{
				Trait:  "HelloController",
				Impl:   "HelloControllerImpl",
				Module: "grpc::controller::hello_controller",
				Ports:  []string{},
			}},
			Ports: []cellmanifest.Port{{
				Trait:  "GreetingStore",
				Module: "rest::port::greeting_store",
			}},
		},
		Requires: cellmanifest.Requires{Ports: []string{}},
	}
}

func TestParseReadsEveryFieldOfTheGrpcCellExample(t *testing.T) {
	t.Parallel()

	m, err := cellmanifest.Parse([]byte(grpcCellExample))
	if err != nil {
		t.Fatalf("parsing the example returned %v", err)
	}

	cases := []struct {
		name string
		got  any
		want any
	}{
		{"the version is one", m.Version, "1"},
		{"the cell name is grpc", m.Cell, "grpc"},
		{"the generator is grpc-rust-tonic", m.Generator, "grpc-rust-tonic"},
		{"one driver is provided", len(m.Provides.Drivers), 1},
		{"the driver type is HelloGrpcDriver", m.Provides.Drivers[0].Type, "HelloGrpcDriver"},
		{
			"the driver module is the hello grpc driver path",
			m.Provides.Drivers[0].Module,
			"grpc::driver::hello_grpc_driver",
		},
		{"the driver requires HelloController", m.Provides.Drivers[0].Requires[0], "HelloController"},
		{"the driver addr field is a string", m.Provides.Drivers[0].Config["addr"].Type, "string"},
		{
			"the driver addr field defaults to the loopback address",
			m.Provides.Drivers[0].Config["addr"].Default,
			"127.0.0.1:0",
		},
		{"one adapter is provided", len(m.Provides.Adapters), 1},
		{"the adapter implements HelloClient", m.Provides.Adapters[0].Implements, "HelloClient"},
		{"the adapter addr field is required", m.Provides.Adapters[0].Config["addr"].Required, true},
		{"one controller is provided", len(m.Provides.Controllers), 1},
		{"the controller impl is HelloControllerImpl", m.Provides.Controllers[0].Impl, "HelloControllerImpl"},
		{"one port is provided", len(m.Provides.Ports), 1},
		{"the port trait is GreetingStore", m.Provides.Ports[0].Trait, "GreetingStore"},
		{"the cell requires no port", len(m.Requires.Ports), 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.got != tc.want {
				t.Fatalf("got %v, want %v", tc.got, tc.want)
			}
		})
	}
}

func TestParseRefusesAManifestItCannotTrust(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		yaml    string
		message string
	}{
		{
			name:    "a manifest with an unknown field is refused",
			yaml:    "cell: grpc\ngenerator: g\nprovide: {}\n",
			message: "parsing cell manifest",
		},
		{
			name:    "a manifest that is not yaml is refused",
			yaml:    "\tnot yaml at all\n",
			message: "parsing cell manifest",
		},
		{
			name:    "a manifest with no cell name is refused",
			yaml:    "generator: g\n",
			message: "cell name is empty",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := cellmanifest.Parse([]byte(tc.yaml))
			if err == nil {
				t.Fatalf("parsing %q returned no error", tc.yaml)
			}

			if !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("got %q, want it to contain %q", err.Error(), tc.message)
			}
		})
	}
}

func TestValidateNamesTheThingItRefuses(t *testing.T) {
	t.Parallel()

	badFieldType := exampleManifest()
	badFieldType.Provides.Drivers[0].Config["addr"] = cellmanifest.ConfigField{Type: "float"}

	noCell := exampleManifest()
	noCell.Cell = ""

	noGenerator := exampleManifest()
	noGenerator.Generator = ""

	driverWithoutController := exampleManifest()
	driverWithoutController.Provides.Drivers[0].Requires = nil

	driverWithoutName := exampleManifest()
	driverWithoutName.Provides.Drivers[0].Name = ""

	adapterWithoutPort := exampleManifest()
	adapterWithoutPort.Provides.Adapters[0].Implements = ""

	adapterWithoutName := exampleManifest()
	adapterWithoutName.Provides.Adapters[0].Name = ""

	badAdapterFieldType := exampleManifest()
	badAdapterFieldType.Provides.Adapters[0].Config["addr"] = cellmanifest.ConfigField{Type: "port"}

	noVersion := exampleManifest()
	noVersion.Version = ""

	wrongVersion := exampleManifest()
	wrongVersion.Version = "2"

	shoutingCell := exampleManifest()
	shoutingCell.Cell = "GrpcCell"

	driverWithoutType := exampleManifest()
	driverWithoutType.Provides.Drivers[0].Type = ""

	driverWithABadType := exampleManifest()
	driverWithABadType.Provides.Drivers[0].Type = "hello-grpc-driver"

	driverWithoutModule := exampleManifest()
	driverWithoutModule.Provides.Drivers[0].Module = ""

	driverWithABadModule := exampleManifest()
	driverWithABadModule.Provides.Drivers[0].Module = "grpc/driver/hello_grpc_driver"

	adapterWithoutType := exampleManifest()
	adapterWithoutType.Provides.Adapters[0].Type = ""

	adapterWithoutModule := exampleManifest()
	adapterWithoutModule.Provides.Adapters[0].Module = ""

	controllerWithoutTrait := exampleManifest()
	controllerWithoutTrait.Provides.Controllers[0].Trait = ""

	controllerWithoutImpl := exampleManifest()
	controllerWithoutImpl.Provides.Controllers[0].Impl = ""

	controllerWithABadImpl := exampleManifest()
	controllerWithABadImpl.Provides.Controllers[0].Impl = "hello controller impl"

	controllerWithoutModule := exampleManifest()
	controllerWithoutModule.Provides.Controllers[0].Module = ""

	portWithoutTrait := exampleManifest()
	portWithoutTrait.Provides.Ports[0].Trait = ""

	portWithABadTrait := exampleManifest()
	portWithABadTrait.Provides.Ports[0].Trait = "greeting-store"

	portWithoutModule := exampleManifest()
	portWithoutModule.Provides.Ports[0].Module = ""

	driverWithABadName := exampleManifest()
	driverWithABadName.Provides.Drivers[0].Name = "GrpcDriver"

	driverWithABadRequiredTrait := exampleManifest()
	driverWithABadRequiredTrait.Provides.Drivers[0].Requires = []string{"hello-controller"}

	adapterWithABadName := exampleManifest()
	adapterWithABadName.Provides.Adapters[0].Name = "HelloGrpcClient"

	adapterWithABadImplements := exampleManifest()
	adapterWithABadImplements.Provides.Adapters[0].Implements = "hello-client"

	controllerWithABadTrait := exampleManifest()
	controllerWithABadTrait.Provides.Controllers[0].Trait = "hello-controller"

	controllerWithABadPort := exampleManifest()
	controllerWithABadPort.Provides.Controllers[0].Ports = []string{"greeting-store"}

	cellRequiringABadPort := exampleManifest()
	cellRequiringABadPort.Requires.Ports = []string{"greeting-store"}

	configFieldWithABadName := exampleManifest()
	configFieldWithABadName.Provides.Drivers[0].Config["Addr"] = cellmanifest.ConfigField{
		Type: cellmanifest.FieldTypeString,
	}

	cases := []struct {
		name     string
		manifest cellmanifest.Manifest
		message  string
	}{
		{
			name:     "a manifest with an empty cell name is refused",
			manifest: noCell,
			message:  "cell name is empty",
		},
		{
			name:     "a cell name that is not snake case is refused",
			manifest: shoutingCell,
			message:  `cell name "GrpcCell" is not snake case`,
		},
		{
			name:     "a manifest with no version is refused",
			manifest: noVersion,
			message:  `cell "grpc" declares version "" and the only version is "1"`,
		},
		{
			name:     "a manifest with a version other than one is refused",
			manifest: wrongVersion,
			message:  `cell "grpc" declares version "2" and the only version is "1"`,
		},
		{
			name:     "a manifest with no generator is refused",
			manifest: noGenerator,
			message:  `cell "grpc" names no generator`,
		},
		{
			name:     "a driver with no type is refused",
			manifest: driverWithoutType,
			message:  `driver "grpc" in cell "grpc" names no type`,
		},
		{
			name:     "a driver type that is not a Rust ident is refused",
			manifest: driverWithABadType,
			message:  `driver "grpc" in cell "grpc" has type "hello-grpc-driver" which is not a Rust ident`,
		},
		{
			name:     "a driver with no module is refused",
			manifest: driverWithoutModule,
			message:  `driver "grpc" in cell "grpc" names no module`,
		},
		{
			name:     "a driver module that is not a double colon path is refused",
			manifest: driverWithABadModule,
			message: `driver "grpc" in cell "grpc" has module "grpc/driver/hello_grpc_driver" ` +
				`which is not a :: path`,
		},
		{
			name:     "an adapter with no type is refused",
			manifest: adapterWithoutType,
			message:  `adapter "hello_grpc_client" in cell "grpc" names no type`,
		},
		{
			name:     "an adapter with no module is refused",
			manifest: adapterWithoutModule,
			message:  `adapter "hello_grpc_client" in cell "grpc" names no module`,
		},
		{
			name:     "a controller with no trait is refused",
			manifest: controllerWithoutTrait,
			message:  `cell "grpc" provides a controller with no trait`,
		},
		{
			name:     "a controller with no impl is refused",
			manifest: controllerWithoutImpl,
			message:  `controller "HelloController" in cell "grpc" names no impl`,
		},
		{
			name:     "a controller impl that is not a Rust ident is refused",
			manifest: controllerWithABadImpl,
			message: `controller "HelloController" in cell "grpc" has impl "hello controller impl" ` +
				`which is not a Rust ident`,
		},
		{
			name:     "a controller with no module is refused",
			manifest: controllerWithoutModule,
			message:  `controller "HelloController" in cell "grpc" names no module`,
		},
		{
			name:     "a port with no trait is refused",
			manifest: portWithoutTrait,
			message:  `cell "grpc" provides a port with no trait`,
		},
		{
			name:     "a port trait that is not a Rust ident is refused",
			manifest: portWithABadTrait,
			message:  `port trait "greeting-store" in cell "grpc" is not a Rust ident`,
		},
		{
			name:     "a port with no module is refused",
			manifest: portWithoutModule,
			message:  `port "GreetingStore" in cell "grpc" names no module`,
		},
		{
			name:     "a driver that requires no controller trait is refused",
			manifest: driverWithoutController,
			message:  `driver "grpc" in cell "grpc" requires no controller trait`,
		},
		{
			name:     "a driver with no name is refused",
			manifest: driverWithoutName,
			message:  `cell "grpc" provides a driver with no name`,
		},
		{
			name:     "an adapter that implements no port is refused",
			manifest: adapterWithoutPort,
			message:  `adapter "hello_grpc_client" in cell "grpc" implements no port`,
		},
		{
			name:     "an adapter with no name is refused",
			manifest: adapterWithoutName,
			message:  `cell "grpc" provides an adapter with no name`,
		},
		{
			name:     "a driver config field with a type outside the list is refused",
			manifest: badFieldType,
			message: `config field "addr" of driver "grpc" in cell "grpc" has type "float" ` +
				`which is not one of string, integer, boolean, duration`,
		},
		{
			name:     "an adapter config field with a type outside the list is refused",
			manifest: badAdapterFieldType,
			message: `config field "addr" of adapter "hello_grpc_client" in cell "grpc" has type "port" ` +
				`which is not one of string, integer, boolean, duration`,
		},
		{
			name:     "a driver name that is not snake case is refused",
			manifest: driverWithABadName,
			message:  `driver name "GrpcDriver" in cell "grpc" is not snake case`,
		},
		{
			name:     "a controller trait a driver requires that is not a Rust ident is refused",
			manifest: driverWithABadRequiredTrait,
			message: `driver "grpc" in cell "grpc" requires "hello-controller" ` +
				`which is not a Rust ident`,
		},
		{
			name:     "an adapter name that is not snake case is refused",
			manifest: adapterWithABadName,
			message:  `adapter name "HelloGrpcClient" in cell "grpc" is not snake case`,
		},
		{
			name:     "a port an adapter implements that is not a Rust ident is refused",
			manifest: adapterWithABadImplements,
			message: `adapter "hello_grpc_client" in cell "grpc" implements "hello-client" ` +
				`which is not a Rust ident`,
		},
		{
			name:     "a controller trait that is not a Rust ident is refused",
			manifest: controllerWithABadTrait,
			message:  `controller trait "hello-controller" in cell "grpc" is not a Rust ident`,
		},
		{
			name:     "a port a controller consumes that is not a Rust ident is refused",
			manifest: controllerWithABadPort,
			message: `controller "HelloController" in cell "grpc" consumes port "greeting-store" ` +
				`which is not a Rust ident`,
		},
		{
			name:     "a port a cell requires that is not a Rust ident is refused",
			manifest: cellRequiringABadPort,
			message:  `cell "grpc" requires port "greeting-store" which is not a Rust ident`,
		},
		{
			name:     "a config field name that is not snake case is refused",
			manifest: configFieldWithABadName,
			message:  `config field "Addr" of driver "grpc" in cell "grpc" is not snake case`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.manifest.Validate()
			if err == nil {
				t.Fatalf("validating returned no error")
			}

			if err.Error() != tc.message {
				t.Fatalf("got %q, want %q", err.Error(), tc.message)
			}
		})
	}
}

func TestValidateAcceptsTheGrpcCellExample(t *testing.T) {
	t.Parallel()

	if err := exampleManifest().Validate(); err != nil {
		t.Fatalf("validating the example returned %v", err)
	}
}

func TestWriteProducesAHeaderNamingTheGeneratorAndTheSameBytesEveryRun(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "cell", cellmanifest.FileName)

	if err := cellmanifest.Write(path, exampleManifest()); err != nil {
		t.Fatalf("writing the example returned %v", err)
	}

	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back the example returned %v", err)
	}

	for run := range 5 {
		if err := cellmanifest.Write(path, exampleManifest()); err != nil {
			t.Fatalf("writing the example on run %d returned %v", run, err)
		}

		again, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading back the example on run %d returned %v", run, err)
		}

		if string(again) != string(first) {
			t.Fatalf("run %d wrote different bytes:\n%s\nwant\n%s", run, again, first)
		}
	}

	wantHeader := "# Code generated by grpc-rust-tonic. DO NOT EDIT.\n"
	if !strings.HasPrefix(string(first), wantHeader) {
		t.Fatalf("got first line %q, want %q", strings.SplitN(string(first), "\n", 2)[0], wantHeader)
	}
}

func TestWriteLeavesTheManifestReadableByEveryoneAndWritableByItsOwner(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "cell", cellmanifest.FileName)

	if err := cellmanifest.Write(path, exampleManifest()); err != nil {
		t.Fatalf("writing the example returned %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stating the written manifest returned %v", err)
	}

	if info.Mode().Perm() != 0o644 {
		t.Fatalf("got mode %v, want %v", info.Mode().Perm(), os.FileMode(0o644))
	}
}

func TestReadReturnsWhatWriteWrote(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), cellmanifest.FileName)

	if err := cellmanifest.Write(path, exampleManifest()); err != nil {
		t.Fatalf("writing the example returned %v", err)
	}

	back, err := cellmanifest.Read(path)
	if err != nil {
		t.Fatalf("reading the example returned %v", err)
	}

	parsed, err := cellmanifest.Parse([]byte(grpcCellExample))
	if err != nil {
		t.Fatalf("parsing the example returned %v", err)
	}

	if back.Cell != parsed.Cell || back.Generator != parsed.Generator {
		t.Fatalf("got cell %q generator %q, want cell %q generator %q",
			back.Cell, back.Generator, parsed.Cell, parsed.Generator)
	}

	if back.Provides.Drivers[0].Config["addr"].Default != parsed.Provides.Drivers[0].Config["addr"].Default {
		t.Fatalf("got default %v, want %v",
			back.Provides.Drivers[0].Config["addr"].Default,
			parsed.Provides.Drivers[0].Config["addr"].Default)
	}
}

func TestReadRefusesAPathThatDoesNotExist(t *testing.T) {
	t.Parallel()

	_, err := cellmanifest.Read(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatal("reading a missing path returned no error")
	}

	if !strings.Contains(err.Error(), "reading cell manifest") {
		t.Fatalf("got %q, want it to contain %q", err.Error(), "reading cell manifest")
	}
}

func TestWriteRefusesAManifestValidateRefuses(t *testing.T) {
	t.Parallel()

	broken := exampleManifest()
	broken.Cell = ""

	err := cellmanifest.Write(filepath.Join(t.TempDir(), cellmanifest.FileName), broken)
	if err == nil {
		t.Fatal("writing a manifest with no cell name returned no error")
	}

	if !strings.Contains(err.Error(), "cell name is empty") {
		t.Fatalf("got %q, want it to contain %q", err.Error(), "cell name is empty")
	}
}

func TestFieldTypesHoldsTheFourTypesAConfigFieldMayCarry(t *testing.T) {
	t.Parallel()

	want := []string{"string", "integer", "boolean", "duration"}

	got := cellmanifest.FieldTypes()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
