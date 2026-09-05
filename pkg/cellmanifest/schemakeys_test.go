package cellmanifest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

func TestTheDerivedKeySetHoldsEveryObjectAndArrayKeyOfTheSchema(t *testing.T) {
	t.Parallel()

	want := map[string]string{
		"":                             kindMap,
		"provides":                     kindMap,
		"provides.drivers":             kindList,
		"provides.drivers[]":           kindMap,
		"provides.drivers[].requires":  kindList,
		"provides.drivers[].config":    kindMap,
		"provides.drivers[].config.*":  kindMap,
		"provides.adapters":            kindList,
		"provides.adapters[]":          kindMap,
		"provides.adapters[].config":   kindMap,
		"provides.adapters[].config.*": kindMap,
		"provides.controllers":         kindList,
		"provides.controllers[]":       kindMap,
		"provides.controllers[].ports": kindList,
		"provides.ports":               kindList,
		"provides.ports[]":             kindMap,
		"requires":                     kindMap,
		"requires.ports":               kindList,
	}

	got := map[string]string{}

	for key, node := range schemaKeys {
		if node.kind == kindScalar {
			continue
		}

		got[key] = node.kind
	}

	for key, kind := range want {
		if got[key] != kind {
			t.Fatalf("got kind %q for key %q, want %q", got[key], key, kind)
		}

		if !schemaKeys[key].refusesNull {
			t.Fatalf("got key %q allowing null, want it refusing null", key)
		}
	}

	for key := range got {
		if _, listed := want[key]; !listed {
			t.Fatalf("got key %q which the test does not list", key)
		}
	}
}

func TestTheDerivedKeySetHoldsEveryScalarKeyOfAConfigField(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		key         string
		refusesNull bool
	}{
		{name: "the type of a config field refuses null", key: "provides.drivers[].config.*.type", refusesNull: true},
		{name: "the required flag of a config field refuses null", key: "provides.drivers[].config.*.required", refusesNull: true},
		{name: "the description of a config field refuses null", key: "provides.drivers[].config.*.description", refusesNull: true},
		{name: "the default of a config field allows null", key: "provides.drivers[].config.*.default", refusesNull: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			node, known := schemaKeys[tc.key]
			if !known {
				t.Fatalf("got no key %q in the derived set", tc.key)
			}

			if node.refusesNull != tc.refusesNull {
				t.Fatalf("got refusesNull %v for key %q, want %v", node.refusesNull, tc.key, tc.refusesNull)
			}
		})
	}
}

func TestEveryDerivedKeyStartsAtTheRoot(t *testing.T) {
	t.Parallel()

	keys := make([]string, 0, len(schemaKeys))
	for key := range schemaKeys {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	for _, key := range keys {
		if key == "" {
			continue
		}

		parent := parentKey(key)
		if _, known := schemaKeys[parent]; !known {
			t.Fatalf("got key %q whose parent %q is not in the derived set", key, parent)
		}
	}
}

func parentKey(key string) string {
	if strings.HasSuffix(key, itemSuffix) {
		return strings.TrimSuffix(key, itemSuffix)
	}

	if cut := strings.LastIndex(key, "."); cut >= 0 {
		return key[:cut]
	}

	return ""
}

func TestTheDerivedKeySetEqualsAnIndependentWalkOfTheSchemaScalarsIncluded(t *testing.T) {
	t.Parallel()

	var root jsonschema.Schema
	if err := json.Unmarshal(schema, &root); err != nil {
		t.Fatalf("unmarshalling the embedded schema returned %v", err)
	}

	if _, err := root.Resolve(nil); err != nil {
		t.Fatalf("resolving the embedded schema returned %v", err)
	}

	want := map[string]schemaNode{}
	walkTypedSchema(&root, &root, "", map[*jsonschema.Schema]bool{}, want)

	for key, node := range want {
		got, known := schemaKeys[key]
		if !known {
			t.Fatalf("the independent walk found key %q which the derived set lacks", key)
		}

		if got != node {
			t.Fatalf("got %+v for key %q, the independent walk says %+v", got, key, node)
		}
	}

	for key := range schemaKeys {
		if _, listed := want[key]; !listed {
			t.Fatalf("the derived set holds key %q which the independent walk never reached", key)
		}
	}
}

func walkTypedSchema(
	root, s *jsonschema.Schema,
	key string,
	open map[*jsonschema.Schema]bool,
	out map[string]schemaNode,
) {
	members := typedConjuncts(root, s, open)

	out[key] = typedNodeOf(members)

	for _, member := range members {
		for name, child := range member.Properties {
			walkTypedSchema(root, child, join(key, name), open, out)
		}

		if member.Items != nil {
			walkTypedSchema(root, member.Items, key+itemSuffix, open, out)
		}

		if member.AdditionalProperties != nil && !isFalseSchema(member.AdditionalProperties) {
			walkTypedSchema(root, member.AdditionalProperties, join(key, anyNameSegment), open, out)
		}
	}

	for _, member := range members {
		delete(open, member)
	}
}

func typedConjuncts(root, s *jsonschema.Schema, open map[*jsonschema.Schema]bool) []*jsonschema.Schema {
	if s == nil || open[s] {
		return nil
	}

	open[s] = true

	members := []*jsonschema.Schema{s}

	if s.Ref != "" {
		target := root.Defs[strings.TrimPrefix(s.Ref, defsPrefix)]
		members = append(members, typedConjuncts(root, target, open)...)
	}

	for _, member := range s.AllOf {
		members = append(members, typedConjuncts(root, member, open)...)
	}

	return members
}

func typedNodeOf(members []*jsonschema.Schema) schemaNode {
	var types map[string]bool

	for _, member := range members {
		declared := member.Types
		if member.Type != "" {
			declared = []string{member.Type}
		}

		if len(declared) == 0 {
			continue
		}

		next := map[string]bool{}

		for _, name := range declared {
			if types == nil || types[name] {
				next[name] = true
			}
		}

		types = next
	}

	kind := kindScalar

	switch {
	case types["array"]:
		kind = kindList
	case types["object"]:
		kind = kindMap
	}

	return schemaNode{kind: kind, refusesNull: len(types) > 0 && !types["null"]}
}

func isFalseSchema(s *jsonschema.Schema) bool {
	encoded, err := json.Marshal(s)
	if err != nil {
		return false
	}

	return bytes.Equal(encoded, []byte("false"))
}

func TestASchemaCarryingAKeywordTheWalkDoesNotModelIsRefused(t *testing.T) {
	t.Parallel()

	for _, keyword := range unmodelledKeywords {
		t.Run("a property carrying "+keyword+" is refused naming the key", func(t *testing.T) {
			t.Parallel()

			doc := fmt.Sprintf(`{"type":"object","properties":{"cell":{%q:[]}}}`, keyword)

			_, err := deriveSchemaKeys([]byte(doc))
			if err == nil {
				t.Fatalf("got no error for a schema carrying %s", keyword)
			}

			for _, want := range []string{`"cell"`, fmt.Sprintf("%q", keyword)} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("got error %q, want it naming %s", err, want)
				}
			}
		})
	}

	t.Run("a definition a ref reaches that carries not is refused naming the key", func(t *testing.T) {
		t.Parallel()

		doc := `{"$defs":{"x":{"not":{}}},"type":"object","properties":{"cell":{"$ref":"#/$defs/x"}}}`

		_, err := deriveSchemaKeys([]byte(doc))
		if err == nil || !strings.Contains(err.Error(), `"cell"`) || !strings.Contains(err.Error(), `"not"`) {
			t.Fatalf("got error %v, want it naming the key and the keyword", err)
		}
	})
}

