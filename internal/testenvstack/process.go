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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"syscall"
	"time"
)

const DefaultReadyTimeout = 30 * time.Second

const pollInterval = 50 * time.Millisecond

const SettleGrace = 500 * time.Millisecond

type Service struct {
	Name         string
	Binary       string
	AddrEnv      string
	Env          map[string]string
	ReadyTimeout time.Duration
}

type Started struct {
	Name    string
	PID     int
	Port    int
	Ports   Ports
	LogPath string
}

func Environment(base map[string]string, service Service) []string {
	merged := map[string]string{}

	for _, kv := range os.Environ() {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				merged[kv[:i]] = kv[i+1:]

				break
			}
		}
	}

	for k, v := range base {
		merged[k] = v
	}

	for k, v := range service.Env {
		merged[k] = v
	}

	merged[service.AddrEnv] = "127.0.0.1:0"

	keys := make([]string, 0, len(merged))
	for k := range merged {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	env := make([]string, 0, len(keys))
	for _, k := range keys {
		env = append(env, k+"="+merged[k])
	}

	return env
}

func Start(ctx context.Context, tmpDir string, base map[string]string, service Service) (Started, error) {
	logPath := filepath.Join(tmpDir, service.Name+".log")

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return Started{}, fmt.Errorf("opening the log of service %q: %w", service.Name, err)
	}

	cmd := exec.Command(service.Binary)
	cmd.Env = Environment(base, service)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	startErr := cmd.Start()

	if err := logFile.Close(); err != nil {
		return Started{}, fmt.Errorf("closing the log of service %q: %w", service.Name, err)
	}

	if startErr != nil {
		return Started{}, fmt.Errorf("starting service %q from %s: %w", service.Name, service.Binary, startErr)
	}

	exited := make(chan error, 1)

	go func() { exited <- cmd.Wait() }()

	ports, err := awaitListening(ctx, logPath, readyTimeout(service), exited)
	if err != nil {
		terminate(cmd.Process.Pid)

		return Started{}, fmt.Errorf("waiting for service %q on %s: %w", service.Name, logPath, err)
	}

	return Started{Name: service.Name, PID: cmd.Process.Pid, Port: ports.Rest, Ports: ports, LogPath: logPath}, nil
}

func readyTimeout(service Service) time.Duration {
	if service.ReadyTimeout <= 0 {
		return DefaultReadyTimeout
	}

	return service.ReadyTimeout
}

func awaitListening(ctx context.Context, logPath string, timeout time.Duration, exited <-chan error) (Ports, error) {
	deadline := time.After(timeout)

	var settle <-chan time.Time

	for {
		output, err := os.ReadFile(logPath)
		if err != nil {
			return Ports{}, fmt.Errorf("reading the log: %w", err)
		}

		ports, ok := FindListening(string(output))
		if ok && ports.Complete() {
			return ports, nil
		}

		if ok && settle == nil {
			settle = time.After(SettleGrace)
		}

		select {
		case <-ctx.Done():
			return Ports{}, fmt.Errorf("cancelled: %w", ctx.Err())
		case err := <-exited:
			return Ports{}, fmt.Errorf("the process exited before printing LISTENING: %v", err)
		case <-settle:
			return ports, nil
		case <-deadline:
			return Ports{}, fmt.Errorf("no LISTENING line within %s", timeout)
		case <-time.After(pollInterval):
		}
	}
}

func terminate(pid int) {
	_ = syscall.Kill(-pid, syscall.SIGKILL)
}
