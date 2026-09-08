package drift_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/drift"
)

const forgeYAML = `name: probe
artifactStorePath: .forge/artifact-store.yaml
envFile: .envrc
`

func TestCompareAnswersNothingWhenEveryDigestIsUnchanged(t *testing.T) {
	before := map[string]string{"a": "1", "b": "2"}
	after := map[string]string{"a": "1", "b": "2"}

	if got := drift.Compare(before, after); len(got) != 0 {
		t.Fatalf("expected no finding, got %v", got)
	}
}

func TestCompareNamesEveryChangeInPathOrder(t *testing.T) {
	before := map[string]string{"gone.rs": "1", "same.rs": "2", "stale.rs": "3"}
	after := map[string]string{"fresh.rs": "9", "same.rs": "2", "stale.rs": "4"}

	want := []drift.Finding{
		{Path: "fresh.rs", Change: drift.Written},
		{Path: "gone.rs", Change: drift.Removed},
		{Path: "stale.rs", Change: drift.Rewritten},
	}

	got := drift.Compare(before, after)

	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("at %d expected %v, got %v", i, want[i], got[i])
		}
	}
}

func TestResolveForgeRunsTheWorkspaceCheckoutWhenTheBuildRecordsNoForgeVersion(t *testing.T) {
	stubForgeOnPath(t, "echo forge version v0.1.0")

	resolved, err := drift.ResolveForge("")
	if err != nil {
		t.Fatalf("resolving a forge for a workspace build: %v", err)
	}

	want := []string{"go", "run", "github.com/alexandremahdhaoui/forge/cmd/forge"}

	if strings.Join(resolved.Argv, " ") != strings.Join(want, " ") {
		t.Fatalf("expected %v, got %v", want, resolved.Argv)
	}

	if resolved.Source != "workspace" {
		t.Fatalf("expected the workspace source, got %q", resolved.Source)
	}
}

func TestResolveForgeAcceptsAForgeReportingTheVersionTheGateWasBuiltAgainst(t *testing.T) {
	stub := stubForgeOnPath(t, "echo forge version v1.2.3")

	resolved, err := drift.ResolveForge("v1.2.3")
	if err != nil {
		t.Fatalf("resolving a forge of the version this gate was built against: %v", err)
	}

	if resolved.Argv[0] != stub {
		t.Fatalf("expected the resolved path %q, got %v", stub, resolved.Argv)
	}
}

func TestResolveForgeAcceptsAForgeBuiltDirtyFromTheVersionTheGateWasBuiltAgainst(t *testing.T) {
	stubForgeOnPath(t, "echo forge version v1.2.3+dirty")

	if _, err := drift.ResolveForge("v1.2.3"); err != nil {
		t.Fatalf("expected a dirty build of the same version to be accepted, got %v", err)
	}
}

func TestResolveForgeRefusesAForgeOfAnotherVersionNamingBothVersionsAndThePath(t *testing.T) {
	stub := stubForgeOnPath(t, "echo forge version v0.46.0")

	_, err := drift.ResolveForge("v0.49.0")
	if err == nil {
		t.Fatal("expected a forge of another version to be refused")
	}

	for _, want := range []string{"v0.46.0", "v0.49.0", stub} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected %q in the refusal, got %v", want, err)
		}
	}
}

func TestResolveForgeRefusesAForgeThatNamesNoVersion(t *testing.T) {
	stubForgeOnPath(t, "echo hello from somewhere else")

	_, err := drift.ResolveForge("v0.49.0")
	if err == nil {
		t.Fatal("expected a forge that names no version to be refused")
	}

	if !strings.Contains(err.Error(), "names no version") {
		t.Fatalf("expected the refusal to say so, got %v", err)
	}

	if !strings.Contains(err.Error(), "hello from somewhere else") {
		t.Fatalf("expected what it answered in the refusal, got %v", err)
	}
}

func TestResolveForgeWrapsAForgeThatCannotAnswerItsVersion(t *testing.T) {
	stubForgeOnPath(t, "echo cannot start >&2; exit 4")

	_, err := drift.ResolveForge("v0.49.0")
	if err == nil {
		t.Fatal("expected a forge that cannot answer to be an error")
	}

	if !strings.Contains(err.Error(), "asking the") {
		t.Fatalf("expected the action in the message, got %v", err)
	}

	if !strings.Contains(err.Error(), "cannot start") {
		t.Fatalf("expected the output in the message, got %v", err)
	}
}

