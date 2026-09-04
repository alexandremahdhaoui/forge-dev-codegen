package forgelint

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"sigs.k8s.io/yaml"
)

type Finding struct {
	Rule    string
	Path    string
	Message string
}

type Options struct {
	RootDir string
}

type entry struct {
	Name   string                 `json:"name"`
	Engine string                 `json:"engine"`
	Runner string                 `json:"runner"`
	Spec   map[string]interface{} `json:"spec"`
}

type forgeConfig struct {
	Name              string                 `json:"name"`
	ArtifactStorePath string                 `json:"artifactStorePath"`
	Build             []entry                `json:"build"`
	Test              []entry                `json:"test"`
	Factory           map[string]interface{} `json:"factory"`
}

func Check(opts Options) ([]Finding, error) {
	rootDir := opts.RootDir
	if rootDir == "" {
		rootDir = "."
	}

	forgeYAMLPath := filepath.Join(rootDir, "forge.yaml")
	raw, err := os.ReadFile(forgeYAMLPath)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", forgeYAMLPath, err)
	}

	var cfg forgeConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %q: %w", forgeYAMLPath, err)
	}

	var findings []Finding

	findings = append(findings, checkEngineURIs(cfg.Build, "build")...)
	findings = append(findings, checkEngineURIs(cfg.Test, "test")...)

	findings = append(findings, checkHackScripts(rootDir, cfg.Build, "build")...)
	findings = append(findings, checkHackScripts(rootDir, cfg.Test, "test")...)

	findings = append(findings, checkUnitStageHasRunner(cfg.Test)...)

	if strings.TrimSpace(cfg.ArtifactStorePath) == "" {
		findings = append(findings, Finding{
			Rule:    "forge-lint-missing-artifact-store-path",
			Path:    "forge.yaml",
			Message: "missing required field artifactStorePath",
		})
	}

	if isRustRepo(rootDir, cfg) {
		findings = append(findings, checkRustUsesGoBuild(cfg.Build)...)

		adapterFindings, err := checkAdapterIsOutOfReach(rootDir)
		if err != nil {
			return nil, err
		}
		findings = append(findings, adapterFindings...)
	}

	return findings, nil
}

var adapterReachLayers = []string{"controller", "driver"}

var adapterReachRe = regexp.MustCompile(`\b(?:crate|super)(?:::[A-Za-z0-9_]+)?::adapter\b`)

func checkAdapterIsOutOfReach(rootDir string) ([]Finding, error) {
	var findings []Finding

	for _, dir := range adapterReachDirs(rootDir) {
		dirFindings, err := adapterReachFindings(rootDir, dir)
		if err != nil {
			return nil, err
		}
		findings = append(findings, dirFindings...)
	}

	return findings, nil
}

func adapterReachDirs(rootDir string) []string {
	var dirs []string

	for _, layer := range adapterReachLayers {
		dirs = append(dirs, filepath.Join("src", layer))
	}

	for _, cell := range cellNames(rootDir) {
		for _, layer := range adapterReachLayers {
			dirs = append(dirs, filepath.Join("src", cell, layer))
		}
	}

	return dirs
}

func cellNames(rootDir string) []string {
	entries, err := os.ReadDir(filepath.Join(rootDir, "src"))
	if err != nil {
		return nil
	}

	var cells []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(rootDir, "src", e.Name(), "forge-dev.yaml")); err == nil {
			cells = append(cells, e.Name())
		}
	}

	return cells
}

func adapterReachFindings(rootDir, dir string) ([]Finding, error) {
	var findings []Finding

	fullDir := filepath.Join(rootDir, dir)
	info, err := os.Stat(fullDir)
	if err != nil || !info.IsDir() {
		return findings, nil
	}

	err = filepath.Walk(fullDir, func(path string, info os.FileInfo, err error) error {
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

		for index, line := range strings.Split(string(content), "\n") {
			match := adapterReachRe.FindString(line)
			if match == "" {
				continue
			}

			findings = append(findings, Finding{
				Rule:    "forge-lint-adapter-out-of-reach",
				Path:    filepath.ToSlash(rel),
				Message: fmt.Sprintf("line %d names %q which no controller or driver file may reach", index+1, match),
			})
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %q: %w", fullDir, err)
	}

	return findings, nil
}

func checkEngineURIs(entries []entry, section string) []Finding {
	var findings []Finding
	for _, e := range entries {
		uri := e.Engine
		if uri == "" {
			uri = e.Runner
		}
		if uri == "" {
			continue
		}
		if strings.HasPrefix(uri, "go://") {
			findings = append(findings, Finding{
				Rule:    "forge-lint-go-uri",
				Path:    fmt.Sprintf("%s[%s]", section, e.Name),
				Message: fmt.Sprintf("engine URI %q uses the stale go:// scheme", uri),
			})
			continue
		}
		if !strings.HasPrefix(uri, "forge://") && !strings.HasPrefix(uri, "alias://") {
			findings = append(findings, Finding{
				Rule:    "forge-lint-unknown-scheme",
				Path:    fmt.Sprintf("%s[%s]", section, e.Name),
				Message: fmt.Sprintf("engine URI %q must start with forge:// or alias://", uri),
			})
		}
	}
	return findings
}

func checkHackScripts(rootDir string, entries []entry, section string) []Finding {
	var findings []Finding
	for _, e := range entries {
		command, _ := e.Spec["command"].(string)
		if command != "sh" && command != "bash" {
			continue
		}

		rawArgs, ok := e.Spec["args"]
		if !ok {
			continue
		}
		args, ok := rawArgs.([]interface{})
		if !ok {
			continue
		}

		for _, rawArg := range args {
			arg, ok := rawArg.(string)
			if !ok {
				continue
			}
			for _, token := range strings.Fields(arg) {
				if !strings.Contains(token, "hack/") {
					continue
				}

				var scriptPath string
				if strings.HasPrefix(token, "../") {
					scriptPath = token
				} else {
					idx := strings.Index(token, "hack/")
					scriptPath = token[idx:]
				}

				fullPath := filepath.Join(rootDir, scriptPath)
				if _, err := os.Stat(fullPath); os.IsNotExist(err) {
					findings = append(findings, Finding{
						Rule:    "forge-lint-missing-hack-script",
						Path:    fmt.Sprintf("%s[%s]", section, e.Name),
						Message: fmt.Sprintf("references %q which does not exist", scriptPath),
					})
				}
			}
		}
	}
	return findings
}

func checkUnitStageHasRunner(testEntries []entry) []Finding {
	var findings []Finding
	for _, e := range testEntries {
		if e.Name == "unit" && e.Runner == "" {
			findings = append(findings, Finding{
				Rule:    "forge-lint-unit-stage-missing-runner",
				Path:    "test[unit]",
				Message: "test stage named unit has no runner",
			})
		}
	}
	return findings
}

func checkRustUsesGoBuild(buildEntries []entry) []Finding {
	var findings []Finding
	for _, e := range buildEntries {
		if e.Engine == "forge://go-build" {
			findings = append(findings, Finding{
				Rule:    "forge-lint-rust-uses-go-build",
				Path:    fmt.Sprintf("build[%s]", e.Name),
				Message: "a Rust repo must never use forge://go-build; use forge://generic-builder",
			})
		}
	}
	return findings
}

func isRustRepo(rootDir string, cfg forgeConfig) bool {
	if _, ok := cfg.Factory["crate"]; ok {
		return true
	}
	if _, err := os.Stat(filepath.Join(rootDir, "Cargo.toml")); err == nil {
		return true
	}
	return false
}
