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
	"strings"
	"syscall"
	"time"
)

const DefaultReadyTimeout = 30 * time.Second

const pollInterval = 50 * time.Millisecond

const SettleGrace = 500 * time.Millisecond

type Service struct {
	Name         string
	Binary       string
	Args         []string
	AddrEnv      string
	Env          map[string]string
	Ports        []string
	Ready        Ready
	ReadyTimeout time.Duration
}

type Started struct {
	Name       string
	PID        int
	Discovered map[string]int
	Allocated  map[string]int
	LogPath    string
}

func mergedEnv(overlays ...map[string]string) map[string]string {
	merged := map[string]string{}

	for _, entry := range os.Environ() {
		if key, value, ok := strings.Cut(entry, "="); ok {
			merged[key] = value
		}
	}

	for _, overlay := range overlays {
		for key, value := range overlay {
			merged[key] = value
		}
	}

	return merged
}

func sortedEnv(merged map[string]string) []string {
	keys := make([]string, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, key+"="+merged[key])
	}

	return env
}

func Environment(base map[string]string, service Service) []string {
	merged := mergedEnv(base, service.Env)

	if len(service.Ports) == 0 {
		merged[service.AddrEnv] = LoopbackHost + ":0"
	}

	return sortedEnv(merged)
}

func Start(ctx context.Context, tmpDir string, base map[string]string, service Service) (Started, error) {
	allocated, err := AllocatePorts(service.Ports)
	if err != nil {
		return Started{}, fmt.Errorf("binding the ports of service %q: %w", service.Name, err)
	}

	placeholders := Placeholders{Ports: allocated, TmpDir: tmpDir}

	resolved, err := resolvePlaceholders(placeholders, service)
	if err != nil {
		return Started{}, fmt.Errorf("reading the declaration of service %q: %w", service.Name, err)
	}

	if err := resolved.Ready.Validate(allocated); err != nil {
		return Started{}, fmt.Errorf("reading the readiness of service %q: %w", service.Name, err)
	}

	logPath := filepath.Join(tmpDir, service.Name+".log")

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return Started{}, fmt.Errorf("opening the log of service %q: %w", service.Name, err)
	}

	cmd := exec.Command(resolved.Binary, resolved.Args...)
	cmd.Env = Environment(base, resolved)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	startErr := cmd.Start()

	if err := logFile.Close(); err != nil {
		return Started{}, fmt.Errorf("closing the log of service %q: %w", service.Name, err)
	}

	if startErr != nil {
		return Started{}, fmt.Errorf("starting service %q from %s: %w", service.Name, resolved.Binary, startErr)
	}

	exited := make(chan error, 1)

	go func() { exited <- cmd.Wait() }()

	discovered, err := awaitReady(ctx, resolved, allocated, logPath, exited)
	if err != nil {
		terminate(cmd.Process.Pid)

		return Started{}, fmt.Errorf("waiting for service %q on %s: %w", service.Name, logPath, err)
	}

	return Started{
		Name:       service.Name,
		PID:        cmd.Process.Pid,
		Discovered: discovered,
		Allocated:  allocated,
		LogPath:    logPath,
	}, nil
}

func resolvePlaceholders(placeholders Placeholders, service Service) (Service, error) {
	args, err := placeholders.SubstituteAll(service.Args)
	if err != nil {
		return Service{}, fmt.Errorf("reading args: %w", err)
	}

	env, err := placeholders.SubstituteMap(service.Env)
	if err != nil {
		return Service{}, fmt.Errorf("reading env: %w", err)
	}

	service.Args = args
	service.Env = env

	return service, nil
}

func awaitReady(ctx context.Context, service Service, allocated map[string]int, logPath string, exited <-chan error) (map[string]int, error) {
	timeout := readyTimeout(service)

	switch service.Ready.Kind {
	case ReadyStdout:
		return awaitListening(ctx, logPath, timeout, exited)
	case ReadyTCP:
		return nil, awaitTCP(ctx, probeAddress(allocated, service.Ready.Port), timeout, exited)
	case ReadyHTTP:
		url := "http://" + probeAddress(allocated, service.Ready.Port) + service.Ready.Path

		return nil, awaitHTTP(ctx, url, service.Ready.Status, timeout, exited)
	default:
		return nil, fmt.Errorf("readiness kind %q: not a kind; the kinds are %s", service.Ready.Kind, ReadyKinds)
	}
}

func readyTimeout(service Service) time.Duration {
	if service.ReadyTimeout <= 0 {
		return DefaultReadyTimeout
	}

	return service.ReadyTimeout
}

func awaitListening(ctx context.Context, logPath string, timeout time.Duration, exited <-chan error) (map[string]int, error) {
	deadline := time.After(timeout)

	var settle <-chan time.Time

	for {
		output, err := os.ReadFile(logPath)
		if err != nil {
			return nil, fmt.Errorf("reading the log: %w", err)
		}

		ports, ok := FindListening(string(output))
		if ok && settle == nil {
			settle = time.After(SettleGrace)
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("cancelled: %w", ctx.Err())
		case err := <-exited:
			return nil, fmt.Errorf("the process exited before printing LISTENING: %v", err)
		case <-settle:
			return ports, nil
		case <-deadline:
			return nil, fmt.Errorf("no LISTENING line within %s", timeout)
		case <-time.After(pollInterval):
		}
	}
}

func terminate(pid int) {
	_ = syscall.Kill(-pid, syscall.SIGKILL)
}
