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
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/alexandremahdhaoui/forge/pkg/engineframework"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/testenvsqlite"
)

func Create(ctx context.Context, input engineframework.CreateInput, spec *Spec) (*engineframework.TestEnvArtifact, error) {
	writer, err := testenvsqlite.DetectWriter()
	if err != nil {
		return nil, err
	}

	return createWith(ctx, input, spec, writer)
}

func createWith(ctx context.Context, input engineframework.CreateInput, spec *Spec, writer testenvsqlite.Writer) (*engineframework.TestEnvArtifact, error) {
	doc, err := os.ReadFile(resolve(input.RootDir, spec.SpecPath))
	if err != nil {
		return nil, fmt.Errorf("reading the OpenAPI document at %s: %w", spec.SpecPath, err)
	}

	var vectors []byte

	if spec.Seed != "" {
		vectors, err = os.ReadFile(resolve(input.RootDir, spec.Seed))
		if err != nil {
			return nil, fmt.Errorf("reading the vectors file at %s: %w", spec.Seed, err)
		}
	}

	databases, err := testenvsqlite.Plan(doc, spec.Stores, vectors)
	if err != nil {
		return nil, fmt.Errorf("planning the stores of %s: %w", spec.SpecPath, err)
	}

	artifact := &engineframework.TestEnvArtifact{
		TestID:           input.TestID,
		Files:            map[string]string{},
		Env:              map[string]string{},
		Metadata:         map[string]string{"testenv-sqlite.writer": writer.Name},
		ManagedResources: []string{},
	}

	for _, database := range databases {
		fileName := database.Store.Snake + ".db"
		path := filepath.Join(input.TmpDir, fileName)

		if err := writer.Write(ctx, path, database.Script); err != nil {
			return nil, fmt.Errorf("creating the %s store: %w", database.Store.Name, err)
		}

		artifact.Files["sqlite."+database.Store.Snake] = fileName
		artifact.Env[pathEnv(spec, database.Store.Name, database.Store.Upper)] = path
		artifact.Metadata["testenv-sqlite."+database.Store.Snake+".rows"] = strconv.Itoa(len(database.Rows))

		if !spec.Keep {
			artifact.ManagedResources = append(artifact.ManagedResources, path)
		}
	}

	return artifact, nil
}

// pathEnv answers the name a store's path is exported as. The consumer names
// it, because the binary that opens the file is the one whose config key has
// to match. The default keeps every stack that predates the field working.
func pathEnv(spec *Spec, store, upper string) string {
	if name, named := spec.PathEnv[store]; named && name != "" {
		return name
	}

	return "SONGE_STORE_" + upper + "_PATH"
}

func Delete(_ context.Context, _ engineframework.DeleteInput, _ *Spec) error {
	return nil
}

func resolve(rootDir string, path string) string {
	if filepath.IsAbs(path) || rootDir == "" {
		return path
	}

	return filepath.Join(rootDir, path)
}
