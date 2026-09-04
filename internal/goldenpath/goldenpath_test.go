package goldenpath_test

import (
	"fmt"
	"strings"
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

func findingFor(findings []goldenpath.Finding, rule, path string) (goldenpath.Finding, bool) {
	for _, f := range findings {
		if f.Rule == rule && f.Path == path {
			return f, true
		}
	}
	return goldenpath.Finding{}, false
}

func check(t *testing.T, layout, rootDir string) []goldenpath.Finding {
	t.Helper()

	findings, err := goldenpath.Check(goldenpath.Options{Layout: layout, RootDir: rootDir})
	if err != nil {
		t.Fatalf("checking %s: %v", rootDir, err)
	}

	return findings
}

func TestACleanGoRepoAndACleanRustCrateCarryNoFinding(t *testing.T) {
	tests := []struct {
		name    string
		layout  string
		rootDir string
	}{
		{
			name:    "a go repo with the three layers and no crossed import passes",
			layout:  goldenpath.LayoutGo,
			rootDir: "testdata/go-clean",
		},
		{
			name:    "a single crate with layers, a cell and one user impl file passes",
			layout:  goldenpath.LayoutRust,
			rootDir: "testdata/rust-clean",
		},
		{
			name:    "the old rust-core-app spelling still names the single crate layout",
			layout:  goldenpath.LayoutRustCoreApp,
			rootDir: "testdata/rust-clean",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			findings := check(t, test.layout, test.rootDir)
			if len(findings) != 0 {
				t.Fatalf("expected zero findings, got %+v", findings)
			}
		})
	}
}

func TestEveryRuleFlagsTheTreeThatBreaksIt(t *testing.T) {
	tests := []struct {
		name    string
		layout  string
		rootDir string
		rule    string
	}{
		{
			name:    "a go repo missing a layer directory is flagged",
			layout:  goldenpath.LayoutGo,
			rootDir: "testdata/go-missing-dirs",
			rule:    "go-layout",
		},
		{
			name:    "a go driver that imports an adapter is flagged",
			layout:  goldenpath.LayoutGo,
			rootDir: "testdata/go-driver-imports-adapter",
			rule:    "go-driver-imports-adapter",
		},
		{
			name:    "a go adapter that imports another adapter is flagged",
			layout:  goldenpath.LayoutGo,
			rootDir: "testdata/go-adapter-imports-adapter",
			rule:    "go-adapter-imports-adapter",
		},
		{
			name:    "a crate with no Cargo.toml is flagged",
			layout:  goldenpath.LayoutRust,
			rootDir: "testdata/rust-missing-cargo-toml",
			rule:    "rust-cargo-toml",
		},
		{
			name:    "a crate with no src lib.rs is flagged",
			layout:  goldenpath.LayoutRust,
			rootDir: "testdata/rust-missing-lib-rs",
			rule:    "rust-lib-rs",
		},
		{
			name:    "a crate with no forge.yaml is flagged",
			layout:  goldenpath.LayoutRust,
			rootDir: "testdata/rust-missing-forge-yaml",
			rule:    "rust-forge-yaml",
		},
		{
			name:    "a crate holding zz_generated files with no forge-dev.yaml is flagged",
			layout:  goldenpath.LayoutRust,
			rootDir: "testdata/rust-missing-forge-dev-yaml",
			rule:    "rust-forge-dev-yaml",
		},
		{
			name:    "a directory under src that is neither a layer nor a cell is flagged",
			layout:  goldenpath.LayoutRust,
			rootDir: "testdata/rust-src-not-a-layer",
			rule:    "rust-src-layout",
		},
		{
			name:    "a directory inside a cell that is not a layer is flagged",
			layout:  goldenpath.LayoutRust,
			rootDir: "testdata/rust-cell-not-a-layer",
			rule:    "rust-cell-layout",
		},
		{
			name:    "a cell holding zz_generated files with no cell manifest is flagged",
			layout:  goldenpath.LayoutRust,
			rootDir: "testdata/rust-missing-cell-manifest",
			rule:    "rust-cell-manifest",
		},
		{
			name:    "an io crate on a use line of a pure layer is flagged",
			layout:  goldenpath.LayoutRust,
			rootDir: "testdata/rust-io-use",
			rule:    "rust-io-use",
		},
		{
			name:    "a user file no generated mod line reaches is flagged",
			layout:  goldenpath.LayoutRust,
			rootDir: "testdata/rust-file-not-mounted",
			rule:    "rust-file-not-mounted",
		},
		{
			name:    "a path attribute under src is flagged",
			layout:  goldenpath.LayoutRust,
			rootDir: "testdata/rust-path-attribute",
			rule:    "rust-no-path-attribute",
		},
		{
			name:    "a subdirectory inside a layer is flagged",
			layout:  goldenpath.LayoutRust,
			rootDir: "testdata/rust-layer-not-flat",
			rule:    "rust-layer-not-flat",
		},
		{
			name:    "a layer carrying no generated mod rs is flagged",
			layout:  goldenpath.LayoutRust,
			rootDir: "testdata/rust-layer-without-generated-mod",
			rule:    "rust-layer-mod-rs",
		},
		{
			name:    "a file inside a layer that is neither rust nor a manifest is flagged",
			layout:  goldenpath.LayoutRust,
			rootDir: "testdata/rust-layer-stray-file",
			rule:    "rust-layer-stray-file",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			findings := check(t, test.layout, test.rootDir)
			if !hasRule(findings, test.rule) {
				t.Fatalf("expected a %s finding, got %+v", test.rule, findings)
			}
		})
	}
}

