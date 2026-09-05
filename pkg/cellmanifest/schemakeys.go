package cellmanifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	kindList       = "list"
	kindMap        = "map"
	kindScalar     = "scalar"
	itemSuffix     = "[]"
	anyNameSegment = "*"
	defsPrefix     = "#/$defs/"
)

var unmodelledKeywords = []string{
	"anyOf", "oneOf", "if", "then", "not",
	"patternProperties", "prefixItems", "contains", "dependentSchemas", "unevaluatedProperties",
}

type schemaNode struct {
	kind        string
	refusesNull bool
}

var schemaKeys = mustDeriveSchemaKeys(schema)

func join(prefix, segment string) string {
	if prefix == "" {
		return segment
	}

	return prefix + "." + segment
}

func mustDeriveSchemaKeys(document []byte) map[string]schemaNode {
	keys, err := deriveSchemaKeys(document)
	if err != nil {
		panic(err)
	}

	return keys
}

func deriveSchemaKeys(document []byte) (map[string]schemaNode, error) {
	var root any
	if err := json.Unmarshal(document, &root); err != nil {
		return nil, fmt.Errorf("parsing the cell schema: %w", err)
	}

	keys := map[string]schemaNode{}

	if err := collectSchemaKeys(root, "", definitionsOf(root), map[string]bool{}, keys); err != nil {
		return nil, fmt.Errorf("deriving the cell schema keys: %w", err)
	}

	if onlyTheRoot(keys) {
		return nil, errors.New("deriving the cell schema keys: the schema declares no key under the root")
	}

	return keys, nil
}

func onlyTheRoot(keys map[string]schemaNode) bool {
	_, hasRoot := keys[""]

	return hasRoot && len(keys) == 1
}

func definitionsOf(root any) map[string]any {
	object, isObject := root.(map[string]any)
	if !isObject {
		return map[string]any{}
	}

	defs, isObject := object["$defs"].(map[string]any)
	if !isObject {
		return map[string]any{}
	}

	return defs
}

func collectSchemaKeys(
	node any,
	key string,
	defs map[string]any,
	open map[string]bool,
	out map[string]schemaNode,
) error {
	sources, entered, err := flattenSchema(node, key, defs, open)
	if err != nil {
		return err
	}

	out[key] = schemaNodeOf(sources)

	for _, source := range sources {
		if err := collectChildKeys(source, key, defs, open, out); err != nil {
			return err
		}
	}

	for _, name := range entered {
		delete(open, name)
	}

	return nil
}

func collectChildKeys(
	source map[string]any,
	key string,
	defs map[string]any,
	open map[string]bool,
	out map[string]schemaNode,
) error {
	if properties, isObject := source["properties"].(map[string]any); isObject {
		for name, child := range properties {
			if err := collectSchemaKeys(child, join(key, name), defs, open, out); err != nil {
				return err
			}
		}
	}

	if items, present := source["items"]; present {
		if err := collectSchemaKeys(items, key+itemSuffix, defs, open, out); err != nil {
			return err
		}
	}

	if additional, isObject := source["additionalProperties"].(map[string]any); isObject {
		if err := collectSchemaKeys(additional, join(key, anyNameSegment), defs, open, out); err != nil {
			return err
		}
	}

	return nil
}

func flattenSchema(
	node any,
	key string,
	defs map[string]any,
	open map[string]bool,
) ([]map[string]any, []string, error) {
	object, isObject := node.(map[string]any)
	if !isObject {
		return nil, nil, nil
	}

	if err := refuseUnmodelledKeywords(object, key); err != nil {
		return nil, nil, err
	}

	sources := []map[string]any{object}
	entered := []string{}

	if ref, isString := object["$ref"].(string); isString {
		name := strings.TrimPrefix(ref, defsPrefix)

		if !open[name] {
			open[name] = true
			entered = append(entered, name)

			nested, nestedEntered, err := flattenSchema(defs[name], key, defs, open)
			if err != nil {
				return nil, nil, err
			}

			sources = append(sources, nested...)
			entered = append(entered, nestedEntered...)
		}
	}

	for _, member := range listOf(object["allOf"]) {
		nested, nestedEntered, err := flattenSchema(member, key, defs, open)
		if err != nil {
			return nil, nil, err
		}

		sources = append(sources, nested...)
		entered = append(entered, nestedEntered...)
	}

	return sources, entered, nil
}

func refuseUnmodelledKeywords(object map[string]any, key string) error {
	for _, keyword := range unmodelledKeywords {
		if _, present := object[keyword]; present {
			return fmt.Errorf("schema key %q carries %q which the key walk does not model", key, keyword)
		}
	}

	return nil
}

func schemaNodeOf(sources []map[string]any) schemaNode {
	var types map[string]bool

	for _, source := range sources {
		declared := declaredTypes(source["type"])
		if len(declared) == 0 {
			continue
		}

		types = intersectTypes(types, declared)
	}

	return schemaNode{kind: kindOf(types), refusesNull: len(types) > 0 && !types["null"]}
}

func intersectTypes(known map[string]bool, declared []string) map[string]bool {
	next := map[string]bool{}

	for _, name := range declared {
		if known == nil || known[name] {
			next[name] = true
		}
	}

	return next
}

func declaredTypes(value any) []string {
	if name, isString := value.(string); isString {
		return []string{name}
	}

	names := []string{}

	for _, item := range listOf(value) {
		if name, isString := item.(string); isString {
			names = append(names, name)
		}
	}

	return names
}

func kindOf(types map[string]bool) string {
	if types["array"] {
		return kindList
	}

	if types["object"] {
		return kindMap
	}

	return kindScalar
}

func listOf(value any) []any {
	items, isList := value.([]any)
	if !isList {
		return nil
	}

	return items
}
