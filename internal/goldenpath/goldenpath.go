package goldenpath

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Finding struct {
	Rule    string
	Path    string
	Message string
}

type Options struct {
	Layout  string
	RootDir string
}

const (
	LayoutGo          = "go"
	LayoutRustCoreApp = "rust-core-app"
)

func Check(opts Options) ([]Finding, error) {
	rootDir := opts.RootDir
	if rootDir == "" {
		rootDir = "."
	}

	switch opts.Layout {
	case LayoutGo:
		return checkGo(rootDir)
	case LayoutRustCoreApp:
		return checkRustCoreApp(rootDir)
	default:
		return nil, fmt.Errorf("unknown layout %q: must be %q or %q", opts.Layout, LayoutGo, LayoutRustCoreApp)
	}
}

func checkGo(rootDir string) ([]Finding, error) {
	var findings []Finding

	for _, dir := range []string{"internal/adapter", "internal/controller", "internal/driver"} {
		if !dirExists(filepath.Join(rootDir, dir)) {
			findings = append(findings, Finding{
				Rule:    "go-layout",
				Path:    dir,
				Message: fmt.Sprintf("missing required directory %q", dir),
			})
		}
	}

	driverImportsAdapter, err := filesImporting(filepath.Join(rootDir, "internal/driver"), []string{"internal/adapter"})
	if err != nil {
		return nil, err
	}
	for _, f := range driverImportsAdapter {
		findings = append(findings, Finding{
			Rule:    "go-driver-imports-adapter",
			Path:    f,
			Message: "a driver file imports an adapter package",
		})
	}

	adapterImportsAdapter, err := adapterImportsAnotherAdapter(filepath.Join(rootDir, "internal/adapter"))
	if err != nil {
		return nil, err
	}
	findings = append(findings, adapterImportsAdapter...)

	return findings, nil
}

func adapterImportsAnotherAdapter(adapterDir string) ([]Finding, error) {
	var findings []Finding

	if !dirExists(adapterDir) {
		return findings, nil
	}

	entries, err := os.ReadDir(adapterDir)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", adapterDir, err)
	}

	var subPackages []string
	for _, e := range entries {
		if e.IsDir() {
			subPackages = append(subPackages, e.Name())
		}
	}

	err = filepath.Walk(adapterDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %q: %w", path, err)
		}

		ownPackage := filepath.Base(filepath.Dir(path))
		for _, pkg := range subPackages {
			if pkg == ownPackage {
				continue
			}
			marker := "internal/adapter/" + pkg
			if strings.Contains(string(content), marker) {
				findings = append(findings, Finding{
					Rule:    "go-adapter-imports-adapter",
					Path:    path,
					Message: fmt.Sprintf("imports another adapter package %q", pkg),
				})
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %q: %w", adapterDir, err)
	}

	return findings, nil
}

func checkRustCoreApp(rootDir string) ([]Finding, error) {
	var findings []Finding

	if !fileExists(filepath.Join(rootDir, "Cargo.toml")) {
		findings = append(findings, Finding{Rule: "rust-cargo-toml", Path: "Cargo.toml", Message: "missing Cargo.toml"})
		return findings, nil
	}

	if !fileExists(filepath.Join(rootDir, "src/lib.rs")) {
		findings = append(findings, Finding{Rule: "rust-lib-rs", Path: "src/lib.rs", Message: "missing src/lib.rs"})
	}

	crateName, err := cargoCrateName(filepath.Join(rootDir, "Cargo.toml"))
	if err != nil {
		return nil, err
	}

	switch {
	case strings.HasSuffix(crateName, "-core"):
		findings = append(findings, checkRustCore(rootDir)...)
	case strings.HasSuffix(crateName, "-app"):
		findings = append(findings, checkRustApp(rootDir)...)
	default:
		findings = append(findings, Finding{
			Rule:    "rust-crate-name",
			Path:    "Cargo.toml",
			Message: fmt.Sprintf("crate name %q must end in -core or -app", crateName),
		})
	}

	if !fileExists(filepath.Join(rootDir, "forge.yaml")) {
		findings = append(findings, Finding{Rule: "rust-forge-yaml", Path: "forge.yaml", Message: "missing forge.yaml"})
	}

	hasGenerated, err := treeHasZzGenerated(filepath.Join(rootDir, "src"))
	if err != nil {
		return nil, err
	}
	if hasGenerated && !fileExists(filepath.Join(rootDir, "forge-dev.yaml")) {
		findings = append(findings, Finding{
			Rule:    "rust-forge-dev-yaml",
			Path:    "forge-dev.yaml",
			Message: "missing forge-dev.yaml although zz_generated files exist under src",
		})
	}

	cells, err := cellNames(rootDir)
	if err != nil {
		return nil, err
	}

	cellFindings, err := checkCells(rootDir, cells, crateName)
	if err != nil {
		return nil, err
	}
	findings = append(findings, cellFindings...)

	handFindings, err := checkHandWrittenFiles(rootDir, cells)
	if err != nil {
		return nil, err
	}
	findings = append(findings, handFindings...)

	pathFindings, err := checkPathAttribute(rootDir)
	if err != nil {
		return nil, err
	}
	findings = append(findings, pathFindings...)

	return findings, nil
}

