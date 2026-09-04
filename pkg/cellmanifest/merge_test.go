package cellmanifest_test

import (
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/cellmanifest"
)

func cell(name string, provides cellmanifest.Provides, requires ...string) cellmanifest.Manifest {
	return cellmanifest.Manifest{
		Version:   cellmanifest.Version,
		Cell:      name,
		Generator: name + "-generator",
		Provides:  provides,
		Requires:  cellmanifest.Requires{Ports: requires},
	}
}

func driver(name, controllerTrait string) cellmanifest.Driver {
	return cellmanifest.Driver{
		Name:     name,
		Type:     "Driver",
		Module:   "driver::" + name,
		Requires: []string{controllerTrait},
	}
}

func adapter(name, portTrait string) cellmanifest.Adapter {
	return cellmanifest.Adapter{
		Name:       name,
		Type:       "Adapter",
		Module:     "adapter::" + name,
		Implements: portTrait,
	}
}

func controller(trait, impl string) cellmanifest.Controller {
	return cellmanifest.Controller{
		Trait:  trait,
		Impl:   impl,
		Module: "controller::hello_controller",
	}
}

func port(trait, module string) cellmanifest.Port {
	return cellmanifest.Port{Trait: trait, Module: module}
}

func joinRequiredPorts(ports []cellmanifest.RequiredPort) string {
	traits := make([]string, 0, len(ports))
	for _, p := range ports {
		traits = append(traits, p.Trait)
	}

	return strings.Join(traits, ",")
}

