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

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/grpcrust"
)

func NewHandlers() Handlers {
	return Handlers{
		Generate: func(_ context.Context, input GenerateInput) (*GenerateOutput, error) {
			if input.Kind != "grpc" {
				return nil, fmt.Errorf("emitting %q: grpc-rust-tonic fills the grpc cell only", input.Kind)
			}

			if input.Language != "" && input.Language != "rust" {
				return nil, fmt.Errorf("emitting for %q: grpc-rust-tonic generates rust only", input.Language)
			}

			files, err := grpcrust.Generate([]byte(input.ProtoSpec), grpcrust.Options{
				Service: input.Name,
				Cell:    layoutString(input.Layout, "cell"),
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
