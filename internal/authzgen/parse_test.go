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

package authzgen_test

import (
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/authzgen"
)

const sessionSpec = `module: session
types:
  - name: account
  - name: character
    relations:
      - name: owner
        subjects: [account]
  - name: session
    relations:
      - name: host
        subjects: [character]
      - name: member
        subjects: [character]
      - name: join
        expression: member
`

func refused(t *testing.T, doc string, wants ...string) {
	t.Helper()

	_, err := authzgen.ParseSpec([]byte(doc))
	if err == nil {
		t.Fatalf("the document was accepted\n%s", doc)
	}

	for _, want := range wants {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the refusal %q never named %q", err, want)
		}
	}
}

func TestAValidModuleParsesWithItsTypesInDeclaredOrder(t *testing.T) {
	m, err := authzgen.ParseSpec([]byte(sessionSpec))
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	if m.Name != "session" {
		t.Fatalf("got module %q", m.Name)
	}

	got := []string{}
	for _, ty := range m.Types {
		got = append(got, ty.Name)
	}

	if strings.Join(got, ",") != "account,character,session" {
		t.Fatalf("got types %q", got)
	}
}

func TestAModuleNameThatIsNotALowerCaseIdentifierIsRefusedWithTheFix(t *testing.T) {
	refused(t, "module: Session\n", `module "Session" is not a lower case identifier`, "start with a letter")
}

func TestAnUnknownKeyInTheDocumentIsRefused(t *testing.T) {
	refused(t, "module: session\nmodul: x\n", "modul")
}

func TestAnUnknownSubjectTypeIsRefusedNamingTypesAndReferences(t *testing.T) {
	refused(t, `module: session
types:
  - name: session
    relations:
      - name: member
        subjects: [player]
`, `subject "player" names unknown type "player"`, "declare it under types or name its module under references")
}

func TestAnExpressionNamingAnUnknownRelationIsRefused(t *testing.T) {
	refused(t, `module: session
types:
  - name: session
    relations:
      - name: join
        expression: member
`, `type "session" relation "join"`, `names unknown relation "member" on type "session"`)
}

func TestACaseNamingAnUnknownTypeIsRefused(t *testing.T) {
	refused(t, sessionSpec+`vectors:
  - name: a room check
    checks:
      - user: character:alice
        relation: join
        object: room:r1
        expected: true
`, `vector "a room check" check`, `object "room:r1" names unknown type "room"`)
}

func TestACaseNamingAnUnknownRelationIsRefused(t *testing.T) {
	refused(t, sessionSpec+`vectors:
  - name: a fly check
    checks:
      - user: character:alice
        relation: fly
        object: session:s1
        expected: true
`, `relation "fly" is not a relation of type "session"`)
}

func TestACaseTupleNamingAnUnknownUserTypeIsRefused(t *testing.T) {
	refused(t, sessionSpec+`vectors:
  - name: a player tuple
    tuples:
      - user: player:p1
        relation: member
        object: session:s1
    checks:
      - user: character:alice
        relation: join
        object: session:s1
        expected: false
`, `vector "a player tuple" tuple`, `user "player:p1" names unknown type "player"`)
}

func TestACheckWithoutExpectedIsRefused(t *testing.T) {
	refused(t, sessionSpec+`vectors:
  - name: no expected
    checks:
      - user: character:alice
        relation: join
        object: session:s1
`, "has no expected, write true or false")
}

func TestACaseWithoutChecksIsRefused(t *testing.T) {
	refused(t, sessionSpec+`vectors:
  - name: nothing asserted
    tuples:
      - user: character:alice
        relation: member
        object: session:s1
`, `vector "nothing asserted" has no checks`)
}

func TestACheckWithAWildcardUserIsRefused(t *testing.T) {
	refused(t, sessionSpec+`vectors:
  - name: wildcard check
    checks:
      - user: "character:*"
        relation: join
        object: session:s1
        expected: true
`, "a check names one user")
}

func TestARelationWithNeitherSubjectsNorAnExpressionIsRefused(t *testing.T) {
	refused(t, `module: session
types:
  - name: session
    relations:
      - name: member
`, `type "session" relation "member" has neither subjects nor an expression`)
}

func TestAnUnknownConditionOnARelationIsRefused(t *testing.T) {
	refused(t, `module: play
types:
  - name: character
  - name: monster
    relations:
      - name: alive
        subjects: [character]
        condition: breathing
`, `names unknown condition "breathing"`, "declare it under conditions")
}

func TestAConditionOnARelationWithoutSubjectsIsRefused(t *testing.T) {
	refused(t, `module: play
conditions:
  - name: alive
    parameters:
      - name: hp
        type: int
    expression: hp > 0
types:
  - name: character
  - name: monster
    relations:
      - name: owner
        subjects: [character]
      - name: attack
        expression: owner
        condition: alive
`, "a condition guards direct subjects")
}

func TestAConditionParameterWithAnUnknownTypeIsRefused(t *testing.T) {
	refused(t, `module: play
conditions:
  - name: alive
    parameters:
      - name: hp
        type: number
    expression: hp > 0
`, `condition "alive" parameter "hp" has type "number"`, "list<T> or map<T>")
}

