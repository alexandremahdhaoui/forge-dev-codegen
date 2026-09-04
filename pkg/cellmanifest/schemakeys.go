package cellmanifest

import (
	"encoding/json"
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

type schemaNode struct {
	kind        string
	refusesNull bool
}

var schemaKeys = deriveSchemaKeys(schema)

func join(prefix, segment string) string {
	if prefix == "" {
		return segment
	}

	return prefix + "." + segment
}

func deriveSchemaKeys(document []byte) map[string]schemaNode {
	keys := map[string]schemaNode{}

	var root any
	if err := json.Unmarshal(document, &root); err != nil {
		return keys
	}

	collectSchemaKeys(root, "", definitionsOf(root), map[string]bool{}, keys)

	return keys
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
) {
	sources, entered := flattenSchema(node, defs, open)

	out[key] = schemaNodeOf(sources)

	for _, source := range sources {
		collectChildKeys(source, key, defs, open, out)
	}

	for _, name := range entered {
		delete(open, name)
	}
}

func collectChildKeys(
	source map[string]any,
	key string,
	defs map[string]any,
	open map[string]bool,
	out map[string]schemaNode,
) {
	if properties, isObject := source["properties"].(map[string]any); isObject {
		for name, child := range properties {
			collectSchemaKeys(child, join(key, name), defs, open, out)
		}
	}

	if items, present := source["items"]; present {
		collectSchemaKeys(items, key+itemSuffix, defs, open, out)
	}

	if additional, isObject := source["additionalProperties"].(map[string]any); isObject {
		collectSchemaKeys(additional, join(key, anyNameSegment), defs, open, out)
	}
}

func flattenSchema(node any, defs map[string]any, open map[string]bool) ([]map[string]any, []string) {
	object, isObject := node.(map[string]any)
	if !isObject {
		return nil, nil
	}

	sources := []map[string]any{object}
	entered := []string{}

	if ref, isString := object["$ref"].(string); isString {
		name := strings.TrimPrefix(ref, defsPrefix)

		if !open[name] {
			open[name] = true
			entered = append(entered, name)

			nested, nestedEntered := flattenSchema(defs[name], defs, open)
			sources = append(sources, nested...)
			entered = append(entered, nestedEntered...)
		}
	}

	for _, member := range listOf(object["allOf"]) {
		nested, nestedEntered := flattenSchema(member, defs, open)
		sources = append(sources, nested...)
		entered = append(entered, nestedEntered...)
	}

	return sources, entered
}

func schemaNodeOf(sources []map[string]any) schemaNode {
	types := map[string]bool{}

	for _, source := range sources {
		for _, name := range declaredTypes(source["type"]) {
			types[name] = true
		}
	}

	return schemaNode{kind: kindOf(types), refusesNull: len(types) > 0 && !types["null"]}
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
