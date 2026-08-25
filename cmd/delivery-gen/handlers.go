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
// surface declares, from the shared concerns renderer.
// Delivery is the one concern with per-repo data - which binaries ship -
// so its surface carries them:
//
//	surface:
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

			repo, binaries, err := deliverySurface(input.Surface)
			if err != nil {
				return nil, fmt.Errorf("reading the surface for %q: %w", input.Name, err)
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

// deliverySurface reads the repo name and the binaries to ship out of the
// opaque surface.
func deliverySurface(surface map[string]interface{}) (string, []concerns.Binary, error) {
	repo, _ := surface["repo"].(string)
	if repo == "" {
		return "", nil, fmt.Errorf("surface.repo is required; the build steps name the module by it")
	}

	raw, ok := surface["binaries"].([]interface{})
	if !ok || len(raw) == 0 {
		return "", nil, fmt.Errorf("at least one surface.binaries entry is required")
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
