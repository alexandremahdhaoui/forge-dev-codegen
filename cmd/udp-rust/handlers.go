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

package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/udprust"
)

func NewHandlers() Handlers {
	return Handlers{
		Generate: func(_ context.Context, input GenerateInput) (*GenerateOutput, error) {
			if input.Kind != "udp" {
				return nil, fmt.Errorf("emitting %q: udp-rust fills the udp cell only", input.Kind)
			}

			if input.Language != "" && input.Language != "rust" {
				return nil, fmt.Errorf("emitting for %q: udp-rust generates rust only", input.Language)
			}

			push, err := layoutStrings(input.Layout, "push")
			if err != nil {
				return nil, fmt.Errorf("emitting the skeleton of %q: %w", input.Name, err)
			}

			ports, err := layoutPorts(input.Layout)
			if err != nil {
				return nil, fmt.Errorf("emitting the skeleton of %q: %w", input.Name, err)
			}

			files, err := udprust.Generate([]byte(input.ProtoSpec), udprust.Options{
				Service: input.Name,
				Cell:    layoutString(input.Layout, "cell"),
				Hello:   layoutString(input.Layout, "hello"),
				Push:    push,
				Ports:   ports,
			})
			if err != nil {
				return nil, fmt.Errorf("emitting the skeleton of %q: %w", input.Name, err)
			}

			out := make([]GeneratedFile, 0, len(files))
			for _, f := range files {
				out = append(out, GeneratedFile{Path: f.Path, Content: f.Content})
			}

			return &GenerateOutput{Files: out, Manifest: true}, nil
		},
	}
}

func layoutString(layout map[string]interface{}, key string) string {
	v, _ := layout[key].(string)

	return v
}

func layoutPorts(layout map[string]interface{}) ([]udprust.PortSpec, error) {
	raw, ok := layout["ports"]
	if !ok {
		return nil, nil
	}

	entries, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("reading layout.ports: it is a list of port names or of name, kind and adapters entries, not %v", raw)
	}

	specs := make([]udprust.PortSpec, 0, len(entries))

	for i, entry := range entries {
		if name, ok := entry.(string); ok && name != "" {
			specs = append(specs, udprust.PortSpec{Name: name})

			continue
		}

		fields, ok := entry.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("reading layout.ports entry %d: it is a port name or a name, kind and adapters entry, not %v", i, entry)
		}

		if _, spelled := fields["methods"]; spelled {
			return nil, fmt.Errorf("reading layout.ports entry %d: it declares methods, a port declares the kind it is and the engine writes the methods, the kinds are %s", i, strings.Join(udprust.PortKinds(), ", "))
		}

		name, _ := fields["name"].(string)
		if name == "" {
			return nil, fmt.Errorf("reading layout.ports entry %d: it names no port", i)
		}

		spec := udprust.PortSpec{Name: name}
		spec.Kind, _ = fields["kind"].(string)

		if _, spelled := fields["adapters"]; spelled {
			adapters, err := layoutStrings(fields, "adapters")
			if err != nil {
				return nil, fmt.Errorf("reading layout.ports entry %d: %w", i, err)
			}

			spec.Adapters = &adapters
		}

		specs = append(specs, spec)
	}

	return specs, nil
}

func layoutStrings(layout map[string]interface{}, key string) ([]string, error) {
	raw, ok := layout[key]
	if !ok {
		return nil, nil
	}

	entries, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("reading layout.%s: it is a list of rpc names, not %v", key, raw)
	}

	names := make([]string, 0, len(entries))

	for i, entry := range entries {
		name, ok := entry.(string)
		if !ok || name == "" {
			return nil, fmt.Errorf("reading layout.%s entry %d: it is an rpc name, not %v", key, i, entry)
		}

		names = append(names, name)
	}

	return names, nil
}
