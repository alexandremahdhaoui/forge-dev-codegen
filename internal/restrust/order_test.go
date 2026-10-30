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

package restrust_test

import (
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/restrust"
)

const chatLineSpec = `
openapi: 3.1.0
info:
  title: Chat API
  version: 1.0.0
paths:
  /chat-lines/{id}:
    get:
      operationId: getChatLine
      x-controller: chat
      x-ports: [ChatLineStore]
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        "200":
          description: The chat line
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/ChatLine"
components:
  schemas:
    ChatLine:
      type: object
      x-store:
        key: id
        lookups:
          - { by: [chatId], answers: page }
          - { by: [accountId, chatId], answers: page }
          - { by: [line], answers: one }
        order: postedAt
        adapters: [sqlite, memory]
      required: [id, chatId, accountId, line, postedAt, edits]
      properties:
        id:
          type: string
        chatId:
          type: string
        accountId:
          type: string
        line:
          type: string
        postedAt:
          type: string
          format: date-time
        edits:
          type: integer
`

const chatLineStore = `      x-store:
        key: id
        lookups:
          - { by: [chatId], answers: page }
          - { by: [accountId, chatId], answers: page }
          - { by: [line], answers: one }
        order: postedAt
        adapters: [sqlite, memory]
`

func withChatLineStore(block string) string {
	return strings.Replace(chatLineSpec, chatLineStore, block, 1)
}

func parseRefusal(t *testing.T, spec, want string) {
	t.Helper()

	_, err := restrust.Parse([]byte(spec))
	if err == nil || err.Error() != want {
		t.Fatalf("wanted the refusal\n%s\ngot\n%v", want, err)
	}
}

func TestAnOrderOnAPropertyTheSchemaDoesNotDeclareIsRefusedByName(t *testing.T) {
	parseRefusal(t,
		withChatLineStore("      x-store:\n        key: id\n        lookups:\n          - { by: [chatId], answers: page }\n        order: sentAt\n        adapters: [sqlite]\n"),
		`reading schema "ChatLine": the x-store order names property "sentAt", which the schema does not declare`,
	)
}

func TestAnOrderOnAPropertyThatIsNotARequiredStringIsRefusedByName(t *testing.T) {
	parseRefusal(t,
		withChatLineStore("      x-store:\n        key: id\n        lookups:\n          - { by: [chatId], answers: page }\n        order: edits\n        adapters: [sqlite]\n"),
		`reading schema "ChatLine": the x-store order names property "edits", which must be a required string property`,
	)
}

func TestAnOrderNamingTheKeyIsRefusedBecauseAStoreWithNoOrderAlreadyReadsInKeyOrder(t *testing.T) {
	parseRefusal(t,
		withChatLineStore("      x-store:\n        key: id\n        lookups:\n          - { by: [chatId], answers: page }\n        order: id\n        adapters: [sqlite]\n"),
		`reading schema "ChatLine": x-store orders by "id", which is the key, and a store with no order already reads every page in key order`,
	)
}

func TestAnOrderOnAStoreWithNoPageLookupIsRefusedBecauseNoLookupReadsIt(t *testing.T) {
	parseRefusal(t,
		withChatLineStore("      x-store:\n        key: id\n        lookups:\n          - { by: [line], answers: one }\n        order: postedAt\n        adapters: [sqlite]\n"),
		`reading schema "ChatLine": x-store names an order and holds no page lookup, an order no lookup reads is a declaration nobody honours`,
	)
}

func TestAnOrderNamingAPropertyOfAPageLookupsOwnByListIsRefusedBecauseOnlyTheKeyWouldOrderItsRows(t *testing.T) {
	parseRefusal(t,
		withChatLineStore("      x-store:\n        key: id\n        lookups:\n          - { by: [chatId], answers: page }\n          - { by: [chatId, postedAt], answers: page }\n        order: postedAt\n        adapters: [sqlite]\n"),
		`reading schema "ChatLine": the page lookup by "chatId", "postedAt" orders by "postedAt", every row it answers holds one value there and only the key would order them`,
	)
}

func TestAnUnknownXStoreKeyIsRefusedNamingTheKeysXStoreHolds(t *testing.T) {
	parseRefusal(t,
		withChatLineStore("      x-store:\n        key: id\n        lookups:\n          - { by: [chatId], answers: page }\n        sort: postedAt\n        adapters: [sqlite]\n"),
		`reading schema "ChatLine": x-store names "sort", x-store names only key, lookups, order and adapters`,
	)
}

func TestAnUnknownLookupKeyIsRefusedNamingTheKeysALookupHolds(t *testing.T) {
	parseRefusal(t,
		withChatLineStore("      x-store:\n        key: id\n        lookups:\n          - { by: [chatId], answers: page, order: postedAt }\n        adapters: [sqlite]\n"),
		`reading schema "ChatLine": the lookup names "order", a lookup names only by and answers`,
	)
}

func TestTheSqliteSchemaOfAStoreWithAnOrderHoldsTheIndexOfEveryPageLookup(t *testing.T) {
	byPath, err := generatedByPath(t, chatLineSpec)
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	wantIn(t, byPath, "adapter/zz_generated_chat_line_sqlite.rs",
		`CREATE INDEX IF NOT EXISTS "chat_line_page_by_chat_id" ON "chat_line" (json_extract(body, '$.chatId'), json_extract(body, '$.postedAt'), "id");`,
		`CREATE INDEX IF NOT EXISTS "chat_line_page_by_account_id_and_chat_id" ON "chat_line" (json_extract(body, '$.accountId'), json_extract(body, '$.chatId'), json_extract(body, '$.postedAt'), "id");`,
	)
}

func TestAStoreWithNoOrderKeepsItsKeyOrderAndItsStringCursor(t *testing.T) {
	byPath, err := generatedByPath(t, helloSpec)
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	wantIn(t, byPath, "port/zz_generated_greeting_store.rs",
		"fn page_by_name(&self, name: &str, after: Option<String>, limit: Option<i64>) -> Result<Vec<Greeting>, GreetingStoreError>;",
	)
	wantIn(t, byPath, "adapter/zz_generated_greeting_sqlite.rs",
		`"SELECT body FROM \"greeting\" WHERE {} ORDER BY \"id\" LIMIT ?{}"`,
		`clauses.push(format!("\"id\" > ?{}", bound.len() + 1));`,
		"self.rows_where(&[(GreetingColumn::Name, name)], after.as_deref(), limit)",
	)
	wantIn(t, byPath, "adapter/zz_generated_greeting_memory.rs",
		".filter(|(key, _)| after.is_none_or(|cursor| key.as_str() > cursor))",
	)

	for path, content := range byPath {
		for _, absent := range []string{"GreetingCursor", "CREATE INDEX"} {
			if strings.Contains(content, absent) {
				t.Errorf("%s holds %q, which only a store with an order carries:\n%s", path, absent, content)
			}
		}
	}
}
