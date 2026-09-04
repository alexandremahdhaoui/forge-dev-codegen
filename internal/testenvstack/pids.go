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
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const PidsFileName = "stack.pids"

const KillGrace = 5 * time.Second

func WritePids(path string, started []Started) error {
	var b strings.Builder

	for _, s := range started {
		b.WriteString(s.Name + " " + strconv.Itoa(s.PID) + "\n")
	}

	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("writing the pid file %s: %w", path, err)
	}

	return nil
}

func ReadPids(path string) ([]int, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading the pid file %s: %w", path, err)
	}

	return ParsePids(string(content))
}

func ParsePids(content string) ([]int, error) {
	var pids []int

	for _, line := range strings.Split(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		pid, err := strconv.Atoi(fields[len(fields)-1])
		if err != nil {
			return nil, fmt.Errorf("reading pid line %q: %w", line, err)
		}

		pids = append(pids, pid)
	}

	return pids, nil
}

func Alive(pid int) bool {
	err := syscall.Kill(pid, 0)

	return err == nil || errors.Is(err, syscall.EPERM)
}

func Stop(pids []int, grace time.Duration) {
	for _, pid := range pids {
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}

	deadline := time.Now().Add(grace)

	for time.Now().Before(deadline) && anyAlive(pids) {
		time.Sleep(pollInterval)
	}

	for _, pid := range pids {
		if Alive(pid) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}
}

func anyAlive(pids []int) bool {
	for _, pid := range pids {
		if Alive(pid) {
			return true
		}
	}

	return false
}
