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

package layoutports

import (
	"fmt"
	"strings"
)

type Spec struct {
	Name     string
	Kind     string
	Adapters *[]string
}

func Read(layout map[string]interface{}, kinds []string) ([]Spec, error) {
	raw, ok := layout["ports"]
	if !ok {
		return nil, nil
	}

	entries, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("reading layout.ports: it is a list of port names or of name, kind and adapters entries, not %v", raw)
	}

	specs := make([]Spec, 0, len(entries))

	for i, entry := range entries {
		spec, err := readEntry(entry, kinds)
		if err != nil {
			return nil, fmt.Errorf("reading layout.ports entry %d: %w", i, err)
		}

		specs = append(specs, spec)
	}

	return specs, nil
}

func readEntry(entry interface{}, kinds []string) (Spec, error) {
	if name, ok := entry.(string); ok && name != "" {
		return Spec{Name: name}, nil
	}

	fields, ok := entry.(map[string]interface{})
	if !ok {
		return Spec{}, fmt.Errorf("it is a port name or a name, kind and adapters entry, not %v", entry)
	}

	if _, spelled := fields["methods"]; spelled {
		return Spec{}, fmt.Errorf("it declares methods, a port declares the kind it is and the engine writes the methods, the kinds are %s", declaredKinds(kinds))
	}

	name, _ := fields["name"].(string)
	if name == "" {
		return Spec{}, fmt.Errorf("it names no port")
	}

	spec := Spec{Name: name}
	spec.Kind, _ = fields["kind"].(string)

	if _, spelled := fields["adapters"]; !spelled {
		return spec, nil
	}

	adapters, err := adapterKinds(fields["adapters"])
	if err != nil {
		return Spec{}, err
	}

	spec.Adapters = &adapters

	return spec, nil
}

func adapterKinds(raw interface{}) ([]string, error) {
	entries, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("its adapters is a list of adapter kinds, not %v", raw)
	}

	names := make([]string, 0, len(entries))

	for _, entry := range entries {
		name, ok := entry.(string)
		if !ok || name == "" {
			return nil, fmt.Errorf("an adapters entry is an adapter kind, not %v", entry)
		}

		names = append(names, name)
	}

	return names, nil
}

func declaredKinds(kinds []string) string {
	if len(kinds) == 0 {
		return "none, this cell writes no port of its own and every entry names a port another cell provides"
	}

	return strings.Join(kinds, ", ")
}
