package main

import (
	"context"
	"fmt"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/chartgen"
)

func NewHandlers() Handlers {
	return Handlers{
		Generate: func(_ context.Context, input GenerateInput) (*GenerateOutput, error) {
			if input.Kind != "chart" {
				return nil, fmt.Errorf("emitting %q: chart-gen fills the chart cell only", input.Kind)
			}

			files, err := chartgen.Generate(chartgen.Options{
				OpenapiSpec: []byte(input.OpenapiSpec),
				SrcDir:      input.SrcDir,
			})
			if err != nil {
				return nil, fmt.Errorf("emitting the chart of %q: %w", input.Name, err)
			}

			out := make([]GeneratedFile, 0, len(files))
			for _, f := range files {
				out = append(out, GeneratedFile{Path: f.Path, Content: f.Content})
			}

			return &GenerateOutput{Files: out}, nil
		},
	}
}