func TestMergeGathersWhatEveryCellProvides(t *testing.T) {
	t.Parallel()

	rest := cell("rest", cellmanifest.Provides{
		Drivers:     []cellmanifest.Driver{driver("rest", "HelloController")},
		Adapters:    []cellmanifest.Adapter{adapter("greeting_sqlite", "GreetingStore")},
		Controllers: []cellmanifest.Controller{controller("HelloController", "HelloControllerImpl")},
		Ports:       []cellmanifest.Port{port("GreetingStore", "rest::port::greeting_store")},
	}, "Clock")

	grpc := cell("grpc", cellmanifest.Provides{
		Drivers:  []cellmanifest.Driver{driver("grpc", "HelloController")},
		Adapters: []cellmanifest.Adapter{adapter("hello_grpc_client", "HelloClient")},
		Ports:    []cellmanifest.Port{port("HelloClient", "grpc::port::hello_client")},
	}, "Clock", "Random")

	merged, err := cellmanifest.Merge([]cellmanifest.Manifest{rest, grpc})
	if err != nil {
		t.Fatalf("merging two cells returned %v", err)
	}

	cases := []struct {
		name string
		got  any
		want any
	}{
		{"the cells keep the order they were given in", strings.Join(merged.Cells, ","), "rest,grpc"},
		{"both drivers are gathered", len(merged.Drivers), 2},
		{"the rest driver belongs to the rest cell", merged.Drivers["rest"].Cell, "rest"},
		{"the grpc driver belongs to the grpc cell", merged.Drivers["grpc"].Cell, "grpc"},
		{"the one controller trait is gathered", len(merged.Controllers), 1},
		{"the controller trait belongs to the rest cell", merged.Controllers["HelloController"].Cell, "rest"},
		{"both adapters are gathered", len(merged.Adapters), 2},
		{"the first adapter belongs to the rest cell", merged.Adapters[0].Cell, "rest"},
		{"both ports are gathered", len(merged.Ports), 2},
		{"the greeting store port belongs to the rest cell", merged.Ports["GreetingStore"].Cell, "rest"},
		{"the required ports are sorted and deduplicated",
			joinRequiredPorts(merged.RequiredPorts), "Clock,Random"},
		{"a required port names the first cell that requires it",
			merged.RequiredPorts[0].Cell, "rest"},
		{"a port only the second cell requires names that cell",
			merged.RequiredPorts[1].Cell, "grpc"},
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

func TestMergeOfNoCellsGathersNothing(t *testing.T) {
	t.Parallel()

	merged, err := cellmanifest.Merge(nil)
	if err != nil {
		t.Fatalf("merging no cell returned %v", err)
	}

	if len(merged.Cells) != 0 || len(merged.Drivers) != 0 || len(merged.RequiredPorts) != 0 {
		t.Fatalf("got %+v, want an empty merge", merged)
	}
}

func TestMergeNamesBothCellsWhenTwoProvideOneThing(t *testing.T) {
	t.Parallel()

	sameDriverName := []cellmanifest.Manifest{
		cell("rest", cellmanifest.Provides{Drivers: []cellmanifest.Driver{driver("hello", "HelloController")}}),
		cell("grpc", cellmanifest.Provides{Drivers: []cellmanifest.Driver{driver("hello", "HelloController")}}),
	}

	sameControllerTrait := []cellmanifest.Manifest{
		cell("rest", cellmanifest.Provides{
			Controllers: []cellmanifest.Controller{controller("HelloController", "A")},
		}),
		cell("grpc", cellmanifest.Provides{
			Controllers: []cellmanifest.Controller{controller("HelloController", "B")},
		}),
	}

	sameAdapterName := []cellmanifest.Manifest{
		cell("rest", cellmanifest.Provides{
			Adapters: []cellmanifest.Adapter{adapter("greeting_store", "GreetingStore")},
		}),
		cell("grpc", cellmanifest.Provides{
			Adapters: []cellmanifest.Adapter{adapter("greeting_store", "HelloClient")},
		}),
	}

	sameCellName := []cellmanifest.Manifest{
		cell("rest", cellmanifest.Provides{}),
		cell("rest", cellmanifest.Provides{}),
	}

	cases := []struct {
		name      string
		manifests []cellmanifest.Manifest
		message   string
	}{
		{
			name:      "two cells providing one driver name are refused by name",
			manifests: sameDriverName,
			message:   `driver "hello" is provided by cell "rest" and by cell "grpc"`,
		},
		{
			name:      "two cells providing one controller trait are refused by name",
			manifests: sameControllerTrait,
			message:   `controller trait "HelloController" is provided by cell "rest" and by cell "grpc"`,
		},
		{
			name:      "two cells providing one adapter name are refused by name",
			manifests: sameAdapterName,
			message:   `adapter "greeting_store" is provided by cell "rest" and by cell "grpc"`,
		},
		{
			name:      "two manifests naming one cell are refused",
			manifests: sameCellName,
			message:   `cell "rest" is named by two manifests`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := cellmanifest.Merge(tc.manifests)
			if err == nil {
				t.Fatal("merging returned no error")
			}

			if err.Error() != tc.message {
				t.Fatalf("got %q, want %q", err.Error(), tc.message)
			}
		})
	}
}

func TestMergeRefusesACellThatDoesNotValidateBeforeItIsMerged(t *testing.T) {
	t.Parallel()

	manifests := []cellmanifest.Manifest{
		cell("rest", cellmanifest.Provides{}),
		{Version: cellmanifest.Version, Cell: "", Generator: "grpc-rust-tonic"},
	}

	_, err := cellmanifest.Merge(manifests)
	if err == nil {
		t.Fatal("merging a cell with no name returned no error")
	}

	want := "merging cell manifests: cell name is empty"
	if err.Error() != want {
		t.Fatalf("got %q, want %q", err.Error(), want)
	}
}

func TestMergeSaysTwiceWhenOneCellProvidesOneThingTwice(t *testing.T) {
	t.Parallel()

	twoDrivers := []cellmanifest.Manifest{
		cell("rest", cellmanifest.Provides{Drivers: []cellmanifest.Driver{
			driver("hello", "HelloController"),
			driver("hello", "HelloController"),
		}}),
	}

	twoControllers := []cellmanifest.Manifest{
		cell("rest", cellmanifest.Provides{Controllers: []cellmanifest.Controller{
			controller("HelloController", "A"),
			controller("HelloController", "B"),
		}}),
	}

	twoAdapters := []cellmanifest.Manifest{
		cell("rest", cellmanifest.Provides{Adapters: []cellmanifest.Adapter{
			adapter("greeting_store", "GreetingStore"),
			adapter("greeting_store", "HelloClient"),
		}}),
	}

	twoPorts := []cellmanifest.Manifest{
		cell("rest", cellmanifest.Provides{Ports: []cellmanifest.Port{
			port("GreetingStore", "rest::port::greeting_store"),
			port("GreetingStore", "grpc::port::greeting_store"),
		}}),
	}

	twoIdenticalPorts := []cellmanifest.Manifest{
		cell("rest", cellmanifest.Provides{Ports: []cellmanifest.Port{
			port("GreetingStore", "rest::port::greeting_store"),
			port("GreetingStore", "rest::port::greeting_store"),
		}}),
	}

	cases := []struct {
		name      string
		manifests []cellmanifest.Manifest
		message   string
	}{
		{
			name:      "one cell providing one port trait twice from one module names the cell once",
			manifests: twoIdenticalPorts,
			message:   `port trait "GreetingStore" is provided twice by cell "rest"`,
		},
		{
			name:      "one cell providing one driver name twice names the cell once",
			manifests: twoDrivers,
			message:   `driver "hello" is provided twice by cell "rest"`,
		},
		{
			name:      "one cell providing one controller trait twice names the cell once",
			manifests: twoControllers,
			message:   `controller trait "HelloController" is provided twice by cell "rest"`,
		},
		{
			name:      "one cell providing one adapter name twice names the cell once",
			manifests: twoAdapters,
			message:   `adapter "greeting_store" is provided twice by cell "rest"`,
		},
		{
			name:      "one cell providing one port trait twice from two modules names the cell once",
			manifests: twoPorts,
			message: `port trait "GreetingStore" is provided twice by cell "rest" ` +
				`with a different module`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := cellmanifest.Merge(tc.manifests)
			if err == nil {
				t.Fatal("merging returned no error")
			}

			if err.Error() != tc.message {
				t.Fatalf("got %q, want %q", err.Error(), tc.message)
			}
		})
	}
}

