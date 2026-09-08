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
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/authzgen"
)

func parses(t *testing.T, doc string) {
	t.Helper()

	if _, err := authzgen.ParseSpec([]byte(doc)); err != nil {
		t.Fatalf("parsing: %v", err)
	}
}

func TestARelationComputedFromItselfIsRefusedNamingTheRelationAndTheCycle(t *testing.T) {
	refused(t, `module: session
types:
  - name: session
    relations:
      - name: a
        expression: a
`, `type "session" relation "a" is computed from itself`, "the cycle is a -> a")
}

func TestTwoRelationsComputedFromEachOtherAreRefusedNamingTheCycle(t *testing.T) {
	refused(t, `module: session
types:
  - name: session
    relations:
      - name: b
        expression: c
      - name: c
        expression: b
`, `type "session" relation "b" is computed from itself`, "the cycle is b -> c -> b")
}

func TestACycleReachedThroughAThirdRelationIsRefusedNamingOnlyTheLoop(t *testing.T) {
	refused(t, `module: session
types:
  - name: character
  - name: session
    relations:
      - name: host
        subjects: [character]
      - name: entry
        expression: b
      - name: b
        expression: c
      - name: c
        expression: b
`, "the cycle is b -> c -> b")
}

func TestACycleInsideAGroupedExpressionIsRefused(t *testing.T) {
	refused(t, `module: session
types:
  - name: character
  - name: session
    relations:
      - name: host
        subjects: [character]
      - name: a
        expression: host or (host and a)
`, `relation "a" is computed from itself`, "the cycle is a -> a")
}

func TestACycleAcrossALocalTypeAndAnExtendedTypeOfTheSameNameIsRefused(t *testing.T) {
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
          - name: turn_owner
            expression: end_turn
          - name: end_turn
            expression: turn_owner
`, `type "session" relation "turn_owner" is computed from itself`, "the cycle is turn_owner -> end_turn -> turn_owner")
}

func TestALegalChainOfComputedRelationsParses(t *testing.T) {
	parses(t, `module: session
types:
  - name: character
  - name: session
    relations:
      - name: host
        subjects: [character]
      - name: a
        expression: b
      - name: b
        expression: c
      - name: c
        expression: host
`)
}

func TestARelationReadFromATuplesetOfAnotherTypeIsNoCycle(t *testing.T) {
	parses(t, `module: chat
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
      - name: post
        expression: member
`)
}

func TestARelationNamedInABranchOfAButNotExpressionStillCounts(t *testing.T) {
	refused(t, `module: session
types:
  - name: character
  - name: session
    relations:
      - name: host
        subjects: [character]
      - name: banned
        expression: a
      - name: a
        expression: host but not banned
`, "the cycle is banned -> a -> banned")
}