func TestTheIOUseRuleNamesTheFileTheLineAndTheCrate(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		message string
	}{
		{
			name:    "a user controller file naming tokio is flagged on its use line",
			path:    "src/controller/greeting_controller.rs",
			message: `line 1 uses "tokio" which no controller, port or types file may name`,
		},
		{
			name:    "a generated types file naming std net is flagged too",
			path:    "src/types/zz_generated_peer.rs",
			message: `line 3 uses "std::net" which no controller, port or types file may name`,
		},
		{
			name:    "a generated port file of a cell naming tonic is flagged too",
			path:    "src/grpc/port/zz_generated_widget_client.rs",
			message: `line 3 uses "tonic" which no controller, port or types file may name`,
		},
	}

	findings := check(t, goldenpath.LayoutRust, "testdata/rust-io-use")

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			finding, ok := findingFor(findings, "rust-io-use", test.path)
			if !ok {
				t.Fatalf("expected a rust-io-use finding for %s, got %+v", test.path, findings)
			}
			if finding.Message != test.message {
				t.Fatalf("expected message %q, got %q", test.message, finding.Message)
			}
		})
	}
}

func hitsLine(findings []goldenpath.Finding, path string, line int, name string) bool {
	want := fmt.Sprintf("line %d uses %q which no controller, port or types file may name", line, name)

	for _, f := range findings {
		if f.Rule == "rust-io-use" && f.Path == path && f.Message == want {
			return true
		}
	}

	return false
}

func hitsAnythingOnLine(findings []goldenpath.Finding, path string, line int) bool {
	prefix := fmt.Sprintf("line %d uses ", line)

	for _, f := range findings {
		if f.Rule == "rust-io-use" && f.Path == path && strings.HasPrefix(f.Message, prefix) {
			return true
		}
	}

	return false
}

func TestEveryNameInsideAGroupedUseLineIsMatched(t *testing.T) {
	findings := check(t, goldenpath.LayoutRust, "testdata/rust-io-use")
	path := "src/controller/greeting_controller.rs"

	for _, name := range []string{"std::fs", "std::net"} {
		t.Run("a grouped use line naming "+name+" is banned", func(t *testing.T) {
			if !hitsLine(findings, path, 7, name) {
				t.Fatalf("expected line 7 to be flagged for %s, got %+v", name, findings)
			}
		})
	}
}

func TestAGroupedUseSpreadOverManyLinesIsMatchedAndReportedOnItsFirstLine(t *testing.T) {
	findings := check(t, goldenpath.LayoutRust, "testdata/rust-io-use")
	path := "src/controller/greeting_controller.rs"

	if !hitsLine(findings, path, 19, "std::process") {
		t.Fatalf("expected the rustfmt multi line use to be flagged on line 19, got %+v", findings)
	}
}

func TestAPathThatOnlyStartsWithABannedPathIsNeverFlagged(t *testing.T) {
	findings := check(t, goldenpath.LayoutRust, "testdata/rust-io-use")

	if hitsAnythingOnLine(findings, "src/controller/greeting_controller.rs", 9) {
		t.Fatalf("std::netlink is not std::net and must not be flagged, got %+v", findings)
	}
}

func TestAUseLineInsideACfgTestBlockOfAControllerIsBannedLikeAnyOther(t *testing.T) {
	findings := check(t, goldenpath.LayoutRust, "testdata/rust-io-use")

	if !hitsLine(findings, "src/controller/greeting_controller.rs", 13, "tokio") {
		t.Fatalf("expected line 13 to be flagged for tokio, got %+v", findings)
	}
}

func TestAnAttributeLineNamingABannedCrateIsFlagged(t *testing.T) {
	findings := check(t, goldenpath.LayoutRust, "testdata/rust-io-use")

	if !hitsLine(findings, "src/controller/greeting_controller.rs", 15, "tokio") {
		t.Fatalf("expected line 15 to be flagged for tokio, got %+v", findings)
	}
}

func TestAHandWrittenBinaryUnderSrcBinIsNeverFlagged(t *testing.T) {
	findings := check(t, goldenpath.LayoutRust, "testdata/rust-clean")

	if _, ok := findingFor(findings, "rust-file-not-mounted", "src/bin/hello_cli.rs"); ok {
		t.Fatalf("cargo discovers src/bin by name, a binary is never mounted, got %+v", findings)
	}
}

