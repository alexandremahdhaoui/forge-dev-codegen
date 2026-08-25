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

// NewHandlers wires the generate tool: one concern, four languages, the
// shared concerns renderer, so two cells naming one concern cannot drift.
func NewHandlers() Handlers {
	return Handlers{
		Generate: func(_ context.Context, input GenerateInput) (*GenerateOutput, error) {
			if input.Kind != "telemetry" {
				return nil, fmt.Errorf("emitting %q: telemetry-gen fills the telemetry cell only", input.Kind)
			}

			lang := concerns.Language(input.Language)
			if input.Language == "" {
				lang = concerns.LangGo
			}

			files, err := concerns.EmitTelemetry(lang)
			if err != nil {
				return nil, fmt.Errorf("emitting telemetry for %q: %w", input.Name, err)
			}

			out := make([]GeneratedFile, 0, len(files))
			for _, f := range files {
				out = append(out, GeneratedFile{Path: f.Path, Content: f.Content})
			}

			return &GenerateOutput{Files: out}, nil
		},
	}
}
