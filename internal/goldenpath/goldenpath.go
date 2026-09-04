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
	LayoutGo   = "go"
	LayoutRust = "rust"
)

const LayoutRustCoreApp = "rust-core-app"

func Check(opts Options) ([]Finding, error) {
	rootDir := opts.RootDir
	if rootDir == "" {
		rootDir = "."
	}

	switch opts.Layout {
	case LayoutGo:
		return checkGo(rootDir)
	case LayoutRust, LayoutRustCoreApp:
		return checkRust(rootDir)
	default:
		return nil, fmt.Errorf(
			"unknown layout %q: must be %q, %q or %q",
			opts.Layout,
			LayoutGo,
			LayoutRust,
			LayoutRustCoreApp,
		)
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

const cellMarker = "forge-dev.yaml"

const cellManifestFileName = "zz_generated_cell.yaml"

var rootLayers = []string{"adapter", "bin", "config", "controller", "driver", "port", "types"}

var cellLayers = []string{"adapter", "controller", "driver", "port", "types"}

var pureLayers = []string{"controller", "port", "types"}

const cargoDiscoveredLayer = "bin"

var mountedRootLayers = namesWithout(rootLayers, cargoDiscoveredLayer)

func namesWithout(names []string, dropped string) []string {
	var kept []string

	for _, name := range names {
		if name == dropped {
			continue
		}
		kept = append(kept, name)
	}

	return kept
}

func checkRust(rootDir string) ([]Finding, error) {
	var findings []Finding

	if !fileExists(filepath.Join(rootDir, "Cargo.toml")) {
		return append(findings, Finding{Rule: "rust-cargo-toml", Path: "Cargo.toml", Message: "missing Cargo.toml"}), nil
	}

	if !fileExists(filepath.Join(rootDir, "src/lib.rs")) {
		findings = append(findings, Finding{Rule: "rust-lib-rs", Path: "src/lib.rs", Message: "missing src/lib.rs"})
	}

	if !fileExists(filepath.Join(rootDir, "forge.yaml")) {
		findings = append(findings, Finding{Rule: "rust-forge-yaml", Path: "forge.yaml", Message: "missing forge.yaml"})
	}

	hasGenerated, err := treeHasZzGenerated(filepath.Join(rootDir, "src"))
	if err != nil {
		return nil, err
	}
	if hasGenerated && !fileExists(filepath.Join(rootDir, cellMarker)) {
		findings = append(findings, Finding{
			Rule:    "rust-forge-dev-yaml",
			Path:    cellMarker,
			Message: "missing forge-dev.yaml although zz_generated files exist under src",
		})
	}

	cells, err := cellNames(rootDir)
	if err != nil {
		return nil, err
	}

	for _, check := range []func(string, []string) ([]Finding, error){
		checkRootLayout,
		checkCellLayout,
		checkCellManifests,
		checkLayersAreFlat,
		checkPureLayers,
		checkEveryFileIsMounted,
	} {
		checkFindings, err := check(rootDir, cells)
		if err != nil {
			return nil, err
		}
		findings = append(findings, checkFindings...)
	}

	pathFindings, err := checkPathAttribute(rootDir)
	if err != nil {
		return nil, err
	}

	return append(findings, pathFindings...), nil
}

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

func checkRootLayout(rootDir string, cells []string) ([]Finding, error) {
	var findings []Finding

	srcDir := filepath.Join(rootDir, "src")
	if !dirExists(srcDir) {
		return findings, nil
	}

	allowed := namesToSet(rootLayers)
	for _, cell := range cells {
		allowed[cell] = true
	}

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", srcDir, err)
	}

	for _, e := range entries {
		if !e.IsDir() || allowed[e.Name()] {
			continue
		}

		holdsRust, err := treeHasRust(filepath.Join(srcDir, e.Name()))
		if err != nil {
			return nil, err
		}
		if !holdsRust {
			continue
		}

		findings = append(findings, Finding{
			Rule:    "rust-src-layout",
			Path:    filepath.ToSlash(filepath.Join("src", e.Name())),
			Message: fmt.Sprintf("a directory under src holds rust only as one of the layers %s or as a cell holding a forge-dev.yaml", strings.Join(rootLayers, ", ")),
		})
	}

	return findings, nil
}

func checkCellLayout(rootDir string, cells []string) ([]Finding, error) {
	var findings []Finding

	allowed := namesToSet(cellLayers)

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
				Path:    filepath.ToSlash(filepath.Join("src", cell, e.Name())),
				Message: fmt.Sprintf("a cell holds rust only under the layers %s", strings.Join(cellLayers, ", ")),
			})
		}
	}

	return findings, nil
}

