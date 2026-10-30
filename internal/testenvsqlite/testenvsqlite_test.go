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
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/restrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/storeddl"
)

const pingPath = `
paths:
  /ping:
    get:
      operationId: ping
      x-controller: ping
      responses:
        "204":
          description: Nothing
`

const helloSpec = pingPath + `
components:
  schemas:
    CreateGreetingRequest:
      type: object
      required: [name]
      properties:
        name:
          type: string
    Greeting:
      type: object
      x-store:
        key: id
        lookups: []
        adapters: [sqlite]
      required: [id, name, count]
      properties:
        id:
          type: string
        name:
          type: string
        count:
          type: integer
    PlayerScore:
      type: object
      x-store:
        key: id
        lookups: []
        adapters: [sqlite]
      required: [id, score]
      properties:
        id:
          type: string
        score:
          type: integer
`

const helloVectors = `{
  "cases": [
    {
      "case": "create_valid_name",
      "operation": "createGreeting",
      "controllerReply": {"id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8", "name": "Songe", "count": 0}
    },
    {
      "case": "create_empty_name_refused",
      "operation": "createGreeting",
      "expectedStatus": 422
    },
    {
      "case": "get_existing_id",
      "operation": "getGreeting",
      "controllerReply": {"id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8", "name": "Songe", "count": 1}
    },
    {
      "case": "create_score",
      "operation": "createPlayerScore",
      "controllerReply": {"id": "s1", "score": 7}
    }
  ]
}`

func TestStoresFindsEveryNamedXStoreSchemaWithItsSnakeAndUpperNames(t *testing.T) {
	stores, err := Stores([]byte(helloSpec), []string{"Greeting", "PlayerScore"})
	if err != nil {
		t.Fatal(err)
	}

	if len(stores) != 2 {
		t.Fatalf("expected 2 stores, got %d", len(stores))
	}

	if stores[0].Snake != "greeting" || stores[0].Upper != "GREETING" {
		t.Errorf("Greeting names: %+v", stores[0])
	}

	if stores[1].Snake != "player_score" || stores[1].Upper != "PLAYER_SCORE" {
		t.Errorf("PlayerScore names: %+v", stores[1])
	}

	if strings.Join(stores[0].Required, ",") != "count,id,name" {
		t.Errorf("Greeting required: %v", stores[0].Required)
	}
}

func TestStoresRefusesAnUnknownSchemaAndASchemaWithoutXStore(t *testing.T) {
	if _, err := Stores([]byte(helloSpec), []string{"Missing"}); err == nil {
		t.Error("an unknown schema must be refused")
	}

	if _, err := Stores([]byte(helloSpec), []string{"CreateGreetingRequest"}); err == nil {
		t.Error("a schema without x-store must be refused")
	}
}

func TestStoresRefusesADocumentThatIsNotYaml(t *testing.T) {
	if _, err := Stores([]byte(":\n  - :\n :"), []string{"Greeting"}); err == nil {
		t.Error("a broken document must be refused")
	}
}

func TestStoresRefusesANameWhoseSnakeFormIsNotAPlainTableName(t *testing.T) {
	for name, want := range map[string]string{
		"Bad; DROP": `reading the OpenAPI document: reading schema "Bad; DROP": its snake form "bad;_drop" is not a name Rust or SQL can spell, use letters, digits and underscores and start with a letter`,
		"9Lives":    `reading the OpenAPI document: reading schema "9Lives": its snake form "9_lives" is not a name Rust or SQL can spell, use letters, digits and underscores and start with a letter`,
	} {
		doc := pingPath + "components:\n  schemas:\n    \"" + name + "\":\n      type: object\n      x-store:\n        key: id\n        lookups: []\n        adapters: [sqlite]\n      required: [id]\n      properties:\n        id:\n          type: string\n"

		_, err := Stores([]byte(doc), []string{name})
		if err == nil || err.Error() != want {
			t.Errorf("%q was not refused as a table name:\n got %v\nwant %s", name, err, want)
		}
	}
}

func TestALookupThatAnswersOneBecomesAUniqueIndexOverEveryFieldItReads(t *testing.T) {
	doc := pingPath + "components:\n  schemas:\n    Profile:\n      type: object\n      x-store:\n        key: subject\n        lookups:\n          - { by: [alias, tag], answers: one }\n          - { by: [alias], answers: page }\n        adapters: [sqlite]\n      required: [subject, alias, tag]\n      properties:\n        subject:\n          type: string\n        alias:\n          type: string\n        tag:\n          type: string\n"

	stores, err := Stores([]byte(doc), []string{"Profile"})
	if err != nil {
		t.Fatal(err)
	}

	want := `CREATE UNIQUE INDEX IF NOT EXISTS "profile_unique_by_alias_and_tag" ON "profile" (json_extract(body, '$.alias'), json_extract(body, '$.tag'));`

	got := storeddl.Table(storeddl.Shape{Snake: stores[0].Snake, Key: stores[0].Key, Uniques: stores[0].Uniques})
	if !strings.Contains(got, want) {
		t.Errorf("table:\n%s", got)
	}

	if strings.Contains(got, `profile_unique_by_alias" ON`) {
		t.Errorf("a lookup that answers page must not become a unique index:\n%s", got)
	}
}

