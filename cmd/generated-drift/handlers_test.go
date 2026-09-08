package main

import (
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/drift"
)

func TestTheReportSaysTheRunAlreadyWroteTheFilesItNames(t *testing.T) {
	rendered := renderFindings([]drift.Finding{{Path: "src/zz_generated.rs", Change: drift.Rewritten}})

	if !strings.Contains(rendered, "src/zz_generated.rs: rewritten") {
		t.Fatalf("expected the path and the change, got %q", rendered)
	}

	if !strings.Contains(rendered, "sit in the tree now") {
		t.Fatalf("expected the report to say the files are already written, got %q", rendered)
	}
}

func TestTheReportTellsAWrittenFileToGoIntoGitignoreAndNotIntoACommitAlone(t *testing.T) {
	rendered := renderFindings([]drift.Finding{{Path: "build/probe", Change: drift.Written}})

	if !strings.Contains(rendered, "add it to .gitignore") {
		t.Fatalf("expected the gitignore remedy, got %q", rendered)
	}

	if strings.Contains(rendered, "the generators write other bytes now") {
		t.Fatalf("expected no rewritten remedy for a written file, got %q", rendered)
	}

	if strings.Contains(rendered, "drop it from git") {
		t.Fatalf("expected no removed remedy for a written file, got %q", rendered)
	}
}

func TestTheReportGivesEachChangeItsOwnRemedyInOneOrder(t *testing.T) {
	rendered := renderFindings([]drift.Finding{
		{Path: "a", Change: drift.Written},
		{Path: "b", Change: drift.Removed},
		{Path: "c", Change: drift.Rewritten},
	})

	order := []string{
		"the generators write other bytes now",
		"add it to .gitignore",
		"drop it from git",
	}

	at := 0

	for _, want := range order {
		found := strings.Index(rendered[at:], want)
		if found < 0 {
			t.Fatalf("expected %q after position %d, got %q", want, at, rendered)
		}

		at += found
	}
}
