package main

import (
	"context"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
)

func TestRunFailsAndReportsTheFindingTextForAGoSchemeEngineURI(t *testing.T) {
	report, err := Run(context.Background(), mcptypes.RunInput{Stage: "unit"}, &Spec{
		RootDir: "../../internal/forgelint/testdata/go-uri",
	})
	if err != nil {
		t.Fatalf("running: %v", err)
	}
	if report.Status != "failed" {
		t.Fatalf("expected status failed, got %q", report.Status)
	}
	if !strings.Contains(report.ErrorMessage, "forge-lint-go-uri") {
		t.Fatalf("expected the error message to name the finding, got %q", report.ErrorMessage)
	}
}

func TestRunPassesOnACleanForgeYAML(t *testing.T) {
	report, err := Run(context.Background(), mcptypes.RunInput{Stage: "unit"}, &Spec{
		RootDir: "../../internal/forgelint/testdata/clean",
	})
	if err != nil {
		t.Fatalf("running: %v", err)
	}
	if report.Status != "passed" {
		t.Fatalf("expected status passed, got %q: %s", report.Status, report.ErrorMessage)
	}
}
