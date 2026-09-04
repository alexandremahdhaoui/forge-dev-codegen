package goldenpath_test

import (
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/goldenpath"
)

func hasRule(findings []goldenpath.Finding, rule string) bool {
	for _, f := range findings {
		if f.Rule == rule {
			return true
		}
	}
	return false
}

func TestCheckReturnsZeroFindingsOnACleanGoLayout(t *testing.T) {
	findings, err := goldenpath.Check(goldenpath.Options{
		Layout:  goldenpath.LayoutGo,
		RootDir: "testdata/go-clean",
	})
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected zero findings, got %+v", findings)
	}
}

func TestCheckFlagsMissingGoLayoutDirectories(t *testing.T) {
	findings, err := goldenpath.Check(goldenpath.Options{
		Layout:  goldenpath.LayoutGo,
		RootDir: "testdata/go-missing-dirs",
	})
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if !hasRule(findings, "go-layout") {
		t.Fatalf("expected a go-layout finding, got %+v", findings)
	}
}

func TestCheckFlagsADriverFileThatImportsAnAdapter(t *testing.T) {
	findings, err := goldenpath.Check(goldenpath.Options{
		Layout:  goldenpath.LayoutGo,
		RootDir: "testdata/go-driver-imports-adapter",
	})
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if !hasRule(findings, "go-driver-imports-adapter") {
		t.Fatalf("expected a go-driver-imports-adapter finding, got %+v", findings)
	}
}

func TestCheckFlagsAnAdapterFileThatImportsAnotherAdapter(t *testing.T) {
	findings, err := goldenpath.Check(goldenpath.Options{
		Layout:  goldenpath.LayoutGo,
		RootDir: "testdata/go-adapter-imports-adapter",
	})
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if !hasRule(findings, "go-adapter-imports-adapter") {
		t.Fatalf("expected a go-adapter-imports-adapter finding, got %+v", findings)
	}
}

func TestCheckReturnsZeroFindingsOnACleanRustCoreCrate(t *testing.T) {
	findings, err := goldenpath.Check(goldenpath.Options{
		Layout:  goldenpath.LayoutRustCoreApp,
		RootDir: "testdata/rust-core-clean",
	})
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected zero findings, got %+v", findings)
	}
}

func TestCheckFlagsAForbiddenImportInARustCoreCrate(t *testing.T) {
	findings, err := goldenpath.Check(goldenpath.Options{
		Layout:  goldenpath.LayoutRustCoreApp,
		RootDir: "testdata/rust-core-forbidden",
	})
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if !hasRule(findings, "rust-core-forbidden-usage") {
		t.Fatalf("expected a rust-core-forbidden-usage finding, got %+v", findings)
	}
}

func TestCheckReturnsZeroFindingsOnACleanRustAppCrate(t *testing.T) {
	findings, err := goldenpath.Check(goldenpath.Options{
		Layout:  goldenpath.LayoutRustCoreApp,
		RootDir: "testdata/rust-app-clean",
	})
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected zero findings, got %+v", findings)
	}
}

func TestCheckFlagsARustAppDriverThatImportsTheAdapterModule(t *testing.T) {
	findings, err := goldenpath.Check(goldenpath.Options{
		Layout:  goldenpath.LayoutRustCoreApp,
		RootDir: "testdata/rust-app-driver-imports-adapter",
	})
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if !hasRule(findings, "rust-driver-imports-adapter") {
		t.Fatalf("expected a rust-driver-imports-adapter finding, got %+v", findings)
	}
}

func TestCheckFlagsAMissingForgeDevYamlWhenZzGeneratedFilesExist(t *testing.T) {
	findings, err := goldenpath.Check(goldenpath.Options{
		Layout:  goldenpath.LayoutRustCoreApp,
		RootDir: "testdata/rust-missing-forge-dev-yaml",
	})
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if !hasRule(findings, "rust-forge-dev-yaml") {
		t.Fatalf("expected a rust-forge-dev-yaml finding, got %+v", findings)
	}
}

func TestCheckReturnsAnErrorForAnUnknownLayout(t *testing.T) {
	_, err := goldenpath.Check(goldenpath.Options{
		Layout:  "unknown",
		RootDir: "testdata/go-clean",
	})
	if err == nil {
		t.Fatalf("expected an error for an unknown layout")
	}
}
