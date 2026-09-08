package main

import (
	"context"
	"fmt"
	"slices"
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

var remedies = []struct {
	Change string
	Remedy string
}{
	{drift.Rewritten, "rewritten: the generators write other bytes now. read the new ones and commit them."},
	{drift.Written, "written: the repo neither tracks nor ignores this path. add it to .gitignore, or commit it."},
	{drift.Removed, "removed: the generators no longer write this path. drop it from git."},
}

func renderFindings(findings []drift.Finding) string {
	var details strings.Builder

	fmt.Fprintf(&details, "a build with no artifact store changed %d file(s) git tracks or does not ignore\n\n", len(findings))

	for _, finding := range findings {
		fmt.Fprintf(&details, "  - %s: %s\n", finding.Path, finding.Change)
	}

	details.WriteString("\nthis run already wrote them, so they sit in the tree now. an ordinary forge build " +
		"would skip every one of them, which is the hole this gate exists to catch.\n")

	for _, one := range remedies {
		if !slices.ContainsFunc(findings, func(f drift.Finding) bool { return f.Change == one.Change }) {
			continue
		}

		fmt.Fprintf(&details, "  %s\n", one.Remedy)
	}

	return details.String()
}