func TestResolveForgeWrapsARefNothingOnThePathResolves(t *testing.T) {
	t.Setenv("FORGE_RUN_LOCAL_ENABLED", "")
	t.Setenv("PATH", t.TempDir())

	_, err := drift.ResolveForge("v0.49.0")
	if err == nil {
		t.Fatal("expected an unresolvable forge to be an error")
	}

	if !strings.Contains(err.Error(), "resolving a forge built from v0.49.0") {
		t.Fatalf("expected the action and the version in the message, got %v", err)
	}
}

func TestCheckAnswersNothingWhenTheBuildWritesNothing(t *testing.T) {
	root := repoWithStubbedWorkspaceForge(t, "exit 0")

	findings := checkOK(t, root)

	if len(findings) != 0 {
		t.Fatalf("expected no finding, got %v", findings)
	}
}

func TestCheckReadsTheCurrentDirectoryWhenNoRootDirectoryIsNamed(t *testing.T) {
	root := repoWithStubbedWorkspaceForge(t, "printf 'changed\\n' > zz_generated.rs")

	t.Chdir(root)

	findings, err := drift.Check(drift.Options{})
	if err != nil {
		t.Fatalf("checking the current directory: %v", err)
	}

	assertOneFinding(t, findings, "zz_generated.rs", drift.Rewritten)
}

func TestCheckNamesAFileTheBuildWroteOverAsRewritten(t *testing.T) {
	root := repoWithStubbedWorkspaceForge(t, "printf 'changed\\n' > zz_generated.rs")

	assertOneFinding(t, checkOK(t, root), "zz_generated.rs", drift.Rewritten)
}

func TestCheckNamesAFileTheBuildCreatedAndTheRepoDoesNotIgnoreAsWritten(t *testing.T) {
	root := repoWithStubbedWorkspaceForge(t, "printf 'new\\n' > zz_generated_extra.rs")

	assertOneFinding(t, checkOK(t, root), "zz_generated_extra.rs", drift.Written)
}

func TestCheckSaysNothingAboutAFileTheRepoGitignores(t *testing.T) {
	root := repoWithStubbedWorkspaceForge(t, "mkdir -p build && printf 'binary\\n' > build/probe")

	findings := checkOK(t, root)

	if len(findings) != 0 {
		t.Fatalf("expected no finding, got %v", findings)
	}
}

func TestCheckNamesAFileTheBuildDeletedAsRemoved(t *testing.T) {
	root := repoWithStubbedWorkspaceForge(t, "rm zz_generated.rs")

	assertOneFinding(t, checkOK(t, root), "zz_generated.rs", drift.Removed)
}

func TestCheckNamesAFileEditedBeforeTheGateOnlyWhenTheBuildWritesOverIt(t *testing.T) {
	root := repoWithStubbedWorkspaceForge(t, "printf 'generated\\n' > zz_generated.rs")

	write(t, filepath.Join(root, "zz_generated.rs"), "edited by hand\n")

	assertOneFinding(t, checkOK(t, root), "zz_generated.rs", drift.Rewritten)
}

func TestCheckSaysNothingAboutAFileEditedBeforeTheGateThatTheBuildLeavesAlone(t *testing.T) {
	root := repoWithStubbedWorkspaceForge(t, "exit 0")

	write(t, filepath.Join(root, "zz_generated.rs"), "edited by hand\n")

	findings := checkOK(t, root)

	if len(findings) != 0 {
		t.Fatalf("expected no finding, got %v", findings)
	}
}

func TestCheckHidesTheArtifactStoreFromTheBuildSoNoEntryCanSkip(t *testing.T) {
	root := repoWithStubbedWorkspaceForge(t, "test ! -e .forge/artifact-store.yaml || { echo store still there >&2; exit 1; }")

	if _, err := drift.Check(drift.Options{RootDir: root}); err != nil {
		t.Fatalf("expected the build to find no store, got %v", err)
	}
}

func TestCheckMovesTheArtifactStoreToASiblingSoAnInterruptedRunLeavesItOnDisk(t *testing.T) {
	root := repoWithStubbedWorkspaceForge(t, "test -e .forge/artifact-store.yaml.aside || { echo no sibling >&2; exit 1; }")

	if _, err := drift.Check(drift.Options{RootDir: root}); err != nil {
		t.Fatalf("expected the store to be moved to a sibling, got %v", err)
	}
}

