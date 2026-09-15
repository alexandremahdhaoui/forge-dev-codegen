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

package storeddl_test

import (
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/storeddl"
)

func TestATableWithNoUniqueLookupIsTheKeyedTableAndTheAuditTable(t *testing.T) {
	want := "CREATE TABLE IF NOT EXISTS \"greeting\" (\"id\" TEXT PRIMARY KEY, body TEXT NOT NULL);\n" +
		"CREATE TABLE IF NOT EXISTS audit (at TEXT NOT NULL, table_name TEXT NOT NULL, key TEXT NOT NULL, op TEXT NOT NULL, before TEXT, after TEXT);"

	if got := storeddl.Table("greeting", "id", nil); got != want {
		t.Errorf("table:\n got %q\nwant %q", got, want)
	}
}

func TestEveryUniqueLookupBecomesOneIndexNamedAfterTheJointItReads(t *testing.T) {
	got := storeddl.Table("profile", "subject", [][]string{{"alias", "tag"}, {"handle"}})

	for _, want := range []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS "profile_unique_by_alias_and_tag" ON "profile" (json_extract(body, '$.alias'), json_extract(body, '$.tag'));`,
		`CREATE UNIQUE INDEX IF NOT EXISTS "profile_unique_by_handle" ON "profile" (json_extract(body, '$.handle'));`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("table:\n%s\nwants %s", got, want)
		}
	}
}

func TestAKeyThatIsASqlReservedWordIsQuotedSoTheDdlStaysValid(t *testing.T) {
	got := storeddl.Table("order", "group", nil)

	if !strings.Contains(got, `CREATE TABLE IF NOT EXISTS "order" ("group" TEXT PRIMARY KEY, body TEXT NOT NULL);`) {
		t.Errorf("table:\n%s", got)
	}
}

func TestAnIdentifierCarryingADoubleQuoteDoublesItRatherThanClosingTheName(t *testing.T) {
	if got := storeddl.Identifier(`a"b`); got != `"a""b"` {
		t.Errorf("identifier: %q", got)
	}
}

func TestALiteralCarryingASingleQuoteDoublesItRatherThanClosingTheValue(t *testing.T) {
	if got := storeddl.Literal("a'b"); got != "'a''b'" {
		t.Errorf("literal: %q", got)
	}
}

func TestTheJointAndTheIndexNameSpellEveryPropertyInSnakeCase(t *testing.T) {
	if got := storeddl.Joint([]string{"givenName", "tag"}); got != "given_name_and_tag" {
		t.Errorf("joint: %q", got)
	}

	if got := storeddl.Index("profile", []string{"givenName", "tag"}); got != "profile_unique_by_given_name_and_tag" {
		t.Errorf("index: %q", got)
	}
}
