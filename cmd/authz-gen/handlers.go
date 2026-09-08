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

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/authzgen"
)

func NewHandlers() Handlers {
	return Handlers{
		Generate: func(_ context.Context, input GenerateInput) (*GenerateOutput, error) {
			if input.Kind != "authz" {
				return nil, fmt.Errorf("emitting %q: authz-gen fills the authz cell only", input.Kind)
			}

			if input.WiringSpec == "" {
				return nil, fmt.Errorf("emitting the authz module of %q: the model carries no authz.yaml, name it under wiring.specPath", input.Name)
			}

			files, err := authzgen.Generate([]byte(input.WiringSpec))
			if err != nil {
				return nil, fmt.Errorf("emitting the authz module of %q: %w", input.Name, err)
			}

			out := make([]GeneratedFile, 0, len(files))
			for _, f := range files {
				out = append(out, GeneratedFile{Path: f.Path, Content: f.Content})
			}

			return &GenerateOutput{Files: out}, nil
		},
	}
}
