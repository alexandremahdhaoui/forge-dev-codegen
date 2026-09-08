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

package authzgen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"
)

const (
	combinedTestsPath = "zz_generated_fga.yaml"
	combineFile       = "combine.yaml"
)

type CombineOptions struct {
	Name    string
	SrcDir  string
	Modules []string
}

type combineDocument struct {
	Modules []string `json:"modules"`
}

type loadedModule struct {
	path     string
	manifest manifest
	model    string
	tests    testDocument
}

func ModulesFromSpec(doc []byte) ([]string, error) {
	if err := checkCombineShape(doc); err != nil {
		return nil, err
	}

	var read combineDocument

	if err := yaml.UnmarshalStrict(doc, &read); err != nil {
		return nil, fmt.Errorf("reading %s: %w", combineFile, err)
	}

	for i, path := range read.Modules {
		if strings.TrimSpace(path) == "" {
			return nil, fmt.Errorf(
				"reading %s modules[%d]: it is empty, write the path to a %s relative to this cell",
				combineFile, i, manifestPath,
			)
		}
	}

	return read.Modules, nil
}

func checkCombineShape(doc []byte) error {
	var document any

	if err := yaml.Unmarshal(doc, &document); err != nil {
		return fmt.Errorf("reading %s: %w", combineFile, err)
	}

	shape := mappingOf(
		"a mapping naming the modules to combine",
		field{"modules", listOf("a list of paths to a "+manifestPath, text("a path"))},
	)

	return checkShapeOf(combineFile, shape, document)
}

func Combine(opts CombineOptions) ([]File, error) {
	if opts.Name == "" {
		return nil, fmt.Errorf("combining the modules: the cell name is required")
	}

	if len(opts.Modules) == 0 {
		return nil, fmt.Errorf(
			"combining %q: the cell lists no module, name every %s under modules in %s",
			opts.Name, manifestPath, combineFile,
		)
	}

	loaded, listedBy, err := loadModules(opts)
	if err != nil {
		return nil, err
	}

	if err := checkForeignModules(opts.Name, loaded, listedBy); err != nil {
		return nil, err
	}

	schema, err := combinedSchema(opts.Name, loaded)
	if err != nil {
		return nil, err
	}

	tests, err := writeTestDocument(mergeTests(opts.Name, loaded))
	if err != nil {
		return nil, fmt.Errorf("combining %q: %w", opts.Name, err)
	}

	files := []File{
		{Path: modelFile, Content: emitMod(schema, loaded)},
		{Path: combinedTestsPath, Content: tests},
	}

	for _, one := range loaded {
		files = append(files, File{Path: one.manifest.Model, Content: one.model})
	}

	return files, nil
}

func loadModules(opts CombineOptions) ([]loadedModule, map[string]string, error) {
	loaded := make([]loadedModule, 0, len(opts.Modules))
	listedBy := map[string]string{}

	for _, path := range opts.Modules {
		one, err := loadModule(opts.SrcDir, path)
		if err != nil {
			return nil, nil, fmt.Errorf("combining %q: %w", opts.Name, err)
		}

		if other, listed := listedBy[one.manifest.Module]; listed {
			return nil, nil, fmt.Errorf(
				"combining %q: module %q is listed by %s and by %s, list each module once",
				opts.Name, one.manifest.Module, other, path,
			)
		}

		listedBy[one.manifest.Module] = path
		loaded = append(loaded, one)
	}

	return loaded, listedBy, nil
}

func loadModule(srcDir, path string) (loadedModule, error) {
	manifestFile := filepath.Join(srcDir, path)

	raw, err := os.ReadFile(manifestFile)
	if err != nil {
		return loadedModule{}, fmt.Errorf("reading the manifest %s: %w", path, err)
	}

	var read manifest

	if err := json.Unmarshal(raw, &read); err != nil {
		return loadedModule{}, fmt.Errorf("reading the manifest %s: %w", path, err)
	}

	if !ident.MatchString(read.Module) {
		return loadedModule{}, fmt.Errorf(
			"reading the manifest %s: module %q is not a lower case identifier, build the module cell before the combination",
			path, read.Module,
		)
	}

	dir := filepath.Dir(manifestFile)

	model, err := os.ReadFile(filepath.Join(dir, read.Model))
	if err != nil {
		return loadedModule{}, fmt.Errorf("reading the module file of %q: %w", read.Module, err)
	}

	tests, err := os.ReadFile(filepath.Join(dir, read.Tests))
	if err != nil {
		return loadedModule{}, fmt.Errorf("reading the test file of %q: %w", read.Module, err)
	}

	document, err := readTestDocument(tests)
	if err != nil {
		return loadedModule{}, fmt.Errorf("reading the test file of %q: %w", read.Module, err)
	}

	return loadedModule{path: path, manifest: read, model: string(model), tests: document}, nil
}

func readTestDocument(doc []byte) (testDocument, error) {
	var document testDocument

	if err := yaml.Unmarshal(doc, &document); err != nil {
		return testDocument{}, err
	}

	return document, nil
}

func checkForeignModules(name string, loaded []loadedModule, listedBy map[string]string) error {
	for _, one := range loaded {
		foreign := make([]manifestExternal, 0, len(one.manifest.References)+len(one.manifest.Extends))
		foreign = append(foreign, one.manifest.References...)
		foreign = append(foreign, one.manifest.Extends...)

		for _, e := range foreign {
			if _, listed := listedBy[e.Module]; listed {
				continue
			}

			return fmt.Errorf(
				"combining %q: module %q names module %q, which the combination does not list, add its %s under modules in %s",
				name, one.manifest.Module, e.Module, manifestPath, combineFile,
			)
		}
	}

	return nil
}

func combinedSchema(name string, loaded []loadedModule) (string, error) {
	first := loaded[0]

	for _, one := range loaded[1:] {
		if one.manifest.Schema == first.manifest.Schema {
			continue
		}

		return "", fmt.Errorf(
			"combining %q: module %q writes schema %q and module %q writes schema %q, one model holds one schema",
			name, first.manifest.Module, first.manifest.Schema, one.manifest.Module, one.manifest.Schema,
		)
	}

	return first.manifest.Schema, nil
}

func emitMod(schema string, loaded []loadedModule) string {
	var b strings.Builder

	b.WriteString(header + "\n")
	fmt.Fprintf(&b, "schema: %s\n", quote(schema))
	b.WriteString("contents:\n")

	for _, one := range loaded {
		fmt.Fprintf(&b, "  - %s\n", one.manifest.Model)
	}

	return b.String()
}

func mergeTests(name string, loaded []loadedModule) testDocument {
	document := testDocument{Name: name, ModelFile: modelFile}

	for _, one := range loaded {
		for _, c := range one.tests.Tests {
			c.Name = one.manifest.Module + ": " + c.Name
			document.Tests = append(document.Tests, c)
		}
	}

	return document
}
