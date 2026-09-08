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
	"syscall"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/toolresolver"
)

const (
	Rewritten = "rewritten"
	Written   = "written"
	Removed   = "removed"

	forgeName   = "forge"
	forgeModule = "github.com/alexandremahdhaoui/forge/cmd/forge"

	asideSuffix     = ".aside"
	storeLockSuffix = ".lock"
	gateLockSuffix  = ".drift-lock"
)

type Options struct {
	RootDir string
}

type Finding struct {
	Path   string
	Change string
}

type Forge struct {
	Argv   []string
	Source string
}

func Check(opts Options) (findings []Finding, err error) {
	root, err := absoluteRoot(opts.RootDir)
	if err != nil {
		return nil, err
	}

	storePath, err := artifactStorePath(root)
	if err != nil {
		return nil, err
	}

	rebuilder, err := ResolveForge(toolresolver.DepVersion(forgeModule))
	if err != nil {
		return nil, err
	}

	releaseGate, err := lockPath(storePath + gateLockSuffix)
	if err != nil {
		return nil, err
	}

	defer releaseGate()

	before, err := snapshot(root)
	if err != nil {
		return nil, err
	}

	store, err := takeArtifactStore(storePath)
	if err != nil {
		return nil, err
	}

	defer func() {
		if back := store.putBack(); back != nil && err == nil {
			findings, err = nil, back
		}
	}()

	if err := runForgeBuild(root, rebuilder); err != nil {
		return nil, err
	}

	after, err := snapshot(root)
	if err != nil {
		return nil, err
	}

	return Compare(before, after), nil
}

func ResolveForge(builtAgainst string) (Forge, error) {
	if builtAgainst == "" {
		return Forge{
			Argv:   []string{"go", "run", forgeModule},
			Source: toolresolver.SourceWorkspace,
		}, nil
	}

	invocation, err := toolresolver.Resolver{}.Resolve(toolresolver.Ref{Name: forgeName, Module: forgeModule})
	if err != nil {
		return Forge{}, fmt.Errorf("resolving a forge built from %s: %w", builtAgainst, err)
	}

	candidate := Forge{
		Argv:   append([]string{invocation.Path}, invocation.Args...),
		Source: invocation.Source,
	}

	reported, err := askForgeItsVersion(candidate)
	if err != nil {
		return Forge{}, err
	}

	if trimDirty(reported) != trimDirty(builtAgainst) {
		return Forge{}, fmt.Errorf(
			"refusing the %s forge at %q: it reports %s and this gate was built against forge %s",
			candidate.Source, invocation.Path, reported, builtAgainst)
	}

	return candidate, nil
}

func askForgeItsVersion(candidate Forge) (string, error) {
	spelled := strings.Join(candidate.Argv, " ")

	outsideAnyRepository, err := os.MkdirTemp("", "generated-drift-version-")
	if err != nil {
		return "", fmt.Errorf("making an empty directory to ask the %s forge %q for its version: %w",
			candidate.Source, spelled, err)
	}

	defer func() { _ = os.RemoveAll(outsideAnyRepository) }()

	cmd := exec.Command(candidate.Argv[0], append(append([]string{}, candidate.Argv[1:]...), "version")...)
	cmd.Dir = outsideAnyRepository
	cmd.Env = append(os.Environ(), "GIT_CEILING_DIRECTORIES="+filepath.Dir(outsideAnyRepository))

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("asking the %s forge %q for its version: %w: %s",
			candidate.Source, spelled, err, strings.TrimSpace(string(out)))
	}

	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[0] == forgeName && fields[1] == "version" {
			return fields[2], nil
		}
	}

	return "", fmt.Errorf("the %s forge %q names no version, it answered: %s",
		candidate.Source, spelled, strings.TrimSpace(string(out)))
}

func trimDirty(version string) string {
	return strings.TrimSuffix(strings.TrimSuffix(version, "-dirty"), "+dirty")
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
	path  string
	aside string
	taken bool
}

func artifactStorePath(root string) (string, error) {
	spec, err := forge.ReadSpecFromPath(filepath.Join(root, "forge.yaml"))
	if err != nil {
		return "", fmt.Errorf("reading the forge.yaml of %q: %w", root, err)
	}

	path := spec.ArtifactStorePath
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}

	return path, nil
}

func takeArtifactStore(path string) (savedStore, error) {
	saved := savedStore{path: path, aside: path + asideSuffix}

	switch _, err := os.Stat(saved.aside); {
	case err == nil:
		return savedStore{}, fmt.Errorf(
			"refusing to move the artifact store %q aside: %q already holds a copy an interrupted run of this gate saved. "+
				"read that copy, move it back over %q when it is the store you want to keep, delete it when it is not, "+
				"then run the gate again",
			path, saved.aside, path)
	case !os.IsNotExist(err):
		return savedStore{}, fmt.Errorf("looking for an artifact store copy at %q: %w", saved.aside, err)
	}

	unlock, err := lockPath(path + storeLockSuffix)
	if err != nil {
		return savedStore{}, err
	}

	defer unlock()

	if err := os.Rename(path, saved.aside); err != nil {
		if os.IsNotExist(err) {
			return saved, nil
		}

		return savedStore{}, fmt.Errorf("moving the artifact store %q aside to %q: %w", path, saved.aside, err)
	}

	saved.taken = true

	return saved, nil
}

func (s savedStore) putBack() error {
	unlock, err := lockPath(s.path + storeLockSuffix)
	if err != nil {
		return err
	}

	defer unlock()

	if !s.taken {
		if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing the artifact store %q this run created: %w", s.path, err)
		}

		return nil
	}

	if err := os.Rename(s.aside, s.path); err != nil {
		return fmt.Errorf("putting the artifact store %q back from %q: %w", s.path, s.aside, err)
	}

	return nil
}

func lockPath(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("creating the directory of the lock %q: %w", path, err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("opening the lock %q: %w", path, err)
	}

	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		_ = file.Close()

		return nil, fmt.Errorf("taking the lock %q: %w", path, err)
	}

	return func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
	}, nil
}

func runForgeBuild(root string, rebuilder Forge) error {
	cmd := exec.Command(rebuilder.Argv[0], append(append([]string{}, rebuilder.Argv[1:]...), "build")...)
	cmd.Dir = root

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("running forge build in %q with the %s forge %q: %w: %s",
			root, rebuilder.Source, strings.Join(rebuilder.Argv, " "), err, strings.TrimSpace(string(out)))
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