func TestCheckPutsTheArtifactStoreBackAfterTheBuild(t *testing.T) {
	root := repoWithStubbedWorkspaceForge(t, "exit 0")

	checkOK(t, root)

	got := read(t, filepath.Join(root, ".forge", "artifact-store.yaml"))
	if got != "artifacts: []\n" {
		t.Fatalf("expected the store to be put back, got %q", got)
	}
}

func TestCheckPutsTheArtifactStoreBackWhenTheBuildFails(t *testing.T) {
	root := repoWithStubbedWorkspaceForge(t, "exit 3")

	if _, err := drift.Check(drift.Options{RootDir: root}); err == nil {
		t.Fatal("expected a failed build to be an error")
	}

	got := read(t, filepath.Join(root, ".forge", "artifact-store.yaml"))
	if got != "artifacts: []\n" {
		t.Fatalf("expected the store to be put back, got %q", got)
	}
}

func TestCheckLeavesNoArtifactStoreWhereTheRepoHadNone(t *testing.T) {
	root := repoWithStubbedWorkspaceForge(t, "mkdir -p .forge && printf 'artifacts: [fresh]\\n' > .forge/artifact-store.yaml")

	store := filepath.Join(root, ".forge", "artifact-store.yaml")

	remove(t, store)

	if _, err := drift.Check(drift.Options{RootDir: root}); err != nil {
		t.Fatalf("checking a repo with no artifact store: %v", err)
	}

	if _, err := os.Stat(store); !os.IsNotExist(err) {
		t.Fatalf("expected no store where the repo had none, got %v", err)
	}
}

func TestCheckWrapsAFailedBuildWithTheRootDirectoryAndTheBuildOutput(t *testing.T) {
	root := repoWithStubbedWorkspaceForge(t, "echo hexagonal-rust exploded >&2; exit 1")

	_, err := drift.Check(drift.Options{RootDir: root})
	if err == nil {
		t.Fatal("expected a failed build to be an error")
	}

	if !strings.Contains(err.Error(), "running forge build in") {
		t.Fatalf("expected the action in the message, got %v", err)
	}

	if !strings.Contains(err.Error(), root) {
		t.Fatalf("expected the root directory in the message, got %v", err)
	}

	if !strings.Contains(err.Error(), "hexagonal-rust exploded") {
		t.Fatalf("expected the build output in the message, got %v", err)
	}
}

func TestCheckNamesTheForgeItRanWhenTheBuildFails(t *testing.T) {
	root := repoWithStubbedWorkspaceForge(t, "exit 1")

	_, err := drift.Check(drift.Options{RootDir: root})
	if err == nil {
		t.Fatal("expected a failed build to be an error")
	}

	if !strings.Contains(err.Error(), "go run github.com/alexandremahdhaoui/forge/cmd/forge") {
		t.Fatalf("expected the forge it ran in the message, got %v", err)
	}

	if !strings.Contains(err.Error(), "workspace") {
		t.Fatalf("expected the source in the message, got %v", err)
	}
}

func TestCheckRefusesAForgeYamlThatNamesNoArtifactStorePath(t *testing.T) {
	root := repoWithStubbedWorkspaceForge(t, "exit 0")

	write(t, filepath.Join(root, "forge.yaml"), "name: probe\nenvFile: .envrc\n")

	_, err := drift.Check(drift.Options{RootDir: root})
	if err == nil {
		t.Fatal("expected a forge.yaml with no artifactStorePath to be refused")
	}

	if !strings.Contains(err.Error(), "artifactStorePath is required") {
		t.Fatalf("expected the key to be named, got %v", err)
	}
}

func TestCheckRefusesARootDirectoryWithNoForgeYaml(t *testing.T) {
	root := repoWithStubbedWorkspaceForge(t, "exit 0")

	remove(t, filepath.Join(root, "forge.yaml"))

	_, err := drift.Check(drift.Options{RootDir: root})
	if err == nil {
		t.Fatal("expected a root with no forge.yaml to be refused")
	}

	if !strings.Contains(err.Error(), "reading the forge.yaml of") {
		t.Fatalf("expected the action in the message, got %v", err)
	}
}

