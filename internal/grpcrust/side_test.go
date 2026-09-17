package grpcrust_test

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/grpcrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/layoutports"
	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/cellmanifest"
)

func generateSide(t *testing.T, cell, side string) (map[string]string, cellmanifest.Manifest) {
	t.Helper()

	files, err := grpcrust.Generate([]byte(helloProto), grpcrust.Options{Service: "songe-social", Cell: cell, Side: side})
	if err != nil {
		t.Fatalf("generating side %s: %v", side, err)
	}

	byPath := map[string]string{}
	for _, f := range files {
		byPath[f.Path] = f.Content
	}

	m, err := cellmanifest.Parse([]byte(byPath[cellmanifest.FileName]))
	if err != nil {
		t.Fatalf("parsing the manifest: %v", err)
	}

	return byPath, m
}

func sortedPaths(byPath map[string]string) []string {
	paths := make([]string, 0, len(byPath))
	for p := range byPath {
		paths = append(paths, p)
	}

	sort.Strings(paths)

	return paths
}

func TestSideClientEmitsThePortTheAdapterAndTheTypesAndTheManifestProvidesNoDriverAndNoController(t *testing.T) {
	byPath, m := generateSide(t, "hello_client", grpcrust.SideClient)

	want := []string{
		"adapter/mod.rs",
		"adapter/zz_generated_hello_grpc_client.rs",
		"controller/mod.rs",
		"driver/mod.rs",
		"mod.rs",
		"port/mod.rs",
		"port/zz_generated_hello_client.rs",
		"proto/zz_generated_hello.proto",
		"types/mod.rs",
		"types/zz_generated_hello_messages.rs",
		"zz_generated_build.rs",
		"zz_generated_cell.yaml",
	}

	if got := sortedPaths(byPath); !reflect.DeepEqual(got, want) {
		t.Fatalf("emitted paths\n got %q\nwant %q", got, want)
	}

	if len(m.Provides.Drivers) != 0 || len(m.Provides.Controllers) != 0 || len(m.Requires.Ports) != 0 {
		t.Errorf("a client cell provides no driver and no controller and requires nothing, got %+v", m)
	}

	if len(m.Provides.Ports) != 1 || m.Provides.Ports[0].Trait != "HelloClient" || m.Provides.Ports[0].Module != "hello_client::port::hello_client" {
		t.Errorf("ports = %+v", m.Provides.Ports)
	}

	if len(m.Provides.Adapters) != 1 {
		t.Fatalf("adapters = %+v", m.Provides.Adapters)
	}

	adapter := m.Provides.Adapters[0]
	if adapter.Name != "hello_client_client" || adapter.Module != "hello_client::adapter::hello_grpc_client" || adapter.Implements != "HelloClient" {
		t.Errorf("adapter = %+v", adapter)
	}

	if _, declared := adapter.Config["endpoint"]; !declared {
		t.Errorf("the client adapter declares no endpoint, got %+v", adapter.Config)
	}

	build := byPath["zz_generated_build.rs"]
	for _, want := range []string{
		`["src/hello_client/proto/zz_generated_hello.proto"]`,
		`["src/hello_client/proto"]`,
		".build_client(true)",
		".build_server(false)",
	} {
		if !strings.Contains(build, want) {
			t.Errorf("the build script never carried %q:\n%s", want, build)
		}
	}

	if strings.Contains(byPath["controller/mod.rs"], "mod ") || strings.Contains(byPath["driver/mod.rs"], "mod ") {
		t.Errorf("a client cell mounts nothing under controller or driver:\n%s\n%s", byPath["controller/mod.rs"], byPath["driver/mod.rs"])
	}
}

