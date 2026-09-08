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

func TestCheckAnswersNothingWhenTheBuildWritesNothing(t *testing.T) {
	root := repoWithFakeForge(t, "exit 0")

	findings := checkOK(t, root)

	if len(findings) != 0 {
		t.Fatalf("expected no finding, got %v", findings)
	}
}

func TestCheckNamesAFileTheBuildWroteOverAsRewritten(t *testing.T) {
	root := repoWithFakeForge(t, "printf 'changed\\n' > zz_generated.rs")

	assertOneFinding(t, checkOK(t, root), "zz_generated.rs", drift.Rewritten)
}

func TestCheckNamesAFileTheBuildCreatedAndTheRepoDoesNotIgnoreAsWritten(t *testing.T) {
	root := repoWithFakeForge(t, "printf 'new\\n' > zz_generated_extra.rs")

	assertOneFinding(t, checkOK(t, root), "zz_generated_extra.rs", drift.Written)
}

func TestCheckSaysNothingAboutAFileTheRepoGitignores(t *testing.T) {
	root := repoWithFakeForge(t, "mkdir -p build && printf 'binary\\n' > build/probe")

	findings := checkOK(t, root)

	if len(findings) != 0 {
		t.Fatalf("expected no finding, got %v", findings)
	}
}

func TestCheckNamesAFileTheBuildDeletedAsRemoved(t *testing.T) {
	root := repoWithFakeForge(t, "rm zz_generated.rs")

	assertOneFinding(t, checkOK(t, root), "zz_generated.rs", drift.Removed)
}

func TestCheckNamesAFileEditedBeforeTheGateOnlyWhenTheBuildWritesOverIt(t *testing.T) {
	root := repoWithFakeForge(t, "printf 'generated\\n' > zz_generated.rs")

	write(t, filepath.Join(root, "zz_generated.rs"), "edited by hand\n")

	assertOneFinding(t, checkOK(t, root), "zz_generated.rs", drift.Rewritten)
}

func TestCheckSaysNothingAboutAFileEditedBeforeTheGateThatTheBuildLeavesAlone(t *testing.T) {
	root := repoWithFakeForge(t, "exit 0")

	write(t, filepath.Join(root, "zz_generated.rs"), "edited by hand\n")

	findings := checkOK(t, root)

	if len(findings) != 0 {
		t.Fatalf("expected no finding, got %v", findings)
	}
}

func TestCheckHidesTheArtifactStoreFromTheBuildSoNoEntryCanSkip(t *testing.T) {
	root := repoWithFakeForge(t, "test ! -e .forge/artifact-store.yaml || { echo store still there >&2; exit 1; }")

	if _, err := drift.Check(drift.Options{RootDir: root}); err != nil {
		t.Fatalf("expected the build to find no store, got %v", err)
	}
}

func TestCheckPutsTheArtifactStoreBackAfterTheBuild(t *testing.T) {
	root := repoWithFakeForge(t, "exit 0")

	checkOK(t, root)

	got := read(t, filepath.Join(root, ".forge", "artifact-store.yaml"))
	if got != "artifacts: []\n" {
		t.Fatalf("expected the store to be put back, got %q", got)
	}
}

func TestCheckPutsTheArtifactStoreBackWhenTheBuildFails(t *testing.T) {
	root := repoWithFakeForge(t, "exit 3")

	if _, err := drift.Check(drift.Options{RootDir: root}); err == nil {
		t.Fatal("expected a failed build to be an error")
	}

	got := read(t, filepath.Join(root, ".forge", "artifact-store.yaml"))
	if got != "artifacts: []\n" {
		t.Fatalf("expected the store to be put back, got %q", got)
	}
}

func TestCheckWrapsAFailedBuildWithTheRootDirectoryAndTheBuildOutput(t *testing.T) {
	root := repoWithFakeForge(t, "echo hexagonal-rust exploded >&2; exit 1")

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

func TestCheckRefusesAForgeYamlThatNamesNoArtifactStorePath(t *testing.T) {
	root := repoWithFakeForge(t, "exit 0")

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
	root := repoWithFakeForge(t, "exit 0")

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

func TestCheckWrapsAnUnreadableArtifactStoreWithItsPath(t *testing.T) {
	root := repoWithFakeForge(t, "exit 0")

	store := filepath.Join(root, ".forge", "artifact-store.yaml")

	remove(t, store)

	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatalf("making %q unreadable: %v", store, err)
	}

	_, err := drift.Check(drift.Options{RootDir: root})
	if err == nil {
		t.Fatal("expected an unreadable artifact store to be refused")
	}

	if !strings.Contains(err.Error(), "reading the artifact store") {
		t.Fatalf("expected the action in the message, got %v", err)
	}

	if !strings.Contains(err.Error(), store) {
		t.Fatalf("expected the store path in the message, got %v", err)
	}
}

func TestCheckWrapsAnArtifactStoreItCannotMoveAsideWithItsPath(t *testing.T) {
	root := repoWithFakeForge(t, "exit 0")
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
	root := repoWithFakeForge(t, "chmod 500 .forge")

	_, err := drift.Check(drift.Options{RootDir: root})
	if err == nil {
		t.Fatal("expected a store that cannot be put back to be refused")
	}

	unsealDirectory(t, filepath.Join(root, ".forge"))

	if !strings.Contains(err.Error(), "restoring the artifact store of") {
		t.Fatalf("expected the action in the message, got %v", err)
	}
}

func TestCheckAnswersNothingWhenTheBuildOnlyRewritesTheArtifactStore(t *testing.T) {
	root := repoWithFakeForge(t, "mkdir -p .forge && printf 'artifacts: [rebuilt]\\n' > .forge/artifact-store.yaml")

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

func repoWithFakeForge(t *testing.T, body string) string {
	t.Helper()

	root := t.TempDir()

	write(t, filepath.Join(root, "forge.yaml"), forgeYAML)
	write(t, filepath.Join(root, ".gitignore"), "/.forge/\n/build/\n")
	write(t, filepath.Join(root, "zz_generated.rs"), "generated\n")
	write(t, filepath.Join(root, ".forge", "artifact-store.yaml"), "artifacts: []\n")

	git(t, root, "init")
	git(t, root, "add", "forge.yaml", ".gitignore", "zz_generated.rs")

	bin := t.TempDir()
	write(t, filepath.Join(bin, "forge"), "#!/bin/sh\n"+body+"\n")

	if err := os.Chmod(filepath.Join(bin, "forge"), 0o755); err != nil {
		t.Fatalf("making the fake forge runnable: %v", err)
	}

	t.Setenv("FORGE_RUN_LOCAL_ENABLED", "")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	return root
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

func remove(t *testing.T, path string) {
	t.Helper()

	if err := os.Remove(path); err != nil {
		t.Fatalf("removing %q: %v", path, err)
	}
}