func conditionTyped(parameterType string) string {
	return `module: play
conditions:
  - name: alive
    parameters:
      - name: hp
        type: ` + parameterType + `
    expression: hp > 0
`
}

func TestAConditionParameterTypedAnyIsRefusedNamingTheParameterTheTypeAndTheAllowedTypes(t *testing.T) {
	refused(t, conditionTyped("any"),
		`condition "alive" parameter "hp" has type "any"`,
		"use bool, string, int, uint, double, duration, timestamp, ipaddress, list<T> or map<T>",
	)
}

func TestAConditionParameterTypedBytesIsRefusedBecauseTheDslGrammarNamesNoBytes(t *testing.T) {
	refused(t, conditionTyped("bytes"), `condition "alive" parameter "hp" has type "bytes"`)
}

func TestAListOfAnyIsRefusedWhileAListOfAScalarIsAccepted(t *testing.T) {
	refused(t, conditionTyped("list<any>"), `parameter "hp" has type "list<any>"`)

	if _, err := authzgen.ParseSpec([]byte(conditionTyped("list<string>"))); err != nil {
		t.Fatalf("parsing a list of string: %v", err)
	}
}

func TestEveryScalarTheDslGrammarNamesIsAcceptedAsAConditionParameterType(t *testing.T) {
	for _, parameterType := range []string{
		"bool", "string", "int", "uint", "double", "duration", "timestamp", "ipaddress",
		"list<int>", "map<string>",
	} {
		if _, err := authzgen.ParseSpec([]byte(conditionTyped(parameterType))); err != nil {
			t.Errorf("parsing a parameter typed %s: %v", parameterType, err)
		}
	}
}

func TestAConditionWithoutAnExpressionIsRefused(t *testing.T) {
	refused(t, `module: play
conditions:
  - name: alive
    parameters:
      - name: hp
        type: int
    expression: ""
`, `condition "alive" has no expression`)
}

func TestATupleNamingAnUnknownConditionIsRefused(t *testing.T) {
	refused(t, sessionSpec+`vectors:
  - name: a conditioned tuple
    tuples:
      - user: character:alice
        relation: member
        object: session:s1
        condition:
          name: alive
    checks:
      - user: character:alice
        relation: join
        object: session:s1
        expected: true
`, `vector "a conditioned tuple" tuple names unknown condition "alive"`)
}

func TestAnExtensionRepeatingAReferencedRelationIsRefused(t *testing.T) {
	refused(t, `module: play
references:
  - module: session
    types:
      - name: character
      - name: session
        relations: [member]
extends:
  - module: session
    types:
      - name: session
        relations:
          - name: member
            subjects: [character]
`, `type "session" relation "member" is already a relation of the referenced type`)
}

func TestAnExtensionOfALocalTypeIsRefused(t *testing.T) {
	refused(t, `module: play
types:
  - name: session
extends:
  - module: session
    types:
      - name: session
        relations:
          - name: turn_owner
            subjects: [session]
`, `type "session" is declared in this module, an extended type lives in another module`)
}

func TestAnExtensionAddingNoRelationIsRefused(t *testing.T) {
	refused(t, `module: play
extends:
  - module: session
    types:
      - name: session
`, `type "session" adds no relation`)
}

func TestAReferenceToThisModuleIsRefused(t *testing.T) {
	refused(t, `module: play
references:
  - module: play
    types:
      - name: session
`, `references module "play" is this module`)
}

func TestAFromRelationReadingThroughAComputedRelationIsRefused(t *testing.T) {
	refused(t, `module: chat
references:
  - module: session
    types:
      - name: session
        relations: [member]
types:
  - name: channel
    relations:
      - name: session
        subjects: [session]
      - name: parent
        expression: session
      - name: member
        expression: member from parent
`, `reads "member" from "parent" but "parent" is computed`)
}

func TestAFromRelationNoSubjectTypeHoldsIsRefused(t *testing.T) {
	refused(t, `module: chat
references:
  - module: session
    types:
      - name: session
        relations: [member]
types:
  - name: channel
    relations:
      - name: session
        subjects: [session]
      - name: writer
        expression: writer from session
`, `reads "writer" from "session" but no subject type of "session" has relation "writer"`)
}

func TestAFromRelationThroughAReferencedTypeWithKnownRelationsParses(t *testing.T) {
	_, err := authzgen.ParseSpec([]byte(`module: chat
references:
  - module: session
    types:
      - name: character
      - name: session
        relations: [member]
types:
  - name: channel
    relations:
      - name: session
        subjects: [session]
      - name: member
        expression: member from session
`))
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
}

func TestAUsersetSubjectNamingAnUnknownRelationIsRefused(t *testing.T) {
	refused(t, `module: chat
types:
  - name: team
  - name: channel
    relations:
      - name: member
        subjects: ["team#member"]
`, `subject "team#member" names unknown relation "member" on type "team"`)
}

func TestADuplicateCaseNameIsRefused(t *testing.T) {
	refused(t, sessionSpec+`vectors:
  - name: twice
    checks:
      - user: character:alice
        relation: join
        object: session:s1
        expected: false
  - name: twice
    checks:
      - user: character:alice
        relation: join
        object: session:s1
        expected: false
`, `vector "twice" is declared twice`)
}
