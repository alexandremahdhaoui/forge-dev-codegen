// Copyright 2024 Alexandre Mahdhaoui
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package testenvstack

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const fakeService = `#!/bin/sh
echo "booting $HELLO_ADDR extra=$EXTRA"
echo "LISTENING 4321"
sleep 60
`

const exitingService = `#!/bin/sh
echo "dying"
exit 3
`

const silentService = `#!/bin/sh
sleep 60
`

func writeBinary(t *testing.T, script string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "svc.sh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	return path
}

func TestParseListeningReadsThePortAndRefusesEverythingElse(t *testing.T) {
	if port, ok := ParseListening("LISTENING 8080"); !ok || port != 8080 {
		t.Errorf("LISTENING 8080 -> %d %v", port, ok)
	}

	if port, ok := ParseListening("  LISTENING 1  "); !ok || port != 1 {
		t.Errorf("padded line -> %d %v", port, ok)
	}

	for _, line := range []string{"", "LISTENING", "LISTENING x", "LISTENING 0", "LISTENING 70000", "listening 80", "LISTENING 80 more", "READY 80"} {
		if _, ok := ParseListening(line); ok {
			t.Errorf("%q must be refused", line)
		}
	}
}

func TestFindListeningPicksTheFirstListeningLineOutOfMixedOutput(t *testing.T) {
	port, ok := FindListening("starting\nwarn: something\nLISTENING 5555\nLISTENING 6666\n")
	if !ok || port != 5555 {
		t.Errorf("got %d %v", port, ok)
	}

	if _, ok := FindListening("starting\nstill starting\n"); ok {
		t.Error("output without the line must not match")
	}
}

func TestEnvironmentLayersTheServiceEnvOverTheBaseAndForcesAFreePortOnTheDeclaredAddrEnv(t *testing.T) {
	t.Setenv("FROM_PARENT", "parent")

	env := Environment(map[string]string{"SHARED": "base", "HELLO_ADDR": "1.2.3.4:9"}, Service{Name: "hello", AddrEnv: "HELLO_ADDR", Env: map[string]string{"SHARED": "service", "HELLO_ADDR": "5.6.7.8:9"}})

	joined := strings.Join(env, "\n")

	for _, want := range []string{"FROM_PARENT=parent", "SHARED=service", "HELLO_ADDR=127.0.0.1:0"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}

	if strings.Contains(joined, "SHARED=base") || strings.Contains(joined, "HELLO_ADDR=1.2.3.4:9") || strings.Contains(joined, "HELLO_ADDR=5.6.7.8:9") {
		t.Errorf("an overridden value survived:\n%s", joined)
	}
}

func TestParsePidsReadsNamePidLinesAndRefusesGarbage(t *testing.T) {
	pids, err := ParsePids("hello 12\n\nworld 34\n")
	if err != nil || len(pids) != 2 || pids[0] != 12 || pids[1] != 34 {
		t.Errorf("got %v %v", pids, err)
	}

	if _, err := ParsePids("hello twelve\n"); err == nil {
		t.Error("a non numeric pid must be refused")
	}
}

func TestReadPidsReportsAMissingFile(t *testing.T) {
	if _, err := ReadPids(filepath.Join(t.TempDir(), "none")); err == nil {
		t.Error("a missing pid file must be reported")
	}
}

