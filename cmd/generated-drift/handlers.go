package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/drift"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
	"github.com/google/uuid"
)

func Run(_ context.Context, input mcptypes.RunInput, spec *Spec) (*forge.TestReport, error) {
	startTime := time.Now()

	rootDir := "."
	if input.RootDir != "" {
		rootDir = input.RootDir
	}

	if spec != nil && spec.RootDir != "" {
		rootDir = spec.RootDir
	}

	findings, err := drift.Check(drift.Options{RootDir: rootDir})

	report := &forge.TestReport{
		ID:        uuid.New().String(),
		Stage:     input.Stage,
		StartTime: startTime,
		Duration:  time.Since(startTime).Seconds(),
		TestStats: forge.TestStats{Total: 1, Failed: 1},
		Status:    "failed",
	}

	if err != nil {
		report.ErrorMessage = fmt.Sprintf("checking %q for generated drift: %v", rootDir, err)

		return report, nil
	}

	if len(findings) > 0 {
		report.ErrorMessage = renderFindings(findings)

		return report, nil
	}

	report.Status = "passed"
	report.TestStats = forge.TestStats{Total: 1, Passed: 1}

	return report, nil
}

func renderFindings(findings []drift.Finding) string {
	var details strings.Builder

	fmt.Fprintf(&details, "a build with no artifact store changed %d file(s) git tracks or does not ignore\n\n", len(findings))

	for _, finding := range findings {
		fmt.Fprintf(&details, "  - %s: %s\n", finding.Path, finding.Change)
	}

	details.WriteString("\nthe generators no longer write what is committed. run forge build and commit the result.\n")

	return details.String()
}
