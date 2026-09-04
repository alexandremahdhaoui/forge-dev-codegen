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

	for _, service := range spec.Services {
		one, err := testenvstack.Start(ctx, input.TmpDir, input.Env, toService(input.RootDir, service))
		if err != nil {
			stopAll(started)

			return nil, fmt.Errorf("starting the stack: %w", err)
		}

		started = append(started, one)

		artifact.Files["stack."+service.Name+".log"] = service.Name + ".log"
		artifact.Env[service.AddrEnv] = "http://127.0.0.1:" + strconv.Itoa(one.Port)
		artifact.Metadata["testenv-stack."+service.Name+".pid"] = strconv.Itoa(one.PID)
		artifact.ManagedResources = append(artifact.ManagedResources, one.LogPath)
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

func toService(rootDir string, service Service) testenvstack.Service {
	return testenvstack.Service{
		Name:         service.Name,
		Binary:       resolve(rootDir, service.Binary),
		AddrEnv:      service.AddrEnv,
		Env:          service.Env,
		ReadyTimeout: time.Duration(service.ReadyTimeoutSeconds) * time.Second,
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
