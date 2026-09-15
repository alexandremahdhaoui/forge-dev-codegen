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

package storeddl

import (
	"strings"

	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/rustname"
)

const auditTable = "CREATE TABLE IF NOT EXISTS audit (at TEXT NOT NULL, table_name TEXT NOT NULL, key TEXT NOT NULL, op TEXT NOT NULL, before TEXT, after TEXT);"

func Joint(properties []string) string {
	snakes := make([]string, 0, len(properties))

	for _, property := range properties {
		snakes = append(snakes, rustname.Snake(property))
	}

	return strings.Join(snakes, "_and_")
}

func Identifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func Literal(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func Index(snake string, properties []string) string {
	return snake + "_unique_by_" + Joint(properties)
}

func Table(snake, key string, uniques [][]string) string {
	var b strings.Builder

	b.WriteString("CREATE TABLE IF NOT EXISTS " + Identifier(snake) + " (" + Identifier(key) + " TEXT PRIMARY KEY, body TEXT NOT NULL);\n")

	for _, properties := range uniques {
		extracts := make([]string, 0, len(properties))

		for _, property := range properties {
			extracts = append(extracts, "json_extract(body, "+Literal("$."+property)+")")
		}

		b.WriteString("CREATE UNIQUE INDEX IF NOT EXISTS " + Identifier(Index(snake, properties)) + " ON " + Identifier(snake) + " (" + strings.Join(extracts, ", ") + ");\n")
	}

	b.WriteString(auditTable)

	return b.String()
}
