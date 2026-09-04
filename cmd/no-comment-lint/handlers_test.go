package main

import (
	"context"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
)

func TestRunFailsAndReportsTheFindingTextForADirtyGoTree(t *testing.T) {
	report, err := Run(context.Background(), mcptypes.RunInput{Stage: "unit"}, &Spec{
		RootDir:   "../../internal/nocomment/testdata/go",
		Languages: []string{"go"},
	})
	if err != nil {
		t.Fatalf("running: %v", err)
	}
	if report.Status != "failed" {
		t.Fatalf("expected status failed, got %q", report.Status)
	}
	if !strings.Contains(report.ErrorMessage, "dirty.go:3: // this explains the function") {
		t.Fatalf("expected the error message to name the finding, got %q", report.ErrorMessage)
	}
}

func TestRunPassesOnACleanTypeScriptTree(t *testing.T) {
	report, err := Run(context.Background(), mcptypes.RunInput{Stage: "unit"}, &Spec{
		RootDir:   "../../internal/nocomment/testdata/typescript",
		Languages: []string{"typescript"},
		Exclude:   []string{"dirty.ts"},
	})
	if err != nil {
		t.Fatalf("running: %v", err)
	}
	if report.Status != "passed" {
		t.Fatalf("expected status passed, got %q: %s", report.Status, report.ErrorMessage)
	}
}
