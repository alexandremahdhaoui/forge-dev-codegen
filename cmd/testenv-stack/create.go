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

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alexandremahdhaoui/forge/pkg/engineframework"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/testenvstack"
)

const pidsPathKey = "testenv-stack.pidsPath"

func Create(ctx context.Context, input engineframework.CreateInput, spec *Spec) (*engineframework.TestEnvArtifact, error) {
	pidsPath := filepath.Join(input.TmpDir, testenvstack.PidsFileName)

	artifact := &engineframework.TestEnvArtifact{
		TestID:           input.TestID,
		Files:            map[string]string{"stack.pids": testenvstack.PidsFileName},
		Env:              map[string]string{},
		Metadata:         map[string]string{pidsPathKey: pidsPath},
		ManagedResources: []string{pidsPath},
	}

	started := []testenvstack.Started{}

	runtimeEnv := map[string]string{}
	for key, value := range input.Env {
		runtimeEnv[key] = value
	}

	for _, service := range spec.Services {
		one, err := testenvstack.Start(ctx, input.TmpDir, runtimeEnv, toService(input.RootDir, service))
		if err != nil {
			stopAll(started)

			return nil, fmt.Errorf("starting the stack: %w", err)
		}

		started = append(started, one)

		artifact.Files["stack."+service.Name+".log"] = service.Name + ".log"

		for key, value := range testenvstack.Addresses(service.AddrEnv, one.Discovered) {
			artifact.Env[key] = value
			runtimeEnv[key] = value
		}

		for key, value := range testenvstack.PortAddresses(service.AddrEnv, one.Allocated) {
			artifact.Env[key] = value
			runtimeEnv[key] = value
		}

		artifact.Metadata["testenv-stack."+service.Name+".pid"] = strconv.Itoa(one.PID)
		artifact.ManagedResources = append(artifact.ManagedResources, one.LogPath)

		if err := runAfter(ctx, input, service, one, runtimeEnv, artifact); err != nil {
			stopAll(started)

			return nil, err
		}
	}

	if err := testenvstack.WritePids(pidsPath, started); err != nil {
		stopAll(started)

		return nil, fmt.Errorf("recording the stack: %w", err)
	}

	return artifact, nil
}

func Delete(_ context.Context, input engineframework.DeleteInput, _ *Spec) error {
	pidsPath := input.Metadata[pidsPathKey]
	if pidsPath == "" {
		log.Printf("testenv-stack: no pid file recorded for %s, nothing to stop", input.TestID)

		return nil
	}

	pids, err := testenvstack.ReadPids(pidsPath)
	if errors.Is(err, os.ErrNotExist) {
		log.Printf("testenv-stack: pid file %s is gone, nothing to stop", pidsPath)

		return nil
	}

	if err != nil {
		return fmt.Errorf("stopping the stack of %s: %w", input.TestID, err)
	}

	testenvstack.Stop(pids, testenvstack.KillGrace)

	return nil
}

func runAfter(
	ctx context.Context,
	input engineframework.CreateInput,
	service Service,
	started testenvstack.Started,
	runtimeEnv map[string]string,
	artifact *engineframework.TestEnvArtifact,
) error {
	placeholders := testenvstack.Placeholders{Ports: started.Allocated, TmpDir: input.TmpDir}

	for _, after := range service.After {
		value, err := testenvstack.RunAfter(ctx, input.RootDir, runtimeEnv, placeholders, toAfter(input.RootDir, after))
		if err != nil {
			return fmt.Errorf("running the after commands of service %q: %w", service.Name, err)
		}

		artifact.Env[after.Export] = value
		runtimeEnv[after.Export] = value
		artifact.Metadata["testenv-stack."+service.Name+".export."+after.Export] = value
	}

	return nil
}

func toService(rootDir string, service Service) testenvstack.Service {
	return testenvstack.Service{
		Name:         service.Name,
		Binary:       resolve(rootDir, service.Binary),
		Args:         service.Args,
		AddrEnv:      service.AddrEnv,
		Env:          service.Env,
		Ports:        service.Ports,
		Ready:        toReady(service.Ready),
		ReadyTimeout: time.Duration(service.ReadyTimeoutSeconds) * time.Second,
	}
}

func toReady(ready Ready) testenvstack.Ready {
	kind := ready.Kind
	if kind == "" {
		kind = testenvstack.ReadyStdout
	}

	return testenvstack.Ready{Kind: kind, Port: ready.Port, Path: ready.Path, Status: ready.Status}
}

func toAfter(rootDir string, after After) testenvstack.After {
	return testenvstack.After{
		Command:  resolveCommand(rootDir, after.Command),
		Args:     after.Args,
		Export:   after.Export,
		Regex:    after.Regex,
		JSONPath: after.JsonPath,
	}
}

func stopAll(started []testenvstack.Started) {
	pids := make([]int, 0, len(started))
	for _, s := range started {
		pids = append(pids, s.PID)
	}

	testenvstack.Stop(pids, testenvstack.KillGrace)
}

func resolve(rootDir string, path string) string {
	if filepath.IsAbs(path) || rootDir == "" {
		return path
	}

	return filepath.Join(rootDir, path)
}

func resolveCommand(rootDir string, command string) string {
	if !strings.ContainsRune(command, filepath.Separator) {
		return command
	}

	return resolve(rootDir, command)
}
