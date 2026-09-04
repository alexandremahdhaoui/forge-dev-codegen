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
)

type Database struct {
	Store  Store
	Rows   []Row
	Script string
}

func Plan(doc []byte, names []string, vectors []byte) ([]Database, error) {
	stores, err := Stores(doc, names)
	if err != nil {
		return nil, fmt.Errorf("planning the stores: %w", err)
	}

	rows := map[string][]Row{}

	if len(vectors) > 0 {
		rows, err = Seeds(vectors, stores)
		if err != nil {
			return nil, fmt.Errorf("planning the seed rows: %w", err)
		}
	}

	databases := make([]Database, 0, len(stores))

	for _, store := range stores {
		databases = append(databases, Database{
			Store:  store,
			Rows:   rows[store.Snake],
			Script: Script(store.Snake, rows[store.Snake]),
		})
	}

	return databases, nil
}
