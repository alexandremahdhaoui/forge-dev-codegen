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
	"strings"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/storeddl"
)

func Script(store Store, rows []Row) string {
	var b strings.Builder

	b.WriteString(storeddl.Table(store.Snake, store.Key, store.Uniques))
	b.WriteString("\n")

	for _, row := range rows {
		b.WriteString("INSERT INTO " + storeddl.Identifier(store.Snake) + " (" + storeddl.Identifier(store.Key) + ", body) VALUES (" + storeddl.Literal(row.ID) + ", " + storeddl.Literal(row.Body) + ");\n")
		b.WriteString("INSERT INTO audit (at, table_name, key, op, before, after) VALUES (datetime('now'), " + storeddl.Literal(store.Snake) + ", " + storeddl.Literal(row.ID) + ", 'seed', NULL, " + storeddl.Literal(row.Body) + ");\n")
	}

	return b.String()
}
