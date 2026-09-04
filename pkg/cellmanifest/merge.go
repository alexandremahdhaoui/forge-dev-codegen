package cellmanifest

import (
	"fmt"
	"sort"
)

type Merged struct {
	Cells         []string
	Drivers       map[string]DriverEntry
	Controllers   map[string]ControllerEntry
	Ports         map[string]PortEntry
	Adapters      []AdapterEntry
	RequiredPorts []string
}

type DriverEntry struct {
	Cell   string
	Driver Driver
}

type ControllerEntry struct {
	Cell       string
	Controller Controller
}

type PortEntry struct {
	Cell string
	Port Port
}

type AdapterEntry struct {
	Cell    string
	Adapter Adapter
}

func Merge(manifests []Manifest) (Merged, error) {
	merged := Merged{
		Cells:       make([]string, 0, len(manifests)),
		Drivers:     map[string]DriverEntry{},
		Controllers: map[string]ControllerEntry{},
		Ports:       map[string]PortEntry{},
		Adapters:    []AdapterEntry{},
	}

	required := map[string]bool{}

	for _, m := range manifests {
		if err := m.Validate(); err != nil {
			return Merged{}, fmt.Errorf("merging cell manifests: %w", err)
		}

		merged.Cells = append(merged.Cells, m.Cell)

		if err := mergeDrivers(&merged, m); err != nil {
			return Merged{}, err
		}

		if err := mergeControllers(&merged, m); err != nil {
			return Merged{}, err
		}

		mergePorts(&merged, m)

		for _, adapter := range m.Provides.Adapters {
			merged.Adapters = append(merged.Adapters, AdapterEntry{Cell: m.Cell, Adapter: adapter})
		}

		for _, trait := range m.Requires.Ports {
			required[trait] = true
		}
	}

	merged.RequiredPorts = sortedKeys(required)

	return merged, nil
}

func mergeDrivers(merged *Merged, m Manifest) error {
	for _, driver := range m.Provides.Drivers {
		if existing, taken := merged.Drivers[driver.Name]; taken {
			return fmt.Errorf(
				"driver %q is provided by cell %q and by cell %q",
				driver.Name, existing.Cell, m.Cell,
			)
		}

		merged.Drivers[driver.Name] = DriverEntry{Cell: m.Cell, Driver: driver}
	}

	return nil
}

func mergeControllers(merged *Merged, m Manifest) error {
	for _, controller := range m.Provides.Controllers {
		if existing, taken := merged.Controllers[controller.Trait]; taken {
			return fmt.Errorf(
				"controller trait %q is provided by cell %q and by cell %q",
				controller.Trait, existing.Cell, m.Cell,
			)
		}

		merged.Controllers[controller.Trait] = ControllerEntry{Cell: m.Cell, Controller: controller}
	}

	return nil
}

func mergePorts(merged *Merged, m Manifest) {
	for _, port := range m.Provides.Ports {
		if _, taken := merged.Ports[port.Trait]; taken {
			continue
		}

		merged.Ports[port.Trait] = PortEntry{Cell: m.Cell, Port: port}
	}
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}
