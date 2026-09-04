package forgelint_test

import (
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/forgelint"
)

func hasRule(findings []forgelint.Finding, rule string) bool {
	for _, f := range findings {
		if f.Rule == rule {
			return true
		}
	}
	return false
}

func findingFor(findings []forgelint.Finding, rule, path string) (forgelint.Finding, bool) {
	for _, f := range findings {
		if f.Rule == rule && f.Path == path {
			return f, true
		}
	}
	return forgelint.Finding{}, false
}

func TestCheckReturnsZeroFindingsOnACleanForgeYAML(t *testing.T) {
	findings, err := forgelint.Check(forgelint.Options{RootDir: "testdata/clean"})
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected zero findings, got %+v", findings)
	}
}

func TestCheckFlagsAGoSchemeEngineURI(t *testing.T) {
	findings, err := forgelint.Check(forgelint.Options{RootDir: "testdata/go-uri"})
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if !hasRule(findings, "forge-lint-go-uri") {
		t.Fatalf("expected a forge-lint-go-uri finding, got %+v", findings)
	}
}

func TestCheckFlagsAShCommandReferencingAMissingHackScript(t *testing.T) {
	findings, err := forgelint.Check(forgelint.Options{RootDir: "testdata/missing-hack-script"})
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if !hasRule(findings, "forge-lint-missing-hack-script") {
		t.Fatalf("expected a forge-lint-missing-hack-script finding, got %+v", findings)
	}
}

func TestCheckFlagsATestStageNamedUnitWithNoRunner(t *testing.T) {
	findings, err := forgelint.Check(forgelint.Options{RootDir: "testdata/unit-no-runner"})
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if !hasRule(findings, "forge-lint-unit-stage-missing-runner") {
		t.Fatalf("expected a forge-lint-unit-stage-missing-runner finding, got %+v", findings)
	}
}

func TestCheckFlagsAMissingArtifactStorePath(t *testing.T) {
	findings, err := forgelint.Check(forgelint.Options{RootDir: "testdata/missing-artifact-store"})
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if !hasRule(findings, "forge-lint-missing-artifact-store-path") {
		t.Fatalf("expected a forge-lint-missing-artifact-store-path finding, got %+v", findings)
	}
}

func TestCheckFlagsARustRepoUsingGoBuild(t *testing.T) {
	findings, err := forgelint.Check(forgelint.Options{RootDir: "testdata/rust-go-build"})
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if !hasRule(findings, "forge-lint-rust-uses-go-build") {
		t.Fatalf("expected a forge-lint-rust-uses-go-build finding, got %+v", findings)
	}
}

func TestCheckFlagsAnEngineURIWithoutForgeOrAliasScheme(t *testing.T) {
	findings, err := forgelint.Check(forgelint.Options{RootDir: "testdata/unknown-scheme"})
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if !hasRule(findings, "forge-lint-unknown-scheme") {
		t.Fatalf("expected a forge-lint-unknown-scheme finding, got %+v", findings)
	}
}

func TestCheckResolvesASingleHackTokenInsideAShDashCArgumentAgainstASiblingRepo(t *testing.T) {
	findings, err := forgelint.Check(forgelint.Options{RootDir: "testdata/resolve-spec-sibling/rootDir"})
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if hasRule(findings, "forge-lint-missing-hack-script") {
		t.Fatalf("expected the sibling hack/resolve-spec.sh to resolve and pass, got %+v", findings)
	}
}

func TestNoControllerAndNoDriverFileReachesAnAdapter(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		message string
	}{
		{
			name:    "a root controller file that names crate adapter is flagged",
			path:    "src/controller/greeting_controller.rs",
			message: `line 1 names "crate::adapter" which no controller or driver file may reach`,
		},
		{
			name:    "a root driver file that names super adapter is flagged",
			path:    "src/driver/zz_generated_http_driver.rs",
			message: `line 3 names "super::adapter" which no controller or driver file may reach`,
		},
		{
			name:    "a cell controller file that names the cell adapter is flagged",
			path:    "src/rest/controller/greeting_controller.rs",
			message: `line 2 names "crate::rest::adapter" which no controller or driver file may reach`,
		},
	}

	findings, err := forgelint.Check(forgelint.Options{RootDir: "testdata/rust-adapter-out-of-reach"})
	if err != nil {
		t.Fatalf("checking: %v", err)
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			finding, ok := findingFor(findings, "forge-lint-adapter-out-of-reach", test.path)
			if !ok {
				t.Fatalf("expected a forge-lint-adapter-out-of-reach finding for %s, got %+v", test.path, findings)
			}
			if finding.Message != test.message {
				t.Fatalf("expected message %q, got %q", test.message, finding.Message)
			}
		})
	}
}

func TestARustRepoWhoseControllerAndDriverStayOffTheAdapterPasses(t *testing.T) {
	findings, err := forgelint.Check(forgelint.Options{RootDir: "testdata/rust-clean"})
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected zero findings, got %+v", findings)
	}
}

func TestCheckReturnsAnErrorWhenForgeYAMLIsMissing(t *testing.T) {
	_, err := forgelint.Check(forgelint.Options{RootDir: "testdata/does-not-exist"})
	if err == nil {
		t.Fatalf("expected an error for a missing forge.yaml")
	}
}