func checkPathAttribute(rootDir string) ([]Finding, error) {
	var findings []Finding

	srcDir := filepath.Join(rootDir, "src")
	if !dirExists(srcDir) {
		return findings, nil
	}

	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".rs") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %q: %w", path, err)
		}

		if !strings.Contains(string(content), "#[path") {
			return nil
		}

		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}

		findings = append(findings, Finding{
			Rule:    "rust-no-path-attribute",
			Path:    filepath.ToSlash(rel),
			Message: "a module is mounted with a path attribute, write a real mod.rs beside the files instead",
		})

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %q: %w", srcDir, err)
	}

	return findings, nil
}

const cellMarker = "forge-dev.yaml"

var coreCellLayers = []string{"port", "controller", "types", "hand"}

var appCellLayers = []string{"adapter", "driver", "hand"}

func cellNames(rootDir string) ([]string, error) {
	srcDir := filepath.Join(rootDir, "src")
	if !dirExists(srcDir) {
		return nil, nil
	}

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", srcDir, err)
	}

	var cells []string
	for _, e := range entries {
		if e.IsDir() && fileExists(filepath.Join(srcDir, e.Name(), cellMarker)) {
			cells = append(cells, e.Name())
		}
	}

	return cells, nil
}

func cellLayersFor(crateName string) []string {
	if strings.HasSuffix(crateName, "-app") {
		return appCellLayers
	}

	return coreCellLayers
}

func checkCells(rootDir string, cells []string, crateName string) ([]Finding, error) {
	var findings []Finding

	allowed := map[string]bool{}
	for _, layer := range cellLayersFor(crateName) {
		allowed[layer] = true
	}

	for _, cell := range cells {
		cellDir := filepath.Join(rootDir, "src", cell)

		entries, err := os.ReadDir(cellDir)
		if err != nil {
			return nil, fmt.Errorf("reading %q: %w", cellDir, err)
		}

		for _, e := range entries {
			if !e.IsDir() || allowed[e.Name()] {
				continue
			}

			holdsRust, err := treeHasRust(filepath.Join(cellDir, e.Name()))
			if err != nil {
				return nil, err
			}

			if !holdsRust {
				continue
			}

			findings = append(findings, Finding{
				Rule:    "rust-cell-layout",
				Path:    filepath.Join("src", cell, e.Name()),
				Message: fmt.Sprintf("a cell holds rust only under %s", strings.Join(cellLayersFor(crateName), ", ")),
			})
		}
	}

	return findings, nil
}

func treeHasRust(dir string) (bool, error) {
	found := false

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".rs") {
			found = true
		}
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("walking %q: %w", dir, err)
	}

	return found, nil
}

func rootCellOwnsTheModFile(rel string) bool {
	if filepath.Base(rel) != "mod.rs" {
		return false
	}

	for _, layer := range append(append([]string{}, coreCellLayers...), appCellLayers...) {
		if rel == layer+"/mod.rs" {
			return true
		}
	}

	return false
}

func cellAllowsHandWritten(rel string, cells []string) bool {
	if filepath.Base(rel) == "mod.rs" {
		for _, cell := range cells {
			if strings.HasPrefix(rel, cell+"/") {
				return true
			}
		}
	}

	for _, cell := range cells {
		if strings.HasPrefix(rel, cell+"/hand/") {
			return true
		}
	}

	return false
}

