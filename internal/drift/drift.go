package drift

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/toolresolver"
)

const (
	Rewritten = "rewritten"
	Written   = "written"
	Removed   = "removed"

	forgeName   = "forge"
	forgeModule = "github.com/alexandremahdhaoui/forge/cmd/forge"
)

type Options struct {
	RootDir string
}

type Finding struct {
	Path   string
	Change string
}

func Check(opts Options) ([]Finding, error) {
	root, err := absoluteRoot(opts.RootDir)
	if err != nil {
		return nil, err
	}

	before, err := snapshot(root)
	if err != nil {
		return nil, err
	}

	store, err := takeArtifactStore(root)
	if err != nil {
		return nil, err
	}

	buildErr := runForgeBuild(root)

	if err := store.putBack(); err != nil {
		return nil, fmt.Errorf("restoring the artifact store of %q: %w", root, err)
	}

	if buildErr != nil {
		return nil, buildErr
	}

	after, err := snapshot(root)
	if err != nil {
		return nil, err
	}

	return Compare(before, after), nil
}

func Compare(before, after map[string]string) []Finding {
	findings := make([]Finding, 0)

	for path, afterDigest := range after {
		beforeDigest, known := before[path]

		switch {
		case !known:
			findings = append(findings, Finding{Path: path, Change: Written})
		case beforeDigest != afterDigest:
			findings = append(findings, Finding{Path: path, Change: Rewritten})
		}
	}

	for path := range before {
		if _, still := after[path]; !still {
			findings = append(findings, Finding{Path: path, Change: Removed})
		}
	}

	sort.Slice(findings, func(i, j int) bool { return findings[i].Path < findings[j].Path })

	return findings
}

func absoluteRoot(rootDir string) (string, error) {
	if rootDir == "" {
		rootDir = "."
	}

	root, err := filepath.Abs(rootDir)
	if err != nil {
		return "", fmt.Errorf("resolving the root directory %q: %w", rootDir, err)
	}

	return root, nil
}

type savedStore struct {
	path    string
	content []byte
}

func takeArtifactStore(root string) (savedStore, error) {
	spec, err := forge.ReadSpecFromPath(filepath.Join(root, "forge.yaml"))
	if err != nil {
		return savedStore{}, fmt.Errorf("reading the forge.yaml of %q: %w", root, err)
	}

	path := spec.ArtifactStorePath
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}

	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return savedStore{path: path}, nil
	}

	if err != nil {
		return savedStore{}, fmt.Errorf("reading the artifact store %q: %w", path, err)
	}

	if err := os.Remove(path); err != nil {
		return savedStore{}, fmt.Errorf("moving the artifact store %q aside: %w", path, err)
	}

	return savedStore{path: path, content: content}, nil
}

func (s savedStore) putBack() error {
	if s.content == nil {
		return nil
	}

	if err := os.WriteFile(s.path, s.content, 0o600); err != nil {
		return fmt.Errorf("writing the artifact store %q back: %w", s.path, err)
	}

	return nil
}

func runForgeBuild(root string) error {
	invocation, err := toolresolver.Resolver{}.Resolve(toolresolver.Ref{Name: forgeName, Module: forgeModule})
	if err != nil {
		return fmt.Errorf("resolving the forge that rebuilds %q: %w", root, err)
	}

	cmd := exec.Command(invocation.Path, append(append([]string{}, invocation.Args...), "build")...)
	cmd.Dir = root

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("running forge build in %q with the %s forge: %w: %s",
			root, invocation.Source, err, strings.TrimSpace(string(out)))
	}

	return nil
}

func snapshot(root string) (map[string]string, error) {
	cmd := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	cmd.Dir = root

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing the files git shows in %q: %w", root, err)
	}

	files := map[string]string{}

	for _, name := range strings.Split(string(out), "\x00") {
		if name == "" {
			continue
		}

		digest, err := digestFile(filepath.Join(root, name))
		if err != nil {
			return nil, err
		}

		if digest == "" {
			continue
		}

		files[name] = digest
	}

	return files, nil
}

func digestFile(path string) (string, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return "", nil
	}

	if err != nil {
		return "", fmt.Errorf("opening %q to digest it: %w", path, err)
	}

	defer func() { _ = file.Close() }()

	sum := sha256.New()
	if _, err := io.Copy(sum, file); err != nil {
		return "", fmt.Errorf("digesting %q: %w", path, err)
	}

	return hex.EncodeToString(sum.Sum(nil)), nil
}
