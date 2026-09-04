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
	"os"
	"path/filepath"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/vectorsrust"
)

func NewHandlers() Handlers {
	return Handlers{
		Generate: func(_ context.Context, input GenerateInput) (*GenerateOutput, error) {
			if input.Kind != "vectors" {
				return nil, fmt.Errorf("emitting %q: vectors-rust fills the vectors cell only", input.Kind)
			}

			if input.Language != "" && input.Language != "rust" {
				return nil, fmt.Errorf("emitting for %q: vectors-rust generates rust only", input.Language)
			}

			vectors, err := readVectors(input)
			if err != nil {
				return nil, err
			}

			files, err := vectorsrust.Generate([]byte(input.OpenapiSpec), vectors, vectorsrust.Options{
				Service:  input.Name,
				CrateDir: layoutString(input.Layout, "crateDir"),
				Cell:     layoutString(input.Layout, "cell"),
				RestCell: layoutString(input.Layout, "restCell"),
				Proto:    []byte(input.ProtoSpec),
			})
			if err != nil {
				return nil, fmt.Errorf("emitting the vectors of %q: %w", input.Name, err)
			}

			out := make([]GeneratedFile, 0, len(files))
			for _, f := range files {
				out = append(out, GeneratedFile{Path: f.Path, Content: f.Content})
			}

			return &GenerateOutput{Files: out}, nil
		},
	}
}

func readVectors(input GenerateInput) ([]byte, error) {
	if input.Vectors != "" {
		return []byte(input.Vectors), nil
	}

	rel := layoutString(input.Layout, "vectors")
	if rel == "" {
		return nil, fmt.Errorf("emitting the vectors of %q: the model carries no vectors document and layout.vectors names no file", input.Name)
	}

	doc, err := os.ReadFile(filepath.Join(input.SrcDir, rel))
	if err != nil {
		return nil, fmt.Errorf("reading the vectors document %s of %q: %w", rel, input.Name, err)
	}

	return doc, nil
}

func layoutString(layout map[string]interface{}, key string) string {
	v, _ := layout[key].(string)

	return v
}
