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

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/concerns"
)

// NewHandlers wires the generate tool: one Containerfile per binary the
// layout declares, from the shared concerns renderer.
// Delivery is the one concern with per-repo data. Its layout carries the
// binaries that ship:
//
//	layout:
//	  repo: golden-go
//	  binaries:
//	    - name: golden-go-server
//	      kind: server
func NewHandlers() Handlers {
	return Handlers{
		Generate: func(_ context.Context, input GenerateInput) (*GenerateOutput, error) {
			if input.Kind != "delivery" {
				return nil, fmt.Errorf("emitting %q: delivery-gen fills the delivery cell only", input.Kind)
			}

			lang := concerns.Language(input.Language)
			if input.Language == "" {
				lang = concerns.LangGo
			}

			repo, binaries, err := deliveryLayout(input.Layout)
			if err != nil {
				return nil, fmt.Errorf("reading the layout for %q: %w", input.Name, err)
			}

			files, err := concerns.EmitDelivery(lang, repo, binaries)
			if err != nil {
				return nil, fmt.Errorf("emitting delivery for %q: %w", input.Name, err)
			}

			out := make([]GeneratedFile, 0, len(files))
			for _, f := range files {
				out = append(out, GeneratedFile{Path: f.Path, Content: f.Content})
			}

			return &GenerateOutput{Files: out}, nil
		},
	}
}

// deliveryLayout reads the repo name and the binaries to ship out of the
// opaque layout.
func deliveryLayout(layout map[string]interface{}) (string, []concerns.Binary, error) {
	repo, _ := layout["repo"].(string)
	if repo == "" {
		return "", nil, fmt.Errorf("layout.repo is required; the build steps name the module by it")
	}

	raw, ok := layout["binaries"].([]interface{})
	if !ok || len(raw) == 0 {
		return "", nil, fmt.Errorf("at least one layout.binaries entry is required")
	}

	binaries := make([]concerns.Binary, 0, len(raw))

	for i, entry := range raw {
		m, ok := entry.(map[string]interface{})
		if !ok {
			return "", nil, fmt.Errorf("binaries entry %d is not an object", i)
		}

		name, _ := m["name"].(string)
		if name == "" {
			return "", nil, fmt.Errorf("binaries entry %d has no name", i)
		}

		kind, _ := m["kind"].(string)

		binaries = append(binaries, concerns.Binary{
			Name: name,
			Kind: kind,
		})
	}

	return repo, binaries, nil
}
