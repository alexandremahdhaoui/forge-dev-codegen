package windowsallow

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func minimalPE(stampByte byte) []byte {
	pe := make([]byte, 128)
	pe[0] = 'M'
	pe[1] = 'Z'
	elfanew := 64
	binary.LittleEndian.PutUint32(pe[elfanewOffset:elfanewOffset+4], uint32(elfanew))
	pe[elfanew] = 'P'
	pe[elfanew+1] = 'E'
	pe[elfanew+2] = 0
	pe[elfanew+3] = 0
	pe[elfanew+timeDateStampGap] = stampByte
	return pe
}

func TestTheTimestampOffsetLandsEightBytesAfterThePeSignature(t *testing.T) {
	pe := minimalPE(0x11)
	offset, err := timeDateStampOffset(pe)
	if err != nil {
		t.Fatalf("expected a valid offset, got %v", err)
	}
	if offset != 64+timeDateStampGap {
		t.Fatalf("expected the offset at %d, got %d", 64+timeDateStampGap, offset)
	}
}

func TestAFileThatDoesNotStartWithMZIsRefused(t *testing.T) {
	pe := minimalPE(0x11)
	pe[0] = 'X'
	if _, err := timeDateStampOffset(pe); err == nil || !strings.Contains(err.Error(), "MZ magic") {
		t.Fatalf("expected an MZ magic error, got %v", err)
	}
}

func TestAPeHeaderOffsetOutsideTheFileIsRefused(t *testing.T) {
	pe := minimalPE(0x11)
	binary.LittleEndian.PutUint32(pe[elfanewOffset:elfanewOffset+4], uint32(len(pe)+10))
	if _, err := timeDateStampOffset(pe); err == nil || !strings.Contains(err.Error(), "points outside") {
		t.Fatalf("expected an out of range error, got %v", err)
	}
}

func TestAnOffsetThatDoesNotCarryThePeSignatureIsRefused(t *testing.T) {
	pe := minimalPE(0x11)
	pe[64] = 'Z'
	if _, err := timeDateStampOffset(pe); err == nil || !strings.Contains(err.Error(), "PE signature") {
		t.Fatalf("expected a PE signature error, got %v", err)
	}
}

func TestMovingTheHashChangesOnlyTheTimestampByteAndLeavesTheOriginalAlone(t *testing.T) {
	pe := minimalPE(0x11)
	stamp := 64 + timeDateStampGap
	moved := moveHash(pe, stamp, 3)
	if moved[stamp] != 0x14 {
		t.Fatalf("expected the stamp byte to become 0x14, got %#x", moved[stamp])
	}
	if pe[stamp] != 0x11 {
		t.Fatalf("expected the original stamp byte to stay 0x11, got %#x", pe[stamp])
	}
	moved[stamp] = pe[stamp]
	for i := range pe {
		if pe[i] != moved[i] {
			t.Fatalf("expected only the stamp byte to differ, byte %d differs", i)
		}
	}
}

func TestTheCandidateNameCarriesTheNameTheCommitAndTheHash(t *testing.T) {
	got := candidateName("songe-hello-node", "3dd48e9-dirty", "cb14ce19")
	want := "songe-hello-node-3dd48e9-dirty-cb14ce19.exe"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestOutputWithInvalidArgumentIsBlockedAndOutputWithTheExpectIsAllowed(t *testing.T) {
	if !blocked("./x.exe: Invalid argument") {
		t.Fatal("expected Invalid argument to read as blocked")
	}
	if blocked("LISTENING 5000") {
		t.Fatal("expected a listening line not to read as blocked")
	}
	if !allowed("LISTENING 5000\nLISTENING_UDP 5002", "LISTENING") {
		t.Fatal("expected the expect string to read as allowed")
	}
	if allowed("./x.exe: Invalid argument", "LISTENING") {
		t.Fatal("expected a blocked line not to read as allowed")
	}
}

func TestAllowStopsAtTheFirstBuildTheMachineAllowsAndWritesTheMarker(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "songe-hello-node.exe")
	if err := os.WriteFile(src, minimalPE(0x11), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()

	calls := 0
	run := func(_ string, exe string, _ []string, _ time.Duration) (string, error) {
		calls++
		if calls < 3 {
			return exe + ": Invalid argument", nil
		}
		return "LISTENING 5000", nil
	}

	req := Request{
		Source:      src,
		Destination: dest,
		Name:        "songe-hello-node",
		Commit:      "abc1234",
		Attempts:    8,
		Probe:       Probe{Expect: "LISTENING", TimeoutSeconds: 1},
	}
	outcome, err := Allow(req, run)
	if err != nil {
		t.Fatalf("expected an allowed build, got %v", err)
	}
	if outcome.Attempts != 3 {
		t.Fatalf("expected three attempts, got %d", outcome.Attempts)
	}
	marker := filepath.Join(dest, "songe-hello-node-allowed.txt")
	body, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("expected the marker to exist, got %v", err)
	}
	if strings.TrimSpace(string(body)) != outcome.AllowedName {
		t.Fatalf("expected the marker to name %q, got %q", outcome.AllowedName, strings.TrimSpace(string(body)))
	}
}