func checkCellManifests(rootDir string, cells []string) ([]Finding, error) {
	var findings []Finding

	for _, cell := range cells {
		cellDir := filepath.Join(rootDir, "src", cell)

		hasGenerated, err := treeHasZzGenerated(cellDir)
		if err != nil {
			return nil, err
		}
		if !hasGenerated || fileExists(filepath.Join(cellDir, cellManifestFileName)) {
			continue
		}

		findings = append(findings, Finding{
			Rule:    "rust-cell-manifest",
			Path:    filepath.ToSlash(filepath.Join("src", cell, cellManifestFileName)),
			Message: "missing zz_generated_cell.yaml although the cell holds zz_generated files",
		})
	}

	return findings, nil
}

var layerManifestFileNames = []string{cellMarker, cellManifestFileName, "zz_generated.runnable.yaml"}

func checkLayersAreFlat(rootDir string, cells []string) ([]Finding, error) {
	var findings []Finding

	for _, dir := range flatLayerDirs(cells) {
		entries, err := layerEntries(rootDir, dir)
		if err != nil {
			return nil, err
		}

		for _, e := range entries {
			if !e.IsDir() {
				continue
			}

			findings = append(findings, Finding{
				Rule: "rust-layer-not-flat",
				Path: filepath.ToSlash(filepath.Join(dir, e.Name())),
				Message: fmt.Sprintf(
					"the layer %s holds the subdirectory %q, a layer directory is flat",
					filepath.ToSlash(dir),
					e.Name(),
				),
			})
		}
	}

	for _, dir := range everyLayerDir(cells) {
		entries, err := layerEntries(rootDir, dir)
		if err != nil {
			return nil, err
		}

		for _, e := range entries {
			if e.IsDir() || isLayerFileName(e.Name()) {
				continue
			}

			findings = append(findings, Finding{
				Rule: "rust-layer-stray-file",
				Path: filepath.ToSlash(filepath.Join(dir, e.Name())),
				Message: fmt.Sprintf(
					"the layer %s holds the file %q which is neither rust nor a known manifest",
					filepath.ToSlash(dir),
					e.Name(),
				),
			})
		}
	}

	return findings, nil
}

func layerEntries(rootDir, dir string) ([]os.DirEntry, error) {
	fullDir := filepath.Join(rootDir, dir)
	if !dirExists(fullDir) {
		return nil, nil
	}

	entries, err := os.ReadDir(fullDir)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", fullDir, err)
	}

	return entries, nil
}

func isLayerFileName(name string) bool {
	if strings.HasSuffix(name, ".rs") {
		return true
	}

	for _, manifest := range layerManifestFileNames {
		if name == manifest {
			return true
		}
	}

	return false
}

var bannedCrates = []string{"axum", "hyper", "reqwest", "rusqlite", "tokio", "tonic", "tower"}

var bannedPaths = []string{"std::fs", "std::net", "std::process", "std::time::Instant"}

var bannedNames = append(append([]string{}, bannedCrates...), bannedPaths...)

var bannedNamePatterns = compileNamePatterns(bannedNames)

