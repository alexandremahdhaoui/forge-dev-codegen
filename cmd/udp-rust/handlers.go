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

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/layoutports"
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

			ports, err := layoutports.Read(input.Layout, udprust.PortKinds())
			if err != nil {
				return nil, fmt.Errorf("emitting the skeleton of %q: %w", input.Name, err)
			}

			gate, err := layoutGate(input.Layout)
			if err != nil {
				return nil, fmt.Errorf("emitting the skeleton of %q: %w", input.Name, err)
			}

			files, err := udprust.Generate([]byte(input.ProtoSpec), udprust.Options{
				Service: input.Name,
				Cell:    layoutString(input.Layout, "cell"),
				Hello:   layoutString(input.Layout, "hello"),
				Push:    push,
				Ports:   ports,
				Gate:    gate,
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

func layoutGate(layout map[string]interface{}) (udprust.GateSpec, error) {
	raw, ok := layout["gate"]
	if !ok {
		return udprust.GateSpec{}, nil
	}

	fields, ok := raw.(map[string]interface{})
	if !ok {
		return udprust.GateSpec{}, fmt.Errorf("reading layout.gate: it is an object naming field and adapters, not %v", raw)
	}

	spec := udprust.GateSpec{}
	spec.Field, _ = fields["field"].(string)

	if _, spelled := fields["adapters"]; spelled {
		adapters, err := layoutStrings(fields, "adapters")
		if err != nil {
			return udprust.GateSpec{}, fmt.Errorf("reading layout.gate: %w", err)
		}

		spec.Adapters = &adapters
	}

	return spec, nil
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
