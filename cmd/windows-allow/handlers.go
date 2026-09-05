package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/windowsallow"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
	"github.com/google/uuid"
)

func Run(_ context.Context, input mcptypes.RunInput, spec *Spec) (*forge.TestReport, error) {
	startTime := time.Now()
	report := &forge.TestReport{
		ID:        uuid.New().String(),
		Stage:     input.Stage,
		StartTime: startTime,
	}

	fail := func(message string) (*forge.TestReport, error) {
		report.Status = "failed"
		report.ErrorMessage = message
		report.TestStats = forge.TestStats{Total: 1, Failed: 1}
		report.Duration = time.Since(startTime).Seconds()
		return report, nil
	}

	if spec == nil {
		return fail("windows-allow needs a spec with a source, a destination and a name")
	}

	rootDir := "."
	if spec.RootDir != "" {
		rootDir = spec.RootDir
	} else if input.RootDir != "" {
		rootDir = input.RootDir
	}

	destination := os.ExpandEnv(spec.Destination)
	if destination == "" {
		return fail("the destination is empty, set it or export the env var it names")
	}
	if spec.ProbeExpect == "" {
		return fail("the probe declares no expect string, name what an allowed build prints")
	}

	attempts := spec.Attempts
	if attempts == 0 {
		attempts = 8
	}
	keep := spec.Keep
	if keep == 0 {
		keep = 3
	}
	timeoutSeconds := spec.ProbeTimeoutSeconds
	if timeoutSeconds == 0 {
		timeoutSeconds = 8
	}

	req := windowsallow.Request{
		Source:      spec.Source,
		Destination: destination,
		Name:        spec.Name,
		Commit:      commitOf(rootDir),
		Attempts:    attempts,
		Keep:        keep,
		Probe: windowsallow.Probe{
			Args:           spec.ProbeArgs,
			Expect:         spec.ProbeExpect,
			TimeoutSeconds: timeoutSeconds,
		},
	}

	outcome, err := windowsallow.Allow(req, windowsallow.RealRunner)
	report.Duration = time.Since(startTime).Seconds()
	if err != nil {
		report.Status = "failed"
		report.ErrorMessage = err.Error()
		report.TestStats = forge.TestStats{Total: 1, Failed: 1}
		return report, nil
	}

	report.Status = "passed"
	report.TestStats = forge.TestStats{Total: 1, Passed: 1}
	fmt.Fprintf(os.Stderr, "windows-allow: smart app control allowed %s after %d build(s), run it from %s\n", outcome.AllowedName, outcome.Attempts, destination)
	if len(outcome.Pruned) > 0 {
		fmt.Fprintf(os.Stderr, "windows-allow: pruned %d old build(s) of %s\n", len(outcome.Pruned), spec.Name)
	}
	return report, nil
}

func commitOf(rootDir string) string {
	short := gitOutput(rootDir, "rev-parse", "--short", "HEAD")
	if short == "" {
		return "nogit"
	}
	if gitOutput(rootDir, "status", "--porcelain") != "" {
		return short + "-dirty"
	}
	return short
}

func gitOutput(rootDir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = rootDir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
