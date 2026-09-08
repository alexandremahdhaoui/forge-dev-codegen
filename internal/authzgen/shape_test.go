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

import "testing"

func TestTypesWrittenAsAMappingOfTypeNamesIsRefusedNamingTheKeyAndTheShape(t *testing.T) {
	refused(t, `module: session
types:
  account: {}
  character:
    relations:
      owner: [account]
`, "authz.yaml types: expected a list of types, got a mapping")
}

func TestExtendsWrittenAsAListOfModuleNamesIsRefusedNamingTheEntryAndTheShape(t *testing.T) {
	refused(t, `module: play
extends: [session]
`, "authz.yaml extends[0]: expected a mapping naming a module and the types it extends, got a string")
}

func TestVectorsWrittenAsAMappingOfTuplesAndChecksIsRefusedNamingTheKeyAndTheShape(t *testing.T) {
	refused(t, `module: chat
vectors:
  tuples:
    - user: character:alice
      relation: member
      object: session:s1
  checks:
    - name: a member may post
      user: character:alice
      relation: post
      object: channel:c1
      expect: allow
`, "authz.yaml vectors: expected a list of cases, got a mapping")
}

func TestConditionsWrittenAsAMappingOfConditionNamesIsRefusedNamingTheKeyAndTheShape(t *testing.T) {
	refused(t, `module: session
conditions:
  code_matches:
    parameters:
      code: string
    expression: code == presented
`, "authz.yaml conditions: expected a list of conditions, got a mapping")
}

func TestSubjectsWrittenAsAMappingIsRefusedNamingTheTypeTheRelationAndTheShape(t *testing.T) {
	refused(t, `module: session
types:
  - name: session
    relations:
      - name: member
        subjects:
          character: true
`, "authz.yaml types.session.relations.member.subjects: expected a list of subject types, got a mapping")
}

func TestAParameterListWrittenAsAMappingIsRefusedNamingTheCondition(t *testing.T) {
	refused(t, `module: session
conditions:
  - name: code_matches
    parameters:
      code: string
    expression: code == presented
`, "authz.yaml conditions.code_matches.parameters: expected a list of parameters, got a mapping")
}

func TestAnExpectedThatIsNotABooleanIsRefusedNamingTheCaseAndTheCheck(t *testing.T) {
	refused(t, sessionSpec+`vectors:
  - name: a member may join
    checks:
      - user: character:alice
        relation: join
        object: session:s1
        expected: allow
`, "authz.yaml vectors.a member may join.checks[0].expected: expected true or false, got a string")
}

func TestAModuleNameWrittenAsAListIsRefusedNamingTheKey(t *testing.T) {
	refused(t, "module: [session]\n", "authz.yaml module: expected a module name, got a list")
}

func TestAnUnknownKeyIsRefusedNamingItsPathAndTheKeysThatBelongThere(t *testing.T) {
	refused(t, `module: session
types:
  - name: session
    relation:
      - name: member
        subjects: [session]
`, "authz.yaml types.session.relation: unknown key, write one of name, relations")
}

func TestAModuleNameWrittenAsANumberIsRefusedNamingTheKind(t *testing.T) {
	refused(t, "module: 3\n", "authz.yaml module: expected a module name, got a number")
}

func TestADocumentThatIsNotYamlIsRefusedNamingTheFile(t *testing.T) {
	refused(t, "module: [session\n", "reading authz.yaml:")
}

func TestADocumentThatIsAListIsRefusedNamingTheFile(t *testing.T) {
	refused(t, "- module: session\n", "reading authz.yaml: expected a mapping naming a module, its types and its vectors, got a list")
}

func TestATupleContextWrittenAsAListIsRefusedNamingTheTuple(t *testing.T) {
	refused(t, sessionSpec+`vectors:
  - name: a conditioned tuple
    tuples:
      - user: character:alice
        relation: member
        object: session:s1
        condition:
          name: code_matches
          context: [BOARRR]
    checks:
      - user: character:alice
        relation: join
        object: session:s1
        expected: true
`, "authz.yaml vectors.a conditioned tuple.tuples[0].condition.context: expected a mapping of parameter values, got a list")
}
