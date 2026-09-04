package cellmanifest_test

import (
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/cellmanifest"
)

func cell(name string, provides cellmanifest.Provides, requires ...string) cellmanifest.Manifest {
	return cellmanifest.Manifest{
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
		Requires: []string{controllerTrait},
	}
}

func TestMergeGathersWhatEveryCellProvides(t *testing.T) {
	t.Parallel()

	rest := cell("rest", cellmanifest.Provides{
		Drivers:     []cellmanifest.Driver{driver("rest", "HelloController")},
		Adapters:    []cellmanifest.Adapter{{Name: "greeting_sqlite", Type: "S", Implements: "GreetingStore"}},
		Controllers: []cellmanifest.Controller{{Trait: "HelloController", Impl: "HelloControllerImpl"}},
		Ports:       []cellmanifest.Port{{Trait: "GreetingStore"}},
	}, "Clock")

	grpc := cell("grpc", cellmanifest.Provides{
		Drivers:  []cellmanifest.Driver{driver("grpc", "HelloController")},
		Adapters: []cellmanifest.Adapter{{Name: "hello_grpc_client", Type: "C", Implements: "HelloClient"}},
		Ports:    []cellmanifest.Port{{Trait: "HelloClient"}},
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
			strings.Join(merged.RequiredPorts, ","), "Clock,Random"},
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
			Controllers: []cellmanifest.Controller{{Trait: "HelloController", Impl: "A"}},
		}),
		cell("grpc", cellmanifest.Provides{
			Controllers: []cellmanifest.Controller{{Trait: "HelloController", Impl: "B"}},
		}),
	}

	invalidCell := []cellmanifest.Manifest{
		cell("rest", cellmanifest.Provides{}),
		{Cell: "", Generator: "grpc-rust-tonic"},
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
			name:      "a cell that does not validate is refused before it is merged",
			manifests: invalidCell,
			message:   "merging cell manifests: cell name is empty",
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

func TestMergeKeepsTheFirstCellThatProvidesAPortTrait(t *testing.T) {
	t.Parallel()

	manifests := []cellmanifest.Manifest{
		cell("rest", cellmanifest.Provides{Ports: []cellmanifest.Port{{Trait: "GreetingStore"}}}),
		cell("grpc", cellmanifest.Provides{Ports: []cellmanifest.Port{{Trait: "GreetingStore"}}}),
	}

	merged, err := cellmanifest.Merge(manifests)
	if err != nil {
		t.Fatalf("merging two cells that name one port returned %v", err)
	}

	if merged.Ports["GreetingStore"].Cell != "rest" {
		t.Fatalf("got %q, want %q", merged.Ports["GreetingStore"].Cell, "rest")
	}
}
