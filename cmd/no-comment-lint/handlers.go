package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/nocomment"
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

	opts := nocomment.Options{RootDir: rootDir}
	if spec != nil {
		opts.Languages = spec.Languages
		opts.Exclude = spec.Exclude
	}

	findings, err := nocomment.Scan(opts)
	duration := time.Since(startTime).Seconds()

	report := &forge.TestReport{
		ID:        uuid.New().String(),
		Stage:     input.Stage,
		StartTime: startTime,
		Duration:  duration,
	}

	if err != nil {
		report.Status = "failed"
		report.ErrorMessage = fmt.Sprintf("scanning %q for comments: %v", rootDir, err)
		return report, nil
	}

	report.TestStats = forge.TestStats{
		Total:  1,
		Passed: 1,
	}

	if len(findings) > 0 {
		report.Status = "failed"
		report.TestStats = forge.TestStats{Total: 1, Failed: 1}

		var details strings.Builder
		fmt.Fprintf(&details, "Found %d comment(s) outside their allowed exemptions\n\n", len(findings))
		for _, f := range findings {
			fmt.Fprintf(&details, "  - %s:%d: %s\n", f.File, f.Line, f.Text)
		}
		report.ErrorMessage = details.String()
		return report, nil
	}

	report.Status = "passed"
	return report, nil
}