func TestCheckRefusesARootDirectoryGitDoesNotKnow(t *testing.T) {
	_, err := drift.Check(drift.Options{RootDir: t.TempDir()})
	if err == nil {
		t.Fatal("expected a directory outside a git repository to be refused")
	}

	if !strings.Contains(err.Error(), "listing the files git shows in") {
		t.Fatalf("expected the action in the message, got %v", err)
	}
}

func TestCheckWrapsAFailureToListTheFilesAfterTheBuild(t *testing.T) {
	root := repoWithStubbedWorkspaceForge(t, "rm -rf .git")

	_, err := drift.Check(drift.Options{RootDir: root})
	if err == nil {
		t.Fatal("expected a build that unmakes the git repository to be an error")
	}

	if !strings.Contains(err.Error(), "listing the files git shows in") {
		t.Fatalf("expected the action in the message, got %v", err)
	}
}

func TestCheckWrapsAFileItCannotOpenToDigestWithItsPath(t *testing.T) {
	root := repoWithStubbedWorkspaceForge(t, "exit 0")

	secret := filepath.Join(root, "zz_generated_secret.rs")

	write(t, secret, "generated\n")
	sealFile(t, secret)

	_, err := drift.Check(drift.Options{RootDir: root})
	if err == nil {
		t.Fatal("expected a file that cannot be opened to be an error")
	}

	if !strings.Contains(err.Error(), "to digest it") {
		t.Fatalf("expected the action in the message, got %v", err)
	}

	if !strings.Contains(err.Error(), secret) {
		t.Fatalf("expected the path in the message, got %v", err)
	}
}

func TestCheckRefusesWhenItCannotTakeTheArtifactStoreLock(t *testing.T) {
	root := repoWithStubbedWorkspaceForge(t, "exit 0")

	remove(t, filepath.Join(root, ".forge", "artifact-store.yaml.lock"))
	sealDirectory(t, filepath.Join(root, ".forge"))

	_, err := drift.Check(drift.Options{RootDir: root})
	if err == nil {
		t.Fatal("expected a lock that cannot be opened to be refused")
	}

	if !strings.Contains(err.Error(), "opening the artifact store lock") {
		t.Fatalf("expected the action in the message, got %v", err)
	}
}

func TestCheckRefusesWhenTheArtifactStoreDirectoryIsAFile(t *testing.T) {
	root := repoWithStubbedWorkspaceForge(t, "exit 0")

	write(t, filepath.Join(root, "forge.yaml"), "name: probe\nartifactStorePath: blocked/store.yaml\nenvFile: .envrc\n")
	write(t, filepath.Join(root, "blocked"), "not a directory\n")

	_, err := drift.Check(drift.Options{RootDir: root})
	if err == nil {
		t.Fatal("expected a store directory that is a file to be refused")
	}

	if !strings.Contains(err.Error(), "creating the directory of the artifact store lock") {
		t.Fatalf("expected the action in the message, got %v", err)
	}
}

func TestCheckWrapsAFailureToTakeTheArtifactStoreLockAfterTheBuild(t *testing.T) {
	root := repoWithStubbedWorkspaceForge(t, "rm .forge/artifact-store.yaml.lock && chmod 500 .forge")

	_, err := drift.Check(drift.Options{RootDir: root})

	unsealDirectory(t, filepath.Join(root, ".forge"))

	if err == nil {
		t.Fatal("expected a lock that cannot be reopened to be an error")
	}

	if !strings.Contains(err.Error(), "opening the artifact store lock") {
		t.Fatalf("expected the action in the message, got %v", err)
	}
}

func TestCheckWrapsAnArtifactStoreItCannotRemoveWhereTheRepoHadNone(t *testing.T) {
	root := repoWithStubbedWorkspaceForge(t,
		"mkdir -p .forge && printf 'artifacts: [fresh]\\n' > .forge/artifact-store.yaml && chmod 500 .forge")

	remove(t, filepath.Join(root, ".forge", "artifact-store.yaml"))

	_, err := drift.Check(drift.Options{RootDir: root})

	unsealDirectory(t, filepath.Join(root, ".forge"))

	if err == nil {
		t.Fatal("expected a store this run created and cannot remove to be an error")
	}

	if !strings.Contains(err.Error(), "removing the artifact store") {
		t.Fatalf("expected the action in the message, got %v", err)
	}
}