func TestTheKeyColumnIsTheDeclaredKeyAndNeverAssumedToBeId(t *testing.T) {
	doc := pingPath + "components:\n  schemas:\n    Account:\n      type: object\n      x-store:\n        key: subject\n        lookups: []\n        adapters: [sqlite]\n      required: [subject]\n      properties:\n        subject:\n          type: string\n"

	stores, err := Stores([]byte(doc), []string{"Account"})
	if err != nil {
		t.Fatal(err)
	}

	if stores[0].Key != "subject" {
		t.Fatalf("key: %q", stores[0].Key)
	}

	table := storeddl.Table(storeddl.Shape{Snake: stores[0].Snake, Key: stores[0].Key})
	if !strings.Contains(table, `"subject" TEXT PRIMARY KEY`) {
		t.Errorf("table: %s", table)
	}
}

func TestACamelCaseKeyIsAQuotedColumnTheWriterSeeds(t *testing.T) {
	doc := pingPath + "components:\n  schemas:\n    Consent:\n      type: object\n      x-store:\n        key: accountId\n        lookups: []\n        adapters: [sqlite]\n      required: [accountId]\n      properties:\n        accountId:\n          type: string\n"

	databases, err := Plan([]byte(doc), []string{"Consent"}, []byte(`{"cases":[{"case":"c","operation":"createConsent","controllerReply":{"accountId":"a1"}}]}`))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(databases[0].Script, `"accountId" TEXT PRIMARY KEY`) || !strings.Contains(databases[0].Script, `INSERT INTO "consent" ("accountId", body) VALUES ('a1',`) {
		t.Fatalf("the key is not a quoted column:\n%s", databases[0].Script)
	}

	writer, err := DetectWriter()
	if err != nil {
		t.Skip(err)
	}

	if err := writer.Write(context.Background(), filepath.Join(t.TempDir(), "consent.db"), databases[0].Script); err != nil {
		t.Fatal(err)
	}
}

func TestTheBooleanFormOfXStoreIsRefusedNamingTheSchema(t *testing.T) {
	doc := pingPath + "components:\n  schemas:\n    Greeting:\n      type: object\n      x-store: true\n      required: [id]\n      properties:\n        id:\n          type: string\n"

	_, err := Stores([]byte(doc), []string{"Greeting"})
	if err == nil || err.Error() != `reading the OpenAPI document: reading schema "Greeting": x-store is a boolean, it is an object naming key, lookups, order and adapters` {
		t.Fatalf("the boolean form was not refused by name: %v", err)
	}
}

func TestAStoreThatNamesNoKeyIsRefusedByName(t *testing.T) {
	doc := pingPath + "components:\n  schemas:\n    Greeting:\n      type: object\n      x-store:\n        lookups: []\n        adapters: [sqlite]\n      required: [id]\n      properties:\n        id:\n          type: string\n"

	_, err := Stores([]byte(doc), []string{"Greeting"})
	if err == nil || err.Error() != `reading the OpenAPI document: reading schema "Greeting": x-store names no key, key names the required string property every row is stored under` {
		t.Fatalf("a store with no key was not refused by name: %v", err)
	}
}

const chatLineSpec = pingPath + `
components:
  schemas:
    ChatLine:
      type: object
      x-store:
        key: id
        lookups:
          - { by: [chatId], answers: page }
          - { by: [accountId, chatId], answers: page }
        order: postedAt
        adapters: [sqlite, memory]
      required: [id, chatId, accountId, postedAt]
      properties:
        id:
          type: string
        chatId:
          type: string
        accountId:
          type: string
        postedAt:
          type: string
          format: date-time
`