func TestStartReturnsThePortFromTheListeningLineAndStopEndsTheProcess(t *testing.T) {
	binary := writeBinary(t, fakeService)
	tmpDir := t.TempDir()

	started, err := Start(context.Background(), tmpDir, map[string]string{"EXTRA": "yes"}, Service{Name: "hello", Binary: binary, AddrEnv: "HELLO_ADDR", ReadyTimeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}

	if started.Port != 4321 || started.PID <= 0 || started.LogPath != filepath.Join(tmpDir, "hello.log") {
		t.Errorf("started: %+v", started)
	}

	log, _ := os.ReadFile(started.LogPath)
	if !strings.Contains(string(log), "booting 127.0.0.1:0 extra=yes") {
		t.Errorf("log: %s", log)
	}

	if !Alive(started.PID) {
		t.Fatal("the service must outlive Start")
	}

	pidsPath := filepath.Join(tmpDir, PidsFileName)
	if err := WritePids(pidsPath, []Started{started}); err != nil {
		t.Fatal(err)
	}

	pids, err := ReadPids(pidsPath)
	if err != nil || len(pids) != 1 || pids[0] != started.PID {
		t.Fatalf("pids: %v %v", pids, err)
	}

	Stop(pids, 2*time.Second)

	deadline := time.Now().Add(2 * time.Second)
	for Alive(started.PID) && time.Now().Before(deadline) {
		time.Sleep(pollInterval)
	}

	if Alive(started.PID) {
		t.Error("the service must be gone after Stop")
	}
}

func TestStartReportsAProcessThatExitsBeforeListening(t *testing.T) {
	binary := writeBinary(t, exitingService)

	_, err := Start(context.Background(), t.TempDir(), nil, Service{Name: "dying", Binary: binary, ReadyTimeout: 5 * time.Second})
	if err == nil || !strings.Contains(err.Error(), "exited before printing LISTENING") {
		t.Errorf("got %v", err)
	}
}

func TestStartReportsATimeoutAndKillsTheSilentProcess(t *testing.T) {
	binary := writeBinary(t, silentService)

	_, err := Start(context.Background(), t.TempDir(), nil, Service{Name: "silent", Binary: binary, ReadyTimeout: 300 * time.Millisecond})
	if err == nil || !strings.Contains(err.Error(), "no LISTENING line within") {
		t.Errorf("got %v", err)
	}
}

func TestStartReportsAMissingBinaryAndAnUnwritableLog(t *testing.T) {
	if _, err := Start(context.Background(), t.TempDir(), nil, Service{Name: "missing", Binary: "/nonexistent/binary"}); err == nil {
		t.Error("a missing binary must be reported")
	}

	if _, err := Start(context.Background(), filepath.Join(t.TempDir(), "nodir"), nil, Service{Name: "nolog", Binary: "/bin/sh"}); err == nil {
		t.Error("an unwritable log must be reported")
	}
}

func TestStartHonoursACancelledContext(t *testing.T) {
	binary := writeBinary(t, silentService)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Start(ctx, t.TempDir(), nil, Service{Name: "silent", Binary: binary, ReadyTimeout: 5 * time.Second})
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("got %v", err)
	}
}

func TestStopKillsTheGrandchildrenOfAServiceTooBecauseTheyShareItsGroup(t *testing.T) {
	tmpDir := t.TempDir()
	childPidPath := filepath.Join(tmpDir, "child.pid")
	binary := writeBinary(t, "#!/bin/sh\nsleep 60 &\necho $! > "+childPidPath+"\necho \"LISTENING 1\"\nwait\n")

	started, err := Start(context.Background(), tmpDir, nil, Service{Name: "parent", Binary: binary, AddrEnv: "PARENT_ADDR", ReadyTimeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(childPidPath)
	if err != nil {
		t.Fatal(err)
	}

	child, err := ParsePids(string(raw))
	if err != nil || len(child) != 1 || !Alive(child[0]) {
		t.Fatalf("child: %v %v", child, err)
	}

	Stop([]int{started.PID}, 2*time.Second)

	deadline := time.Now().Add(2 * time.Second)
	for (Alive(started.PID) || Alive(child[0])) && time.Now().Before(deadline) {
		time.Sleep(pollInterval)
	}

	if Alive(child[0]) {
		t.Error("the grandchild must be gone after Stop")
	}
}

func TestStopSurvivesAPidThatIsAlreadyGone(t *testing.T) {
	Stop([]int{999999999}, 100*time.Millisecond)
}