func compileNamePatterns(names []string) map[string]*regexp.Regexp {
	patterns := make(map[string]*regexp.Regexp, len(names))
	for _, name := range names {
		patterns[name] = regexp.MustCompile(`(^|[^A-Za-z0-9_:])` + regexp.QuoteMeta(name) + `(::|[;,}\s]|$)`)
	}

	return patterns
}

func checkPureLayers(rootDir string, cells []string) ([]Finding, error) {
	var findings []Finding

	for _, dir := range pureLayerDirs(cells) {
		dirFindings, err := bannedUseFindings(rootDir, dir)
		if err != nil {
			return nil, err
		}
		findings = append(findings, dirFindings...)
	}

	return findings, nil
}

func pureLayerDirs(cells []string) []string {
	var dirs []string

	for _, layer := range pureLayers {
		dirs = append(dirs, filepath.Join("src", layer))
	}

	for _, cell := range cells {
		for _, layer := range pureLayers {
			dirs = append(dirs, filepath.Join("src", cell, layer))
		}
	}

	return dirs
}

func bannedUseFindings(rootDir, dir string) ([]Finding, error) {
	var findings []Finding

	fullDir := filepath.Join(rootDir, dir)
	if !dirExists(fullDir) {
		return findings, nil
	}

	err := filepath.Walk(fullDir, func(path string, info os.FileInfo, err error) error {
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

		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}

		for _, hit := range bannedUseHits(string(content)) {
			findings = append(findings, Finding{
				Rule:    "rust-io-use",
				Path:    filepath.ToSlash(rel),
				Message: fmt.Sprintf("line %d uses %q which no controller, port or types file may name", hit.line, hit.name),
			})
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %q: %w", fullDir, err)
	}

	return findings, nil
}

type useHit struct {
	line int
	name string
}

func bannedUseHits(content string) []useHit {
	var hits []useHit

	lines := strings.Split(content, "\n")

	for index := 0; index < len(lines); index++ {
		if isUseLine(lines[index]) {
			statement, last := joinUntilSemicolon(lines, index)
			hits = append(hits, bannedNameHits(statement, index+1)...)
			index = last

			continue
		}

		if isAttributeLine(lines[index]) {
			hits = append(hits, bannedNameHits(lines[index], index+1)...)
		}
	}

	return hits
}

func joinUntilSemicolon(lines []string, start int) (string, int) {
	var joined strings.Builder

	for index := start; index < len(lines); index++ {
		joined.WriteString(" ")
		joined.WriteString(strings.TrimSpace(lines[index]))

		if strings.Contains(lines[index], ";") {
			return joined.String(), index
		}
	}

	return joined.String(), len(lines) - 1
}

func bannedNameHits(statement string, line int) []useHit {
	var hits []useHit

	flattened := flattenGroupedPaths(statement)

	for _, name := range bannedNames {
		if bannedNamePatterns[name].MatchString(flattened) {
			hits = append(hits, useHit{line: line, name: name})
		}
	}

	return hits
}

var groupedPathRe = regexp.MustCompile(`([A-Za-z0-9_]+(?:::[A-Za-z0-9_]+)*::)\{([^}]*)\}`)

func flattenGroupedPaths(line string) string {
	flattened := line

	for _, group := range groupedPathRe.FindAllStringSubmatch(line, -1) {
		for _, name := range strings.Split(group[2], ",") {
			trimmed := strings.TrimSpace(name)
			if trimmed == "" {
				continue
			}
			flattened += " " + group[1] + trimmed + " "
		}
	}

	return flattened
}

func isUseLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "pub ")
	trimmed = strings.TrimPrefix(trimmed, "pub(crate) ")

	return strings.HasPrefix(trimmed, "use ")
}

func isAttributeLine(line string) bool {
	trimmed := strings.TrimSpace(line)

	return strings.HasPrefix(trimmed, "#[") || strings.HasPrefix(trimmed, "#![")
}