func TestTestenvSqliteSeedsTheSameIndexesTheSqliteAdapterCreates(t *testing.T) {
	files, err := restrust.Generate([]byte(chatLineSpec), restrust.Options{Service: "songe-social"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	schema := ""

	for _, f := range files {
		if f.Path != "adapter/zz_generated_chat_line_sqlite.rs" {
			continue
		}

		_, rest, found := strings.Cut(f.Content, "const SCHEMA: &str = r#\"")
		if !found {
			t.Fatalf("the sqlite adapter holds no SCHEMA:\n%s", f.Content)
		}

		schema, _, _ = strings.Cut(rest, "\"#;")
	}

	if !strings.Contains(schema, `CREATE INDEX IF NOT EXISTS "chat_line_page_by_chat_id"`) {
		t.Fatalf("the sqlite adapter creates no page index, so this proof compares nothing:\n%s", schema)
	}

	stores, err := Stores([]byte(chatLineSpec), []string{"ChatLine"})
	if err != nil {
		t.Fatal(err)
	}

	if got := Script(stores[0], nil); got != schema+"\n" {
		t.Errorf("testenv-sqlite seeds a schema the sqlite adapter does not create:\n got %q\nwant %q", got, schema+"\n")
	}
}

func TestSeedsInsertsOnlyCreateVectorsThatCarryAControllerReply(t *testing.T) {
	stores, err := Stores([]byte(helloSpec), []string{"Greeting", "PlayerScore"})
	if err != nil {
		t.Fatal(err)
	}

	rows, err := Seeds([]byte(helloVectors), stores)
	if err != nil {
		t.Fatal(err)
	}

	if len(rows["greeting"]) != 1 {
		t.Fatalf("greeting rows: %+v", rows["greeting"])
	}

	if rows["greeting"][0].ID != "6ba7b810-9dad-11d1-80b4-00c04fd430c8" {
		t.Errorf("greeting id: %q", rows["greeting"][0].ID)
	}

	if rows["greeting"][0].Body != `{"id":"6ba7b810-9dad-11d1-80b4-00c04fd430c8","name":"Songe","count":0}` {
		t.Errorf("greeting body: %q", rows["greeting"][0].Body)
	}

	if len(rows["player_score"]) != 1 || rows["player_score"][0].ID != "s1" {
		t.Errorf("player_score rows: %+v", rows["player_score"])
	}
}

func TestSeedsFallsBackToTheStoreWhoseRequiredKeysTheReplyCovers(t *testing.T) {
	stores, err := Stores([]byte(helloSpec), []string{"PlayerScore"})
	if err != nil {
		t.Fatal(err)
	}

	vectors := `{"cases":[{"case":"c","operation":"createThing","controllerReply":{"id":"x","score":1}}]}`

	rows, err := Seeds([]byte(vectors), stores)
	if err != nil {
		t.Fatal(err)
	}

	if len(rows["player_score"]) != 1 {
		t.Errorf("player_score rows: %+v", rows)
	}
}

func TestSeedsSkipsAnErrorCaseWhoseControllerReplyIsAbsentOrNull(t *testing.T) {
	stores, err := Stores([]byte(helloSpec), []string{"Greeting"})
	if err != nil {
		t.Fatal(err)
	}

	vectors := `{"cases":[
	  {"case":"absent","operation":"createGreeting"},
	  {"case":"null","operation":"createGreeting","controllerReply":null}
	]}`

	rows, err := Seeds([]byte(vectors), stores)
	if err != nil {
		t.Fatal(err)
	}

	if len(rows) != 0 {
		t.Errorf("rows: %+v", rows)
	}
}

func TestPlanRefusesACreateCaseAnsweringARowOfNoDeclaredStoreNamingTheCaseTheOperationAndTheStores(t *testing.T) {
	for _, reply := range []string{`{"name":"no id"}`, `{"id":"x","other":true}`} {
		vectors := `{"cases":[{"case":"stray","operation":"createOther","controllerReply":` + reply + `}]}`

		_, err := Plan([]byte(helloSpec), []string{"Greeting", "PlayerScore"}, []byte(vectors))
		if err == nil {
			t.Fatalf("a create case answering %s matches no store and must be refused", reply)
		}

		for _, name := range []string{`"stray"`, "createOther", "Greeting, PlayerScore"} {
			if !strings.Contains(err.Error(), name) {
				t.Errorf("the refusal names no %s: %v", name, err)
			}
		}
	}
}

func TestSeedsRefusesABrokenVectorsFileAndANonObjectReply(t *testing.T) {
	stores, _ := Stores([]byte(helloSpec), []string{"Greeting"})

	if _, err := Seeds([]byte("{"), stores); err == nil {
		t.Error("a broken vectors file must be refused")
	}

	if _, err := Seeds([]byte(`{"cases":[{"operation":"createGreeting","controllerReply":[1]}]}`), stores); err == nil {
		t.Error("an array reply must be refused")
	}

	if _, err := Seeds([]byte(`{"cases":[{"operation":"createGreeting","controllerReply":{"id":1,"name":"n","count":0}}]}`), stores); err == nil {
		t.Error("a numeric id must be refused")
	}
}

func TestScriptQuotesEveryValueAndWritesOneAuditRowPerSeed(t *testing.T) {
	script := Script(Store{Snake: "greeting", Key: "id"}, []Row{{ID: "a'b", Body: `{"id":"a'b"}`}})

	if !strings.Contains(script, "INSERT INTO \"greeting\" (\"id\", body) VALUES ('a''b', '{\"id\":\"a''b\"}');") {
		t.Errorf("script:\n%s", script)
	}

	if !strings.Contains(script, "'greeting', 'a''b', 'seed', NULL,") {
		t.Errorf("script:\n%s", script)
	}
}

func TestPlanPairsEveryStoreWithItsRowsAndScript(t *testing.T) {
	databases, err := Plan([]byte(helloSpec), []string{"Greeting", "PlayerScore"}, []byte(helloVectors))
	if err != nil {
		t.Fatal(err)
	}

	if len(databases) != 2 || len(databases[0].Rows) != 1 || len(databases[1].Rows) != 1 {
		t.Fatalf("databases: %+v", databases)
	}

	if !strings.HasPrefix(databases[0].Script, storeddl.Table(storeddl.Shape{Snake: "greeting", Key: "id"})) {
		t.Errorf("script: %s", databases[0].Script)
	}
}

func TestPlanWithoutVectorsYieldsEmptyStores(t *testing.T) {
	databases, err := Plan([]byte(helloSpec), []string{"Greeting"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(databases[0].Rows) != 0 || databases[0].Script != storeddl.Table(storeddl.Shape{Snake: "greeting", Key: "id"})+"\n" {
		t.Errorf("databases: %+v", databases)
	}
}

func TestPlanReportsAMissingStoreAndABrokenVectorsFile(t *testing.T) {
	if _, err := Plan([]byte(helloSpec), []string{"Nope"}, nil); err == nil {
		t.Error("a missing store must be reported")
	}

	if _, err := Plan([]byte(helloSpec), []string{"Greeting"}, []byte("{")); err == nil {
		t.Error("a broken vectors file must be reported")
	}
}

func TestTheDetectedWriterCreatesAFileThatHoldsTheSeededRows(t *testing.T) {
	writer, err := DetectWriter()
	if err != nil {
		t.Skip(err)
	}

	path := filepath.Join(t.TempDir(), "greeting.db")

	if err := writer.Write(context.Background(), path, Script(Store{Snake: "greeting", Key: "id"}, []Row{{ID: "1", Body: `{"id":"1"}`}})); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(content) < 16 || string(content[:15]) != "SQLite format 3" {
		t.Errorf("header: %q", content[:min(len(content), 16)])
	}

	if !strings.Contains(string(content), `{"id":"1"}`) {
		t.Error("the seeded body must sit in the file")
	}
}

func TestASeedHoldingTwoRowsThatShareOneUniqueLookupIsRefusedByTheIndexThatNamesThem(t *testing.T) {
	writer, err := DetectWriter()
	if err != nil {
		t.Skip(err)
	}

	store := Store{Snake: "profile", Key: "subject", Uniques: [][]string{{"alias", "tag"}}}
	rows := []Row{
		{ID: "account-one", Body: `{"subject":"account-one","alias":"songe","tag":"blue"}`},
		{ID: "account-two", Body: `{"subject":"account-two","alias":"songe","tag":"blue"}`},
	}

	err = writer.Write(context.Background(), filepath.Join(t.TempDir(), "profile.db"), Script(store, rows))
	if err == nil || !strings.Contains(err.Error(), "UNIQUE constraint failed: index 'profile_unique_by_alias_and_tag'") {
		t.Fatalf("a duplicate seed was not refused by the unique index: %v", err)
	}
}

func TestASeedHoldingOneKeyTwiceIsRefusedRatherThanReplacingTheEarlierRow(t *testing.T) {
	writer, err := DetectWriter()
	if err != nil {
		t.Skip(err)
	}

	store := Store{Snake: "greeting", Key: "id"}
	rows := []Row{
		{ID: "one", Body: `{"id":"one","name":"first"}`},
		{ID: "one", Body: `{"id":"one","name":"second"}`},
	}

	err = writer.Write(context.Background(), filepath.Join(t.TempDir(), "greeting.db"), Script(store, rows))
	if err == nil || !strings.Contains(err.Error(), "UNIQUE constraint failed: greeting.id") {
		t.Fatalf("a repeated key was not refused: %v", err)
	}
}

func TestTheWriterReportsAScriptTheSqliteEngineRefuses(t *testing.T) {
	writer, err := DetectWriter()
	if err != nil {
		t.Skip(err)
	}

	path := filepath.Join(t.TempDir(), "broken.db")

	if err := writer.Write(context.Background(), path, "THIS IS NOT SQL;"); err == nil {
		t.Error("a broken script must be reported")
	}
}

func TestDetectWriterFailsWhenNeitherSqlite3NorPython3IsOnPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	if _, err := exec.LookPath("python3"); err == nil {
		t.Skip("python3 resolves without PATH")
	}

	if _, err := DetectWriter(); err == nil {
		t.Error("an empty PATH must leave no writer")
	}
}
