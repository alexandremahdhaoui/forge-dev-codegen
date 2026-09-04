package cellmanifest

import (
	"sort"
	"strings"
	"testing"
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
		key         string
		refusesNull bool
	}{
		{key: "provides.drivers[].config.*.type", refusesNull: true},
		{key: "provides.drivers[].config.*.required", refusesNull: true},
		{key: "provides.drivers[].config.*.description", refusesNull: true},
		{key: "provides.drivers[].config.*.default", refusesNull: false},
	}

	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
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
