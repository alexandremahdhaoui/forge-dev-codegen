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

package testenvsqlite

import (
	"encoding/json"
	"fmt"
	"regexp"

	"sigs.k8s.io/yaml"

	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/rustname"
)

var tableName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

type Store struct {
	Name     string
	Snake    string
	Upper    string
	Key      string
	Required []string
}

type document struct {
	Components struct {
		Schemas map[string]struct {
			Store    json.RawMessage `json:"x-store"`
			Required []string        `json:"required"`
		} `json:"schemas"`
	} `json:"components"`
}

func Stores(doc []byte, names []string) ([]Store, error) {
	var parsed document
	if err := yaml.Unmarshal(doc, &parsed); err != nil {
		return nil, fmt.Errorf("reading the OpenAPI document: %w", err)
	}

	stores := make([]Store, 0, len(names))

	for _, name := range names {
		schema, ok := parsed.Components.Schemas[name]
		if !ok {
			return nil, fmt.Errorf("finding store %q: components.schemas has no such schema", name)
		}

		key, err := storeKey(name, schema.Store)
		if err != nil {
			return nil, err
		}

		snake := rustname.Snake(name)
		if !tableName.MatchString(snake) {
			return nil, fmt.Errorf("naming the table of store %q: %q is not a table name matching %s", name, snake, tableName)
		}

		if !tableName.MatchString(key) {
			return nil, fmt.Errorf("naming the key column of store %q: %q is not a column name matching %s", name, key, tableName)
		}

		stores = append(stores, Store{
			Name:     name,
			Snake:    snake,
			Upper:    rustname.Upper(name),
			Key:      key,
			Required: schema.Required,
		})
	}

	return stores, nil
}

func storeKey(name string, raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", fmt.Errorf("finding store %q: the schema is not marked x-store", name)
	}

	var flag bool
	if err := json.Unmarshal(raw, &flag); err == nil {
		return "", fmt.Errorf("finding store %q: x-store is a boolean, it is an object naming key, lookups and adapters", name)
	}

	var declared struct {
		Key string `json:"key"`
	}

	if err := json.Unmarshal(raw, &declared); err != nil {
		return "", fmt.Errorf("finding store %q: x-store is an object naming key, lookups and adapters: %w", name, err)
	}

	if declared.Key == "" {
		return "", fmt.Errorf("finding store %q: x-store names no key, key names the property every row is stored under", name)
	}

	return declared.Key, nil
}
