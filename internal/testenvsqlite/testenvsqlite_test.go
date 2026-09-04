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
)

const helloSpec = `
components:
  schemas:
    CreateGreetingRequest:
      type: object
      required: [name]
    Greeting:
      type: object
      x-store: true
      required: [id, name, count]
    PlayerScore:
      type: object
      x-store: true
      required: [id, score]
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

	if strings.Join(stores[0].Required, ",") != "id,name,count" {
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

func TestDDLMatchesTheSchemaTheHexagonalRustSqliteAdapterCreates(t *testing.T) {
	want := "CREATE TABLE IF NOT EXISTS greeting (id TEXT PRIMARY KEY, body TEXT NOT NULL);\n" +
		"CREATE TABLE IF NOT EXISTS audit (at TEXT NOT NULL, table_name TEXT NOT NULL, key TEXT NOT NULL, op TEXT NOT NULL, before TEXT, after TEXT);"

	if got := DDL("greeting"); got != want {
		t.Errorf("DDL:\n got %q\nwant %q", got, want)
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

func TestSeedsSkipsAReplyThatMatchesNoStoreOrHasNoId(t *testing.T) {
	stores, err := Stores([]byte(helloSpec), []string{"Greeting"})
	if err != nil {
		t.Fatal(err)
	}

	vectors := `{"cases":[
	  {"case":"a","operation":"createGreeting","controllerReply":{"name":"no id"}},
	  {"case":"b","operation":"createOther","controllerReply":{"id":"x","other":true}}
	]}`

	rows, err := Seeds([]byte(vectors), stores)
	if err != nil {
		t.Fatal(err)
	}

	if len(rows) != 0 {
		t.Errorf("rows: %+v", rows)
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
	script := Script("greeting", []Row{{ID: "a'b", Body: `{"id":"a'b"}`}})

	if !strings.Contains(script, "INSERT OR REPLACE INTO greeting (id, body) VALUES ('a''b', '{\"id\":\"a''b\"}');") {
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

	if !strings.HasPrefix(databases[0].Script, DDL("greeting")) {
		t.Errorf("script: %s", databases[0].Script)
	}
}

func TestPlanWithoutVectorsYieldsEmptyStores(t *testing.T) {
	databases, err := Plan([]byte(helloSpec), []string{"Greeting"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(databases[0].Rows) != 0 || databases[0].Script != DDL("greeting")+"\n" {
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

	if err := writer.Write(context.Background(), path, Script("greeting", []Row{{ID: "1", Body: `{"id":"1"}`}})); err != nil {
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