func checkEveryFileIsMounted(rootDir string, cells []string) ([]Finding, error) {
	var findings []Finding

	for _, dir := range layerDirs(cells) {
		dirFindings, err := unmountedFindings(rootDir, dir)
		if err != nil {
			return nil, err
		}
		findings = append(findings, dirFindings...)
	}

	return findings, nil
}

func layerDirs(cells []string) []string {
	return dirsForLayers(mountedRootLayers, cells)
}

func flatLayerDirs(cells []string) []string {
	return dirsForLayers(mountedRootLayers, cells)
}

func everyLayerDir(cells []string) []string {
	return dirsForLayers(rootLayers, cells)
}

func dirsForLayers(layers []string, cells []string) []string {
	var dirs []string

	for _, layer := range layers {
		dirs = append(dirs, filepath.Join("src", layer))
	}

	for _, cell := range cells {
		for _, layer := range cellLayers {
			dirs = append(dirs, filepath.Join("src", cell, layer))
		}
	}

	return dirs
}

var modLineRe = regexp.MustCompile(`(?m)^\s*(?:pub(?:\([^)]*\))?\s+)?mod\s+([A-Za-z0-9_]+)\s*;`)

func unmountedFindings(rootDir, dir string) ([]Finding, error) {
	var findings []Finding

	fullDir := filepath.Join(rootDir, dir)
	if !dirExists(fullDir) {
		return findings, nil
	}

	entries, err := os.ReadDir(fullDir)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", fullDir, err)
	}

	mounted := map[string]bool{}
	modFileRel := filepath.ToSlash(filepath.Join(dir, "mod.rs"))
	hasGeneratedModFile := false

	for _, e := range entries {
		if e.IsDir() || e.Name() != "mod.rs" {
			continue
		}

		modPath := filepath.Join(fullDir, e.Name())

		generated, err := isGenerated(modPath)
		if err != nil {
			return nil, err
		}
		if !generated {
			continue
		}

		hasGeneratedModFile = true

		content, err := os.ReadFile(modPath)
		if err != nil {
			return nil, fmt.Errorf("reading %q: %w", modPath, err)
		}

		for _, m := range modLineRe.FindAllStringSubmatch(string(content), -1) {
			mounted[m[1]] = true
		}
	}

	if !hasGeneratedModFile {
		userFiles, err := holdsUserRustFile(fullDir, entries)
		if err != nil {
			return nil, err
		}
		if !userFiles {
			return findings, nil
		}

		return append(findings, Finding{
			Rule:    "rust-layer-mod-rs",
			Path:    modFileRel,
			Message: fmt.Sprintf("the layer %s carries no generated mod.rs", filepath.ToSlash(dir)),
		}), nil
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".rs") || e.Name() == "mod.rs" {
			continue
		}

		generated, err := isGenerated(filepath.Join(fullDir, e.Name()))
		if err != nil {
			return nil, err
		}
		if generated || mounted[strings.TrimSuffix(e.Name(), ".rs")] {
			continue
		}

		findings = append(findings, Finding{
			Rule:    "rust-file-not-mounted",
			Path:    filepath.ToSlash(filepath.Join(dir, e.Name())),
			Message: fmt.Sprintf("no mod line in %s reaches this file", modFileRel),
		})
	}

	return findings, nil
}

func holdsUserRustFile(fullDir string, entries []os.DirEntry) (bool, error) {
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".rs") || e.Name() == "mod.rs" {
			continue
		}

		generated, err := isGenerated(filepath.Join(fullDir, e.Name()))
		if err != nil {
			return false, err
		}
		if !generated {
			return true, nil
		}
	}

	return false, nil
}

const generatedMarker = "Code generated by"

func isGenerated(path string) (bool, error) {
	if strings.HasPrefix(filepath.Base(path), "zz_generated") {
		return true, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("reading %q: %w", path, err)
	}

	firstLine, _, _ := strings.Cut(string(content), "\n")

	return strings.Contains(firstLine, generatedMarker), nil
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

func namesToSet(names []string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}

	return set
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
