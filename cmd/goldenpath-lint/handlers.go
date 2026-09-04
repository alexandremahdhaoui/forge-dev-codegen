package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/goldenpath"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
	"github.com/google/uuid"
)

func Run(_ context.Context, input mcptypes.RunInput, spec *Spec) (*forge.TestReport, error) {
	startTime := time.Now()

	rootDir := "."
	if spec != nil && spec.RootDir != "" {
		rootDir = spec.RootDir
	} else if input.RootDir != "" {
		rootDir = input.RootDir
	}

	layout := ""
	if spec != nil {
		layout = spec.Layout
	}

	findings, err := goldenpath.Check(goldenpath.Options{Layout: layout, RootDir: rootDir})
	duration := time.Since(startTime).Seconds()

	report := &forge.TestReport{
		ID:        uuid.New().String(),
		Stage:     input.Stage,
		StartTime: startTime,
		Duration:  duration,
	}

	if err != nil {
		report.Status = "failed"
		report.ErrorMessage = fmt.Sprintf("checking %q against the %q layout: %v", rootDir, layout, err)
		return report, nil
	}

	report.TestStats = forge.TestStats{Total: 1, Passed: 1}

	if len(findings) > 0 {
		report.Status = "failed"
		report.TestStats = forge.TestStats{Total: 1, Failed: 1}

		var details strings.Builder
		fmt.Fprintf(&details, "Found %d layout violation(s) against the %q layout\n\n", len(findings), layout)
		for _, f := range findings {
			fmt.Fprintf(&details, "  - [%s] %s: %s\n", f.Rule, f.Path, f.Message)
		}
		report.ErrorMessage = details.String()
		return report, nil
	}

	report.Status = "passed"
	return report, nil
}