func TestASchemaDeclaringNoKeyUnderTheRootIsRefused(t *testing.T) {
	t.Parallel()

	_, err := deriveSchemaKeys([]byte(`{"type":"object"}`))
	if err == nil || !strings.Contains(err.Error(), "declares no key under the root") {
		t.Fatalf("got error %v, want the empty key set refused", err)
	}
}

func TestASchemaThatIsNotJsonIsRefused(t *testing.T) {
	t.Parallel()

	_, err := deriveSchemaKeys([]byte(`{`))
	if err == nil || !strings.Contains(err.Error(), "parsing the cell schema") {
		t.Fatalf("got error %v, want the parse refused", err)
	}
}

func TestARefusedSchemaPanicsAtInit(t *testing.T) {
	t.Parallel()

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatalf("got no panic for a schema with no key under the root")
		}

		if !strings.Contains(fmt.Sprint(recovered), "declares no key under the root") {
			t.Fatalf("got panic %v, want it naming the empty key set", recovered)
		}
	}()

	mustDeriveSchemaKeys([]byte(`{"type":"object"}`))
}

func TestAnAllOfIntersectsTheDeclaredTypesOfItsMembers(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		doc  string
		key  string
		want schemaNode
	}{
		{
			name: "a member allowing null and a member refusing it leave the key refusing null",
			doc:  `{"type":"object","properties":{"name":{"allOf":[{"type":["string","null"]},{"type":"string"}]}}}`,
			key:  "name",
			want: schemaNode{kind: kindScalar, refusesNull: true},
		},
		{
			name: "a ref to an array and a member with no type leave the key a list",
			doc:  `{"$defs":{"list":{"type":"array"}},"type":"object","properties":{"names":{"allOf":[{"$ref":"#/$defs/list"},{"minItems":1}]}}}`,
			key:  "names",
			want: schemaNode{kind: kindList, refusesNull: true},
		},
		{
			name: "two members sharing no type leave the key with no type at all",
			doc:  `{"type":"object","properties":{"odd":{"allOf":[{"type":"array"},{"type":"object"}]}}}`,
			key:  "odd",
			want: schemaNode{kind: kindScalar, refusesNull: false},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			keys, err := deriveSchemaKeys([]byte(tc.doc))
			if err != nil {
				t.Fatalf("deriving returned %v", err)
			}

			if keys[tc.key] != tc.want {
				t.Fatalf("got %+v for key %q, want %+v", keys[tc.key], tc.key, tc.want)
			}
		})
	}
}