func TestSideServerEmitsTheDriverTheControllerAndTheTypesAndNoClientPortOrAdapter(t *testing.T) {
	byPath, m := generateSide(t, "grpc", grpcrust.SideServer)

	want := []string{
		"adapter/mod.rs",
		"controller/mod.rs",
		"controller/zz_generated_hello_controller.rs",
		"driver/mod.rs",
		"driver/zz_generated_hello_grpc_driver.rs",
		"mod.rs",
		"port/mod.rs",
		"proto/zz_generated_hello.proto",
		"types/mod.rs",
		"types/zz_generated_hello_messages.rs",
		"zz_generated_build.rs",
		"zz_generated_cell.yaml",
	}

	if got := sortedPaths(byPath); !reflect.DeepEqual(got, want) {
		t.Fatalf("emitted paths\n got %q\nwant %q", got, want)
	}

	if len(m.Provides.Ports) != 0 || len(m.Provides.Adapters) != 0 {
		t.Errorf("a server cell provides no client port and no adapter, got %+v", m.Provides)
	}

	if len(m.Provides.Drivers) != 1 || len(m.Provides.Controllers) != 1 {
		t.Errorf("a server cell provides one driver and one controller, got %+v", m.Provides)
	}

	build := byPath["zz_generated_build.rs"]
	if !strings.Contains(build, ".build_client(false)") || !strings.Contains(build, ".build_server(true)") {
		t.Errorf("the server build script compiles the server only:\n%s", build)
	}
}

func TestAnAbsentSideIsBothAndMatchesTheExplicitBoth(t *testing.T) {
	absent, err := grpcrust.Generate([]byte(helloProto), grpcrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating with no side: %v", err)
	}

	both, err := grpcrust.Generate([]byte(helloProto), grpcrust.Options{Service: "songe-hello", Side: grpcrust.SideBoth})
	if err != nil {
		t.Fatalf("generating side both: %v", err)
	}

	if !reflect.DeepEqual(absent, both) {
		t.Fatal("an absent side emits something other than side both")
	}
}

func TestASideOutsideServerClientAndBothIsRefusedByName(t *testing.T) {
	_, err := grpcrust.Generate([]byte(helloProto), grpcrust.Options{Service: "songe-hello", Side: "core"})
	if err == nil || !strings.Contains(err.Error(), `side "core" is not one of server, client and both`) {
		t.Fatalf("want the side refused by name, got %v", err)
	}
}

func TestLayoutPortsOnAClientCellIsRefusedByName(t *testing.T) {
	_, err := grpcrust.Generate([]byte(helloProto), grpcrust.Options{
		Service: "songe-social",
		Cell:    "identity_client",
		Side:    grpcrust.SideClient,
		Ports:   []layoutports.Spec{{Name: "TicketStore"}},
	})
	if err == nil || !strings.Contains(err.Error(), `layout.ports names "TicketStore" on a client cell, a client cell holds no controller to consume a port`) {
		t.Fatalf("want the port refused by name, got %v", err)
	}
}

func TestTwoClientCellsOverTwoProtosShareNoModulePathInPortOrAdapter(t *testing.T) {
	identityFiles, err := grpcrust.Generate([]byte(helloProto), grpcrust.Options{Service: "songe-social", Cell: "hello_client", Side: grpcrust.SideClient})
	if err != nil {
		t.Fatalf("generating the hello client cell: %v", err)
	}

	authzFiles, err := grpcrust.Generate([]byte(rosterProto), grpcrust.Options{Service: "songe-social", Cell: "roster_client", Side: grpcrust.SideClient})
	if err != nil {
		t.Fatalf("generating the roster client cell: %v", err)
	}

	manifests := []cellmanifest.Manifest{}

	for _, files := range [][]grpcrust.File{identityFiles, authzFiles} {
		for _, f := range files {
			if f.Path != cellmanifest.FileName {
				continue
			}

			m, err := cellmanifest.Parse([]byte(f.Content))
			if err != nil {
				t.Fatalf("parsing a manifest: %v", err)
			}

			manifests = append(manifests, m)
		}
	}

	merged, err := cellmanifest.Merge(manifests)
	if err != nil {
		t.Fatalf("merging the two client cells: %v", err)
	}

	if len(merged.Ports) != 2 || len(merged.Adapters) != 2 {
		t.Fatalf("want two ports and two adapters, got %+v", merged)
	}

	if merged.Adapters[0].Adapter.Name == merged.Adapters[1].Adapter.Name {
		t.Fatalf("both adapters carry the name %q", merged.Adapters[0].Adapter.Name)
	}
}
