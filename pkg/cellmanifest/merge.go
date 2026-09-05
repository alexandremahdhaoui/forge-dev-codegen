// Copyright 2024 Alexandre Mahdhaoui
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	RequiredPorts []RequiredPort
}

type RequiredPort struct {
	Trait string
	Cell  string
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

	requiredBy := map[string]string{}
	cellOfAdapter := map[string]string{}
	seenCell := map[string]bool{}

	for _, m := range manifests {
		if err := m.Validate(); err != nil {
			return Merged{}, fmt.Errorf("merging cell manifests: %w", err)
		}

		if seenCell[m.Cell] {
			return Merged{}, fmt.Errorf("cell %q is named by two manifests", m.Cell)
		}

		seenCell[m.Cell] = true

		merged.Cells = append(merged.Cells, m.Cell)

		if err := mergeDrivers(&merged, m); err != nil {
			return Merged{}, err
		}

		if err := mergeControllers(&merged, m); err != nil {
			return Merged{}, err
		}

		if err := mergePorts(&merged, m); err != nil {
			return Merged{}, err
		}

		if err := mergeAdapters(&merged, cellOfAdapter, m); err != nil {
			return Merged{}, err
		}

		for _, trait := range m.Requires.Ports {
			if _, taken := requiredBy[trait]; !taken {
				requiredBy[trait] = m.Cell
			}
		}
	}

	for _, trait := range sortedKeys(requiredBy) {
		merged.RequiredPorts = append(merged.RequiredPorts, RequiredPort{Trait: trait, Cell: requiredBy[trait]})
	}

	return merged, nil
}

func mergeDrivers(merged *Merged, m Manifest) error {
	for _, driver := range m.Provides.Drivers {
		if existing, taken := merged.Drivers[driver.Name]; taken {
			return fmt.Errorf("driver %q is %s", driver.Name, providedBy(existing.Cell, m.Cell))
		}

		merged.Drivers[driver.Name] = DriverEntry{Cell: m.Cell, Driver: driver}
	}

	return nil
}

func mergeControllers(merged *Merged, m Manifest) error {
	for _, controller := range m.Provides.Controllers {
		if existing, taken := merged.Controllers[controller.Trait]; taken {
			return fmt.Errorf(
				"controller trait %q is %s",
				controller.Trait, providedBy(existing.Cell, m.Cell),
			)
		}

		merged.Controllers[controller.Trait] = ControllerEntry{Cell: m.Cell, Controller: controller}
	}

	return nil
}

func mergeAdapters(merged *Merged, cellOfAdapter map[string]string, m Manifest) error {
	for _, adapter := range m.Provides.Adapters {
		if cell, taken := cellOfAdapter[adapter.Name]; taken {
			return fmt.Errorf("adapter %q is %s", adapter.Name, providedBy(cell, m.Cell))
		}

		cellOfAdapter[adapter.Name] = m.Cell
		merged.Adapters = append(merged.Adapters, AdapterEntry{Cell: m.Cell, Adapter: adapter})
	}

	return nil
}

func mergePorts(merged *Merged, m Manifest) error {
	for _, port := range m.Provides.Ports {
		existing, taken := merged.Ports[port.Trait]
		if taken {
			if existing.Port != port {
				return fmt.Errorf(
					"port trait %q is %s with a different module",
					port.Trait, providedBy(existing.Cell, m.Cell),
				)
			}

			if existing.Cell == m.Cell {
				return fmt.Errorf("port trait %q is %s", port.Trait, providedBy(existing.Cell, m.Cell))
			}

			continue
		}

		merged.Ports[port.Trait] = PortEntry{Cell: m.Cell, Port: port}
	}

	return nil
}

func providedBy(first, second string) string {
	if first == second {
		return fmt.Sprintf("provided twice by cell %q", first)
	}

	return fmt.Sprintf("provided by cell %q and by cell %q", first, second)
}

func sortedKeys[V any](set map[string]V) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}