func checkRustCore(rootDir string) []Finding {
	var findings []Finding

	for _, dir := range []string{"src/port", "src/controller", "src/types"} {
		if !dirExists(filepath.Join(rootDir, dir)) {
			findings = append(findings, Finding{
				Rule:    "rust-core-layout",
				Path:    dir,
				Message: fmt.Sprintf("missing required directory %q", dir),
			})
		}
	}

	forbidden, err := filesUsing(filepath.Join(rootDir, "src"), []string{"std::fs", "std::net", "std::process", "tokio"})
	if err != nil {
		return append(findings, Finding{Rule: "rust-core-scan", Path: "src", Message: err.Error()})
	}
	findings = append(findings, forbidden...)

	return findings
}

func checkRustApp(rootDir string) []Finding {
	var findings []Finding

	for _, dir := range []string{"src/adapter", "src/driver", "src/bin"} {
		if !dirExists(filepath.Join(rootDir, dir)) {
			findings = append(findings, Finding{
				Rule:    "rust-app-layout",
				Path:    dir,
				Message: fmt.Sprintf("missing required directory %q", dir),
			})
		}
	}

	driverDir := filepath.Join(rootDir, "src/driver")
	if dirExists(driverDir) {
		adapterUseRe := regexp.MustCompile(`use\s+crate::adapter|_app::adapter`)
		err := filepath.Walk(driverDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(path, ".rs") {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("reading %q: %w", path, err)
			}
			if adapterUseRe.MatchString(string(content)) {
				findings = append(findings, Finding{
					Rule:    "rust-driver-imports-adapter",
					Path:    path,
					Message: "a driver file imports the adapter module",
				})
			}
			return nil
		})
		if err != nil {
			findings = append(findings, Finding{Rule: "rust-app-scan", Path: driverDir, Message: err.Error()})
		}
	}

	return findings
}

func checkHandWrittenFiles(rootDir string, cells []string) ([]Finding, error) {
	var findings []Finding

	srcDir := filepath.Join(rootDir, "src")
	if !dirExists(srcDir) {
		return findings, nil
	}

	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".rs") {
			return nil
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		if rel == "lib.rs" {
			return nil
		}
		if strings.HasPrefix(rel, "bin/") {
			return nil
		}
		if strings.HasPrefix(rel, "hand/") {
			return nil
		}
		if strings.HasPrefix(filepath.Base(path), "zz_generated") {
			return nil
		}
		if rootCellOwnsTheModFile(rel) || cellAllowsHandWritten(rel, cells) {
			return nil
		}

		findings = append(findings, Finding{
			Rule:    "rust-hand-written-outside-hand",
			Path:    filepath.Join("src", rel),
			Message: "hand written Rust file must live under src/hand, be src/lib.rs, be under src/bin, or be a layer or cell mod.rs or hand file",
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %q: %w", srcDir, err)
	}

	return findings, nil
}

func filesImporting(dir string, forbiddenSubstrings []string) ([]string, error) {
	var matches []string
	if !dirExists(dir) {
		return matches, nil
	}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %q: %w", path, err)
		}
		for _, s := range forbiddenSubstrings {
			if strings.Contains(string(content), s) {
				matches = append(matches, path)
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %q: %w", dir, err)
	}

	return matches, nil
}

func filesUsing(dir string, forbidden []string) ([]Finding, error) {
	var findings []Finding
	if !dirExists(dir) {
		return findings, nil
	}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".rs") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %q: %w", path, err)
		}
		for _, s := range forbidden {
			if strings.Contains(string(content), s) {
				findings = append(findings, Finding{
					Rule:    "rust-core-forbidden-usage",
					Path:    path,
					Message: fmt.Sprintf("uses forbidden %q inside a core crate", s),
				})
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %q: %w", dir, err)
	}

	return findings, nil
}

func treeHasZzGenerated(dir string) (bool, error) {
	found := false
	if !dirExists(dir) {
		return false, nil
	}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasPrefix(filepath.Base(path), "zz_generated") {
			found = true
		}
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("walking %q: %w", dir, err)
	}

	return found, nil
}

func cargoCrateName(cargoTomlPath string) (string, error) {
	content, err := os.ReadFile(cargoTomlPath)
	if err != nil {
		return "", fmt.Errorf("reading %q: %w", cargoTomlPath, err)
	}

	nameRe := regexp.MustCompile(`(?m)^name\s*=\s*"([^"]+)"`)
	m := nameRe.FindSubmatch(content)
	if m == nil {
		return "", fmt.Errorf("no name field found in %q", cargoTomlPath)
	}
	return string(m[1]), nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