func TestCheckWrapsAnArtifactStoreItCannotMoveAsideWithItsPath(t *testing.T) {
	root := repoWithStubbedWorkspaceForge(t, "exit 0")
	store := filepath.Join(root, ".forge", "artifact-store.yaml")

	sealDirectory(t, filepath.Dir(store))

	_, err := drift.Check(drift.Options{RootDir: root})
	if err == nil {
		t.Fatal("expected a store that cannot be moved aside to be refused")
	}

	if !strings.Contains(err.Error(), "moving the artifact store") {
		t.Fatalf("expected the action in the message, got %v", err)
	}

	if !strings.Contains(err.Error(), store) {
		t.Fatalf("expected the store path in the message, got %v", err)
	}
}

func TestCheckWrapsAnArtifactStoreItCannotPutBackWithItsPath(t *testing.T) {
	root := repoWithStubbedWorkspaceForge(t, "chmod 500 .forge")

	_, err := drift.Check(drift.Options{RootDir: root})
	if err == nil {
		t.Fatal("expected a store that cannot be put back to be refused")
	}

	unsealDirectory(t, filepath.Join(root, ".forge"))

	if !strings.Contains(err.Error(), "putting the artifact store") {
		t.Fatalf("expected the action in the message, got %v", err)
	}
}

func TestCheckAnswersNothingWhenTheBuildOnlyRewritesTheArtifactStore(t *testing.T) {
	root := repoWithStubbedWorkspaceForge(t, "mkdir -p .forge && printf 'artifacts: [rebuilt]\\n' > .forge/artifact-store.yaml")

	findings := checkOK(t, root)

	if len(findings) != 0 {
		t.Fatalf("expected the ignored store to be invisible, got %v", findings)
	}
}

func checkOK(t *testing.T, root string) []drift.Finding {
	t.Helper()

	findings, err := drift.Check(drift.Options{RootDir: root})
	if err != nil {
		t.Fatalf("checking %q: %v", root, err)
	}

	return findings
}

func assertOneFinding(t *testing.T, findings []drift.Finding, path, change string) {
	t.Helper()

	if len(findings) != 1 {
		t.Fatalf("expected one finding, got %v", findings)
	}

	if findings[0].Path != path || findings[0].Change != change {
		t.Fatalf("expected %s: %s, got %v", path, change, findings[0])
	}
}

func repoWithStubbedWorkspaceForge(t *testing.T, body string) string {
	t.Helper()

	root := t.TempDir()

	write(t, filepath.Join(root, "forge.yaml"), forgeYAML)
	write(t, filepath.Join(root, ".gitignore"), "/.forge/\n/build/\n")
	write(t, filepath.Join(root, "zz_generated.rs"), "generated\n")
	write(t, filepath.Join(root, ".forge", "artifact-store.yaml"), "artifacts: []\n")
	write(t, filepath.Join(root, ".forge", "artifact-store.yaml.lock"), "")

	git(t, root, "init")
	git(t, root, "add", "forge.yaml", ".gitignore", "zz_generated.rs")

	stubBinaryOnPath(t, "go", body)

	return root
}

func stubForgeOnPath(t *testing.T, body string) string {
	t.Helper()

	return stubBinaryOnPath(t, "forge", body)
}

func stubBinaryOnPath(t *testing.T, name, body string) string {
	t.Helper()

	bin := t.TempDir()
	path := filepath.Join(bin, name)

	write(t, path, "#!/bin/sh\n"+body+"\n")

	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("making the stubbed %s runnable: %v", name, err)
	}

	t.Setenv("FORGE_RUN_LOCAL_ENABLED", "")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	return path
}

func git(t *testing.T, root string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = root

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("running git %v in %q: %v: %s", args, root, err, out)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("creating the directory of %q: %v", path, err)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %q: %v", path, err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %q: %v", path, err)
	}

	return string(content)
}

func sealDirectory(t *testing.T, path string) {
	t.Helper()

	if err := os.Chmod(path, 0o500); err != nil {
		t.Fatalf("sealing %q: %v", path, err)
	}

	t.Cleanup(func() { _ = os.Chmod(path, 0o755) })
}

func unsealDirectory(t *testing.T, path string) {
	t.Helper()

	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("unsealing %q: %v", path, err)
	}
}

func sealFile(t *testing.T, path string) {
	t.Helper()

	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("sealing %q: %v", path, err)
	}

	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
}

func remove(t *testing.T, path string) {
	t.Helper()

	if err := os.Remove(path); err != nil {
		t.Fatalf("removing %q: %v", path, err)
	}
}
