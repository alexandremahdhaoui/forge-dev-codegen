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
	"fmt"
	"sort"
	"strconv"
	"strings"

	"sigs.k8s.io/yaml"
)

const specFile = "authz.yaml"

type shapeKind int

const (
	shapeMapping shapeKind = iota
	shapeList
	shapeText
	shapeTruth
	shapeValues
)

type field struct {
	key   string
	shape shape
}

type shape struct {
	kind    shapeKind
	says    string
	fields  []field
	element *shape
}

func text(says string) shape {
	return shape{kind: shapeText, says: says}
}

func truth(says string) shape {
	return shape{kind: shapeTruth, says: says}
}

func values(says string) shape {
	return shape{kind: shapeValues, says: says}
}

func mappingOf(says string, fields ...field) shape {
	return shape{kind: shapeMapping, says: says, fields: fields}
}

func listOf(says string, element shape) shape {
	return shape{kind: shapeList, says: says, element: &element}
}

func documentShape() shape {
	declaredType := mappingOf("a mapping naming a type and its relations",
		field{"name", text("a type name")},
		field{"relations", listOf("a list of relations", relationShape())},
	)

	return mappingOf("a mapping naming a module, its types and its vectors",
		field{"module", text("a module name")},
		field{"references", listOf("a list of references", mappingOf(
			"a mapping naming a module and the types this module reads",
			field{"module", text("a module name")},
			field{"types", listOf("a list of referenced types", mappingOf(
				"a mapping naming a type and the relations this module reads",
				field{"name", text("a type name")},
				field{"relations", listOf("a list of relation names", text("a relation name"))},
			))},
		))},
		field{"types", listOf("a list of types", declaredType)},
		field{"extends", listOf("a list of extensions", mappingOf(
			"a mapping naming a module and the types it extends",
			field{"module", text("a module name")},
			field{"types", listOf("a list of types", declaredType)},
		))},
		field{"conditions", listOf("a list of conditions", conditionShape())},
		field{"vectors", listOf("a list of cases", caseShape())},
	)
}

func relationShape() shape {
	return mappingOf("a mapping naming a relation and how it is granted",
		field{"name", text("a relation name")},
		field{"subjects", listOf("a list of subject types", text("a subject type"))},
		field{"expression", text("an expression")},
		field{"condition", text("a condition name")},
	)
}

func conditionShape() shape {
	return mappingOf("a mapping naming a condition, its parameters and its expression",
		field{"name", text("a condition name")},
		field{"parameters", listOf("a list of parameters", mappingOf(
			"a mapping naming a parameter and its type",
			field{"name", text("a parameter name")},
			field{"type", text("a parameter type")},
		))},
		field{"expression", text("a CEL expression")},
	)
}

func caseShape() shape {
	return mappingOf("a mapping naming a case, its tuples and its checks",
		field{"name", text("a case name")},
		field{"tuples", listOf("a list of tuples", mappingOf(
			"a mapping with a user, a relation and an object",
			field{"user", text("a user")},
			field{"relation", text("a relation name")},
			field{"object", text("an object")},
			field{"condition", mappingOf("a mapping naming a condition and its context",
				field{"name", text("a condition name")},
				field{"context", values("a mapping of parameter values")},
			)},
		))},
		field{"checks", listOf("a list of checks", mappingOf(
			"a mapping with a user, a relation, an object and an expectation",
			field{"user", text("a user")},
			field{"relation", text("a relation name")},
			field{"object", text("an object")},
			field{"expected", truth("true or false")},
			field{"context", values("a mapping of parameter values")},
		))},
	)
}

func checkDocumentShape(doc []byte) error {
	var document any

	if err := yaml.Unmarshal(doc, &document); err != nil {
		return fmt.Errorf("reading %s: %w", specFile, err)
	}

	return checkShapeOf(specFile, documentShape(), document)
}

func checkShapeOf(file string, s shape, document any) error {
	return shapeReader{file: file}.check("", s, document)
}

type shapeReader struct {
	file string
}

func (r shapeReader) check(path string, s shape, value any) error {
	if value == nil {
		return nil
	}

	switch s.kind {
	case shapeText:
		if _, ok := value.(string); !ok {
			return r.mismatch(path, s.says, value)
		}
	case shapeTruth:
		if _, ok := value.(bool); !ok {
			return r.mismatch(path, s.says, value)
		}
	case shapeValues:
		if _, ok := value.(map[string]any); !ok {
			return r.mismatch(path, s.says, value)
		}
	case shapeList:
		return r.checkList(path, s, value)
	case shapeMapping:
		return r.checkMapping(path, s, value)
	}

	return nil
}

func (r shapeReader) checkList(path string, s shape, value any) error {
	entries, ok := value.([]any)
	if !ok {
		return r.mismatch(path, s.says, value)
	}

	for i, entry := range entries {
		if err := r.check(path+entrySegment(i, entry), *s.element, entry); err != nil {
			return err
		}
	}

	return nil
}

func (r shapeReader) checkMapping(path string, s shape, value any) error {
	given, ok := value.(map[string]any)
	if !ok {
		return r.mismatch(path, s.says, value)
	}

	if err := r.checkKnownKeys(path, s, given); err != nil {
		return err
	}

	for _, f := range s.fields {
		entry, present := given[f.key]
		if !present {
			continue
		}

		if err := r.check(joinPath(path, f.key), f.shape, entry); err != nil {
			return err
		}
	}

	return nil
}

func (r shapeReader) checkKnownKeys(path string, s shape, given map[string]any) error {
	known := map[string]bool{}
	names := make([]string, 0, len(s.fields))

	for _, f := range s.fields {
		known[f.key] = true
		names = append(names, f.key)
	}

	keys := make([]string, 0, len(given))
	for key := range given {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	for _, key := range keys {
		if known[key] {
			continue
		}

		return fmt.Errorf(
			"reading %s %s: unknown key, write one of %s",
			r.file, joinPath(path, key), strings.Join(names, ", "),
		)
	}

	return nil
}

func (r shapeReader) mismatch(path, says string, value any) error {
	if path == "" {
		return fmt.Errorf("reading %s: expected %s, got %s", r.file, says, describeValue(value))
	}

	return fmt.Errorf("reading %s %s: expected %s, got %s", r.file, path, says, describeValue(value))
}

func describeValue(value any) string {
	switch value.(type) {
	case map[string]any:
		return "a mapping"
	case []any:
		return "a list"
	case string:
		return "a string"
	case bool:
		return "a boolean"
	case float64:
		return "a number"
	}

	return "nothing"
}

func entrySegment(index int, entry any) string {
	if fields, ok := entry.(map[string]any); ok {
		if name, ok := fields["name"].(string); ok && name != "" {
			return "." + name
		}
	}

	return "[" + strconv.Itoa(index) + "]"
}

func joinPath(path, key string) string {
	if path == "" {
		return key
	}

	return path + "." + key
}