func TestABlockedCopyIsDeletedAndOnlyTheAllowedBuildRemains(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "songe-hello-node.exe")
	if err := os.WriteFile(src, minimalPE(0x11), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()

	calls := 0
	run := func(_ string, exe string, _ []string, _ time.Duration) (string, error) {
		calls++
		if calls < 2 {
			return exe + ": Invalid argument", nil
		}
		return "LISTENING 5000", nil
	}

	req := Request{Source: src, Destination: dest, Name: "songe-hello-node", Commit: "abc1234", Attempts: 8, Probe: Probe{Expect: "LISTENING", TimeoutSeconds: 1}}
	outcome, err := Allow(req, run)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dest)
	if err != nil {
		t.Fatal(err)
	}
	exes := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".exe") {
			exes++
			if e.Name() != outcome.AllowedName {
				t.Fatalf("expected only the allowed build, found %q", e.Name())
			}
		}
	}
	if exes != 1 {
		t.Fatalf("expected one exe left, found %d", exes)
	}
}

func TestAllowFailsWhenEveryBuildIsRefusedAndNamesTheAttempts(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "songe-hello-node.exe")
	if err := os.WriteFile(src, minimalPE(0x11), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	run := func(_ string, exe string, _ []string, _ time.Duration) (string, error) {
		return exe + ": Invalid argument", nil
	}
	req := Request{Source: src, Destination: dest, Name: "songe-hello-node", Commit: "abc1234", Attempts: 4, Probe: Probe{Expect: "LISTENING", TimeoutSeconds: 1}}
	_, err := Allow(req, run)
	if err == nil || !strings.Contains(err.Error(), "4 builds") {
		t.Fatalf("expected a refusal naming four builds, got %v", err)
	}
	entries, _ := os.ReadDir(dest)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".exe") {
			t.Fatalf("expected no exe left after every build was refused, found %q", e.Name())
		}
	}
}

func seedBuild(t *testing.T, dest, file string, when time.Time) {
	t.Helper()
	path := filepath.Join(dest, file)
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
}

func TestPruningKeepsTheAllowedBuildAndTheNewestOthersAndRemovesTheRest(t *testing.T) {
	dest := t.TempDir()
	names := []string{
		"songe-hello-node-a-11111111.exe",
		"songe-hello-node-a-22222222.exe",
		"songe-hello-node-a-33333333.exe",
		"songe-hello-node-a-44444444.exe",
	}
	base := time.Now()
	for i, n := range names {
		seedBuild(t, dest, n, base.Add(time.Duration(i)*time.Minute))
	}
	allowed := "songe-hello-node-a-44444444.exe"
	pruned, err := pruneOldBuilds(dest, "songe-hello-node", allowed, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(pruned) != 2 {
		t.Fatalf("expected two builds pruned, got %d %v", len(pruned), pruned)
	}
	left := map[string]bool{}
	entries, _ := os.ReadDir(dest)
	for _, e := range entries {
		left[e.Name()] = true
	}
	if !left[allowed] {
		t.Fatal("expected the allowed build to survive")
	}
	if !left["songe-hello-node-a-33333333.exe"] {
		t.Fatal("expected the newest other build to survive")
	}
	if left["songe-hello-node-a-11111111.exe"] || left["songe-hello-node-a-22222222.exe"] {
		t.Fatal("expected the two oldest builds to be pruned")
	}
}

func TestPruningNeverTouchesABuildOfAnotherBinaryOrProject(t *testing.T) {
	dest := t.TempDir()
	base := time.Now()
	others := []string{"GameSync-abcd1234.exe", "OpenDS-5320b95-abf684ce.exe", "songe-launcher-deadbeef.exe"}
	for _, n := range others {
		seedBuild(t, dest, n, base)
	}
	stale := "songe-hello-node-old-99999999.exe"
	seedBuild(t, dest, stale, base)
	if _, err := pruneOldBuilds(dest, "songe-hello-node", "songe-hello-node-new-11111111.exe", 1); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dest)
	left := map[string]bool{}
	for _, e := range entries {
		left[e.Name()] = true
	}
	for _, n := range others {
		if !left[n] {
			t.Fatalf("expected another binary or project build %q to survive", n)
		}
	}
	if left[stale] {
		t.Fatal("expected the old build of this binary to be pruned")
	}
}

func TestABuildOfThisBinaryIsToldApartFromAnother(t *testing.T) {
	if !belongsToThisBinary("songe-hello-node-abc-1234.exe", "songe-hello-node") {
		t.Fatal("expected a build of this binary to match")
	}
	if belongsToThisBinary("songe-launcher-1234.exe", "songe-hello-node") {
		t.Fatal("expected another binary's build not to match")
	}
	if belongsToThisBinary("GameSync-1234.exe", "songe-hello-node") {
		t.Fatal("expected another project's build not to match")
	}
	if belongsToThisBinary("songe-hello-node-allowed.txt", "songe-hello-node") {
		t.Fatal("expected the marker not to match")
	}
}

func TestAllowRefusesADestinationThatIsNotADirectory(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "songe-hello-node.exe")
	if err := os.WriteFile(src, minimalPE(0x11), 0o644); err != nil {
		t.Fatal(err)
	}
	notDir := filepath.Join(dir, "afile")
	if err := os.WriteFile(notDir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	req := Request{Source: src, Destination: notDir, Name: "n", Commit: "c", Attempts: 1, Probe: Probe{Expect: "LISTENING", TimeoutSeconds: 1}}
	_, err := Allow(req, func(_, _ string, _ []string, _ time.Duration) (string, error) { return "", nil })
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("expected a not a directory error, got %v", err)
	}
}