func TestALayerHoldsNoSubdirectoryAtTheRootAndInsideACell(t *testing.T) {
	findings := check(t, goldenpath.LayoutRust, "testdata/rust-layer-not-flat")

	tests := []struct {
		name    string
		path    string
		message string
	}{
		{
			name:    "a subdirectory of a root layer names the directory",
			path:    "src/controller/greeting",
			message: `the layer src/controller holds the subdirectory "greeting", a layer directory is flat`,
		},
		{
			name:    "a subdirectory of a cell layer names the directory",
			path:    "src/rest/driver/http",
			message: `the layer src/rest/driver holds the subdirectory "http", a layer directory is flat`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			finding, ok := findingFor(findings, "rust-layer-not-flat", test.path)
			if !ok {
				t.Fatalf("expected a rust-layer-not-flat finding for %s, got %+v", test.path, findings)
			}
			if finding.Message != test.message {
				t.Fatalf("expected message %q, got %q", test.message, finding.Message)
			}
		})
	}
}

func TestAStrayFileInALayerNamesTheFileAndTheLayer(t *testing.T) {
	findings := check(t, goldenpath.LayoutRust, "testdata/rust-layer-stray-file")

	tests := []struct {
		name    string
		path    string
		message string
	}{
		{
			name:    "a stray file under a root layer is named",
			path:    "src/controller/notes.txt",
			message: `the layer src/controller holds the file "notes.txt" which is neither rust nor a known manifest`,
		},
		{
			name:    "a stray file under src bin is named too",
			path:    "src/bin/notes.txt",
			message: `the layer src/bin holds the file "notes.txt" which is neither rust nor a known manifest`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			finding, ok := findingFor(findings, "rust-layer-stray-file", test.path)
			if !ok {
				t.Fatalf("expected a rust-layer-stray-file finding for %s, got %+v", test.path, findings)
			}
			if finding.Message != test.message {
				t.Fatalf("expected message %q, got %q", test.message, finding.Message)
			}
		})
	}
}

func TestABinaryDirectoryUnderSrcBinIsNeverFlaggedAsNotFlat(t *testing.T) {
	findings := check(t, goldenpath.LayoutRust, "testdata/rust-clean")

	if hasRule(findings, "rust-layer-not-flat") {
		t.Fatalf("cargo discovers src/bin/<name>/main.rs, a binary directory is legal, got %+v", findings)
	}
}

func TestALayerWithNoGeneratedModFileIsFlaggedOnceAndNeverPerFile(t *testing.T) {
	findings := check(t, goldenpath.LayoutRust, "testdata/rust-layer-without-generated-mod")

	finding, ok := findingFor(findings, "rust-layer-mod-rs", "src/controller/mod.rs")
	if !ok {
		t.Fatalf("expected a rust-layer-mod-rs finding for the layer, got %+v", findings)
	}

	want := "the layer src/controller carries no generated mod.rs"
	if finding.Message != want {
		t.Fatalf("expected message %q, got %q", want, finding.Message)
	}

	if hasRule(findings, "rust-file-not-mounted") {
		t.Fatalf("no user file is flagged when the layer carries no generated mod.rs, got %+v", findings)
	}
}

func TestTheMountRuleNamesTheFileAndTheModFileThatShouldReachIt(t *testing.T) {
	findings := check(t, goldenpath.LayoutRust, "testdata/rust-file-not-mounted")

	finding, ok := findingFor(findings, "rust-file-not-mounted", "src/controller/greeting_controller.rs")
	if !ok {
		t.Fatalf("expected a rust-file-not-mounted finding for the user file, got %+v", findings)
	}

	want := "no mod line in src/controller/mod.rs reaches this file"
	if finding.Message != want {
		t.Fatalf("expected message %q, got %q", want, finding.Message)
	}
}

func TestAUserFileAGeneratedModFileMountsIsNeverFlagged(t *testing.T) {
	findings := check(t, goldenpath.LayoutRust, "testdata/rust-clean")

	if hasRule(findings, "rust-file-not-mounted") {
		t.Fatalf("a mounted user file must not be flagged, got %+v", findings)
	}
}

func TestAnIOCrateInAnAdapterOrADriverIsNeverFlagged(t *testing.T) {
	findings := check(t, goldenpath.LayoutRust, "testdata/rust-clean")

	if hasRule(findings, "rust-io-use") {
		t.Fatalf("an adapter and a driver may name an io crate, got %+v", findings)
	}
}

func TestTheRestCellIsACellLikeGrpcAndUdp(t *testing.T) {
	findings := check(t, goldenpath.LayoutRust, "testdata/rust-clean")

	if hasRule(findings, "rust-src-layout") || hasRule(findings, "rust-cell-layout") {
		t.Fatalf("src/rest is a cell and must not be flagged, got %+v", findings)
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

	for _, layout := range []string{goldenpath.LayoutGo, goldenpath.LayoutRust, goldenpath.LayoutRustCoreApp} {
		if !strings.Contains(err.Error(), layout) {
			t.Fatalf("expected the error to name the %s layout, got %q", layout, err.Error())
		}
	}
}
