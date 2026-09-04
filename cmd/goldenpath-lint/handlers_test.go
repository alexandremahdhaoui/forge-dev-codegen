package main

import (
	"context"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
)

func TestRunFailsAndReportsTheFindingTextForADriverThatImportsAnAdapter(t *testing.T) {
	report, err := Run(context.Background(), mcptypes.RunInput{Stage: "unit"}, &Spec{
		Layout:  "go",
		RootDir: "../../internal/goldenpath/testdata/go-driver-imports-adapter",
	})
	if err != nil {
		t.Fatalf("running: %v", err)
	}
	if report.Status != "failed" {
		t.Fatalf("expected status failed, got %q", report.Status)
	}
	if !strings.Contains(report.ErrorMessage, "go-driver-imports-adapter") {
		t.Fatalf("expected the error message to name the finding, got %q", report.ErrorMessage)
	}
}

func TestRunPassesOnACleanGoLayout(t *testing.T) {
	report, err := Run(context.Background(), mcptypes.RunInput{Stage: "unit"}, &Spec{
		Layout:  "go",
		RootDir: "../../internal/goldenpath/testdata/go-clean",
	})
	if err != nil {
		t.Fatalf("running: %v", err)
	}
	if report.Status != "passed" {
		t.Fatalf("expected status passed, got %q: %s", report.Status, report.ErrorMessage)
	}
}