func TestMergeRefusesTwoCellsThatProvideOnePortTraitFromDifferentModules(t *testing.T) {
	t.Parallel()

	manifests := []cellmanifest.Manifest{
		cell("rest", cellmanifest.Provides{
			Ports: []cellmanifest.Port{port("GreetingStore", "rest::port::greeting_store")},
		}),
		cell("grpc", cellmanifest.Provides{
			Ports: []cellmanifest.Port{port("GreetingStore", "grpc::port::greeting_store")},
		}),
	}

	_, err := cellmanifest.Merge(manifests)
	if err == nil {
		t.Fatal("merging two cells that name one port from two modules returned no error")
	}

	want := `port trait "GreetingStore" is provided by cell "rest" and by cell "grpc" with a different module`
	if err.Error() != want {
		t.Fatalf("got %q, want %q", err.Error(), want)
	}
}

func TestMergeAcceptsTwoCellsThatProvideTheSamePort(t *testing.T) {
	t.Parallel()

	manifests := []cellmanifest.Manifest{
		cell("rest", cellmanifest.Provides{
			Ports: []cellmanifest.Port{port("GreetingStore", "rest::port::greeting_store")},
		}),
		cell("grpc", cellmanifest.Provides{
			Ports: []cellmanifest.Port{port("GreetingStore", "rest::port::greeting_store")},
		}),
	}

	merged, err := cellmanifest.Merge(manifests)
	if err != nil {
		t.Fatalf("merging two cells that name the same port returned %v", err)
	}

	if len(merged.Ports) != 1 {
		t.Fatalf("got %d ports, want 1", len(merged.Ports))
	}

	if merged.Ports["GreetingStore"].Cell != "rest" {
		t.Fatalf("got %q, want %q", merged.Ports["GreetingStore"].Cell, "rest")
	}
}
