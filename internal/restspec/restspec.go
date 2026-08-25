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

// Package restspec reads the operations out of an OpenAPI document's
// paths. Shared by every rest engine here, so the cells cannot drift on
// what an operation is: the same rules as forge-dev's builtin cell - the
// paths are the surface, and an operation with no operationId has nothing
// to dispatch to.
package restspec

import (
	"fmt"
	"sort"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/surface"
)

// Operation is one operation of the OpenAPI paths.
type Operation struct {
	// Method is the HTTP method, upper case; MethodLower is the same in
	// lower case, for routers whose route builders are named that way.
	Method      string
	MethodLower string
	// Path is the OpenAPI path with {param} segments, verbatim.
	Path string
	// ColonPath is the path with :param segments, for routers that spell
	// parameters that way.
	ColonPath string
	// GoName, CamelName and SnakeName are the operationId in each casing.
	GoName    string
	CamelName string
	SnakeName string
	// Description is the operation summary.
	Description string
}

var methods = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}

// Operations derives the operation list from the raw document, sorted by
// path then method so generation is deterministic.
func Operations(doc []byte) ([]Operation, error) {
	var parsed struct {
		Paths map[string]map[string]struct {
			OperationID string `json:"operationId"`
			Summary     string `json:"summary"`
		} `json:"paths"`
	}

	if err := yaml.Unmarshal(doc, &parsed); err != nil {
		return nil, fmt.Errorf("parsing the OpenAPI document: %w", err)
	}

	if len(parsed.Paths) == 0 {
		return nil, fmt.Errorf("reading the OpenAPI paths: a rest engine's surface is its paths, and there are none")
	}

	ops := []Operation{}

	for path, item := range parsed.Paths {
		for _, method := range methods {
			op, ok := item[method]
			if !ok {
				continue
			}

			if op.OperationID == "" {
				return nil, fmt.Errorf("reading %s %s: operationId is required, it names the handler", strings.ToUpper(method), path)
			}

			ops = append(ops, Operation{
				Method:      strings.ToUpper(method),
				MethodLower: method,
				Path:        path,
				ColonPath:   colonPath(path),
				GoName:      surface.Pascal(op.OperationID),
				CamelName:   surface.Camel(op.OperationID),
				SnakeName:   surface.Snake(op.OperationID),
				Description: op.Summary,
			})
		}
	}

	sort.Slice(ops, func(i, j int) bool {
		if ops[i].Path != ops[j].Path {
			return ops[i].Path < ops[j].Path
		}

		return ops[i].Method < ops[j].Method
	})

	return ops, nil
}

// colonPath turns /greet/{name} into /greet/:name.
func colonPath(path string) string {
	out := strings.ReplaceAll(path, "{", ":")

	return strings.ReplaceAll(out, "}", "")
}
