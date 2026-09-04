package nocomment_test

import (
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/nocomment"
)

func findingLines(t *testing.T, findings []nocomment.Finding) []int {
	t.Helper()
	lines := make([]int, 0, len(findings))
	for _, f := range findings {
		lines = append(lines, f.Line)
	}
	return lines
}

func TestScanFindsALineCommentInAGoFile(t *testing.T) {
	findings, err := nocomment.Scan(nocomment.Options{
		RootDir:   "testdata/go",
		Languages: []string{"go"},
	})
	if err != nil {
		t.Fatalf("scanning: %v", err)
	}

	var got []nocomment.Finding
	for _, f := range findings {
		if f.File == "dirty.go" {
			got = append(got, f)
		}
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 findings in dirty.go, got %d: %+v", len(got), got)
	}
}

func TestScanReportsNoFindingsForACleanGoFile(t *testing.T) {
	findings, err := nocomment.Scan(nocomment.Options{
		RootDir:   "testdata/go",
		Languages: []string{"go"},
	})
	if err != nil {
		t.Fatalf("scanning: %v", err)
	}

	for _, f := range findings {
		if f.File == "clean.go" {
			t.Fatalf("expected no findings in clean.go, got %+v", f)
		}
	}
}

func TestScanExemptsALicenseHeaderBlockStartingOnLineOne(t *testing.T) {
	findings, err := nocomment.Scan(nocomment.Options{
		RootDir:   "testdata/go",
		Languages: []string{"go"},
	})
	if err != nil {
		t.Fatalf("scanning: %v", err)
	}

	for _, f := range findings {
		if f.File == "license_header.go" {
			t.Fatalf("expected the license header to be exempt, got %+v", f)
		}
	}
}

func TestScanSkipsAFileMarkedCodeGeneratedByOnLineOne(t *testing.T) {
	findings, err := nocomment.Scan(nocomment.Options{
		RootDir:   "testdata/go",
		Languages: []string{"go"},
	})
	if err != nil {
		t.Fatalf("scanning: %v", err)
	}

	for _, f := range findings {
		if f.File == "generated.go" {
			t.Fatalf("expected the generated file to be skipped entirely, got %+v", f)
		}
	}
}

func TestScanIgnoresACommentMarkerInsideAStringLiteral(t *testing.T) {
	findings, err := nocomment.Scan(nocomment.Options{
		RootDir:   "testdata/go",
		Languages: []string{"go"},
	})
	if err != nil {
		t.Fatalf("scanning: %v", err)
	}

	for _, f := range findings {
		if f.File == "url_in_string.go" {
			t.Fatalf("expected the URL inside a string literal to not be a finding, got %+v", f)
		}
	}
}

func TestScanIgnoresCommentMarkersInsideAGoBacktickRawString(t *testing.T) {
	findings, err := nocomment.Scan(nocomment.Options{
		RootDir:   "testdata/go",
		Languages: []string{"go"},
	})
	if err != nil {
		t.Fatalf("scanning: %v", err)
	}

	for _, f := range findings {
		if f.File == "backtick_string.go" {
			t.Fatalf("expected the comment markers inside the backtick string to not be a finding, got %+v", f)
		}
	}
}

func TestScanFindsALineCommentInARustFile(t *testing.T) {
	findings, err := nocomment.Scan(nocomment.Options{
		RootDir:   "testdata/rust",
		Languages: []string{"rust"},
	})
	if err != nil {
		t.Fatalf("scanning: %v", err)
	}

	found := false
	for _, f := range findings {
		if f.File == "dirty.rs" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a finding in dirty.rs, got %+v", findings)
	}

	for _, f := range findings {
		if f.File == "clean.rs" {
			t.Fatalf("expected no findings in clean.rs, got %+v", f)
		}
		if f.File == "license_header.rs" {
			t.Fatalf("expected the license header to be exempt in license_header.rs, got %+v", f)
		}
	}
}

func TestScanFindsAHashCommentInAPythonFile(t *testing.T) {
	findings, err := nocomment.Scan(nocomment.Options{
		RootDir:   "testdata/python",
		Languages: []string{"python"},
	})
	if err != nil {
		t.Fatalf("scanning: %v", err)
	}

	found := false
	for _, f := range findings {
		if f.File == "dirty.py" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a finding in dirty.py, got %+v", findings)
	}
}

func TestScanExemptsAShebangOnLineOneInAPythonFile(t *testing.T) {
	findings, err := nocomment.Scan(nocomment.Options{
		RootDir:   "testdata/python",
		Languages: []string{"python"},
	})
	if err != nil {
		t.Fatalf("scanning: %v", err)
	}

	for _, f := range findings {
		if f.File == "shebang.py" {
			t.Fatalf("expected the shebang to be exempt in shebang.py, got %+v", f)
		}
	}
}

func TestScanFindsALineCommentInATypeScriptFile(t *testing.T) {
	findings, err := nocomment.Scan(nocomment.Options{
		RootDir:   "testdata/typescript",
		Languages: []string{"typescript"},
	})
	if err != nil {
		t.Fatalf("scanning: %v", err)
	}

	found := false
	for _, f := range findings {
		if f.File == "dirty.ts" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a finding in dirty.ts, got %+v", findings)
	}
	for _, f := range findings {
		if f.File == "clean.ts" {
			t.Fatalf("expected no findings in clean.ts, got %+v", f)
		}
	}
}

func TestScanExcludesFilesMatchingTheDefaultZzGeneratedGlobs(t *testing.T) {
	findings, err := nocomment.Scan(nocomment.Options{
		RootDir:   "testdata/excluded",
		Languages: []string{"go"},
	})
	if err != nil {
		t.Fatalf("scanning: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected the default excludes to drop every fixture, got %+v", findings)
	}
}

func TestScanReturnsZeroFindingsOnACleanTree(t *testing.T) {
	findings, err := nocomment.Scan(nocomment.Options{
		RootDir:   "testdata/typescript",
		Languages: []string{"typescript"},
		Exclude:   []string{"dirty.ts"},
	})
	if err != nil {
		t.Fatalf("scanning: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected zero findings, got %+v", findings)
	}
}
