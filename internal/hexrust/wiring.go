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

package hexrust

import (
	"fmt"
	"sort"

	"sigs.k8s.io/yaml"

	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/cellmanifest"
	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/rustname"
)

type Wiring struct {
	Binary  string                  `json:"binary"`
	Ports   map[string]WiringPort   `json:"ports,omitempty"`
	Drivers map[string]WiringDriver `json:"drivers,omitempty"`
}

type WiringPort struct {
	Default  string                     `json:"default"`
	Adapters map[string]WiringCandidate `json:"adapters"`
}

type WiringCandidate struct {
	Type     string                              `json:"type,omitempty"`
	Module   string                              `json:"module,omitempty"`
	Fallible bool                                `json:"fallible,omitempty"`
	Config   map[string]cellmanifest.ConfigField `json:"config,omitempty"`
}

type WiringDriver struct {
	Enabled bool `json:"enabled"`
}

func ParseWiring(doc []byte) (Wiring, error) {
	var w Wiring

	if err := yaml.UnmarshalStrict(doc, &w); err != nil {
		return Wiring{}, fmt.Errorf("reading the wiring: %w", err)
	}

	if err := w.validate(); err != nil {
		return Wiring{}, fmt.Errorf("reading the wiring: %w", err)
	}

	return w, nil
}

func (w Wiring) validate() error {
	if w.Binary == "" {
		return fmt.Errorf("binary is empty, it names the binary main lands in")
	}

	if !rustname.IsModuleName(rustname.Snake(w.Binary)) {
		return fmt.Errorf("binary %q is not a name Rust can spell as a module", w.Binary)
	}

	for _, trait := range sortedPortNames(w.Ports) {
		port := w.Ports[trait]

		if len(port.Adapters) == 0 {
			return fmt.Errorf("port %q names no candidate, a port needs at least one adapter", trait)
		}

		if port.Default == "" {
			return fmt.Errorf("port %q names no default, one candidate is the one main builds with no configuration", trait)
		}

		if _, taken := port.Adapters[port.Default]; !taken {
			return fmt.Errorf(
				"port %q has default %q, which names no candidate, the candidates are %s",
				trait, port.Default, list(candidateNames(port)),
			)
		}

		for _, name := range candidateNames(port) {
			candidate := port.Adapters[name]

			if candidate.Type == "" && candidate.Module != "" {
				return fmt.Errorf("port %q candidate %q names a module and no type", trait, name)
			}

			if candidate.Type != "" && candidate.Module == "" {
				return fmt.Errorf("port %q candidate %q names a type and no module", trait, name)
			}
		}
	}

	return nil
}

func candidateNames(port WiringPort) []string {
	names := make([]string, 0, len(port.Adapters))
	for name := range port.Adapters {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

func sortedPortNames(ports map[string]WiringPort) []string {
	names := make([]string, 0, len(ports))
	for name := range ports {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

func sortedDriverNames(drivers map[string]WiringDriver) []string {
	names := make([]string, 0, len(drivers))
	for name := range drivers {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}
