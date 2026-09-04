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
)

const auditDDL = "CREATE TABLE IF NOT EXISTS audit (at TEXT NOT NULL, table_name TEXT NOT NULL, key TEXT NOT NULL, op TEXT NOT NULL, before TEXT, after TEXT);"

func DDL(snake string) string {
	return "CREATE TABLE IF NOT EXISTS " + identifier(snake) + " (id TEXT PRIMARY KEY, body TEXT NOT NULL);\n" + auditDDL
}

func Script(snake string, rows []Row) string {
	var b strings.Builder

	b.WriteString(DDL(snake))
	b.WriteString("\n")

	for _, row := range rows {
		b.WriteString("INSERT OR REPLACE INTO " + identifier(snake) + " (id, body) VALUES (" + quote(row.ID) + ", " + quote(row.Body) + ");\n")
		b.WriteString("INSERT INTO audit (at, table_name, key, op, before, after) VALUES (datetime('now'), " + quote(snake) + ", " + quote(row.ID) + ", 'seed', NULL, " + quote(row.Body) + ");\n")
	}

	return b.String()
}

func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func identifier(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
