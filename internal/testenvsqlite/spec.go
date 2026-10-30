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
	"fmt"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/restrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/rustname"
)

type Store struct {
	Name     string
	Snake    string
	Upper    string
	Key      string
	Order    string
	Uniques  [][]string
	Pages    [][]string
	Required []string
}

func Stores(doc []byte, names []string) ([]Store, error) {
	spec, err := restrust.Parse(doc)
	if err != nil {
		return nil, fmt.Errorf("reading the OpenAPI document: %w", err)
	}

	types := map[string]restrust.TypeDef{}
	for _, t := range spec.Types {
		types[t.Name] = t
	}

	stores := make([]Store, 0, len(names))

	for _, name := range names {
		t, ok := types[name]
		if !ok {
			return nil, fmt.Errorf("finding store %q: components.schemas has no such schema", name)
		}

		if t.Store == nil {
			return nil, fmt.Errorf("finding store %q: the schema is not marked x-store", name)
		}

		stores = append(stores, storeOfType(t))
	}

	return stores, nil
}

func storeOfType(t restrust.TypeDef) Store {
	store := Store{
		Name:  t.Name,
		Snake: t.Snake,
		Upper: rustname.Upper(t.Name),
		Key:   t.Store.Key,
	}

	if t.Store.Order != nil {
		store.Order = t.Store.Order.Name
	}

	for _, l := range t.Store.Lookups {
		if l.Answers == restrust.LookupOne {
			store.Uniques = append(store.Uniques, l.By)
		}

		if l.Answers == restrust.LookupPage {
			store.Pages = append(store.Pages, l.By)
		}
	}

	for _, f := range t.Fields {
		if !f.Optional {
			store.Required = append(store.Required, f.Name)
		}
	}

	return store
}
