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

package authzgen

import (
	"strings"
	"testing"
)

func rendered(t *testing.T, text string) string {
	t.Helper()

	e, err := parseExpression(text)
	if err != nil {
		t.Fatalf("parsing %q: %v", text, err)
	}

	return e.String()
}

func refusedExpression(t *testing.T, text, want string) {
	t.Helper()

	_, err := parseExpression(text)
	if err == nil {
		t.Fatalf("%q was accepted", text)
	}

	if !strings.Contains(err.Error(), want) {
		t.Fatalf("the refusal %q never named %q", err, want)
	}
}

func TestParsingAnOrChainKeepsTheOrderOfItsTerms(t *testing.T) {
	if got := rendered(t, "owner  or editor or   viewer"); got != "owner or editor or viewer" {
		t.Fatalf("got %q", got)
	}
}

func TestParsingAFromTermRendersRelationFromTupleset(t *testing.T) {
	if got := rendered(t, "member from session"); got != "member from session" {
		t.Fatalf("got %q", got)
	}
}

func TestParsingAGroupRendersItInParentheses(t *testing.T) {
	if got := rendered(t, "(owner or editor) and member"); got != "(owner or editor) and member" {
		t.Fatalf("got %q", got)
	}
}

func TestParsingButNotRendersBothSides(t *testing.T) {
	if got := rendered(t, "member but not blocked"); got != "member but not blocked" {
		t.Fatalf("got %q", got)
	}
}

func TestTheTopOperatorOfAnAndChainIsAnd(t *testing.T) {
	e, err := parseExpression("alive and turn_owner from session and member from session")
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	if e.operator != operatorAnd || len(e.terms) != 3 {
		t.Fatalf("got operator %q with %d terms", e.operator, len(e.terms))
	}
}

func TestMixingOrAndAndWithoutParenthesesIsRefused(t *testing.T) {
	refusedExpression(t, "owner or editor and member", "group one side with parentheses")
}

func TestButNotWithAThirdTermIsRefused(t *testing.T) {
	refusedExpression(t, "member but not blocked but not banned", "but not takes one relation on each side")
}

func TestButWithoutNotIsRefused(t *testing.T) {
	refusedExpression(t, "member but blocked", "write but not")
}

func TestAnUnclosedParenthesisIsRefused(t *testing.T) {
	refusedExpression(t, "(owner or editor", "never closed")
}

func TestAClosingParenthesisWithoutAnOpeningOneIsRefused(t *testing.T) {
	refusedExpression(t, "owner or editor)", "no opening one")
}

func TestAnEmptyExpressionIsRefused(t *testing.T) {
	refusedExpression(t, "   ", "the expression is empty")
}

func TestAnExpressionEndingOnAnOperatorIsRefused(t *testing.T) {
	refusedExpression(t, "owner or", "ends where a relation is expected")
}

func TestAFromWithoutATuplesetIsRefused(t *testing.T) {
	refusedExpression(t, "member from", "write member from <relation>")
}

func TestAWordThatIsNotARelationNameIsRefused(t *testing.T) {
	refusedExpression(t, "Owner", `"Owner" is not a relation name`)
}

func TestTwoRelationsWithNoOperatorBetweenThemAreRefused(t *testing.T) {
	refusedExpression(t, "owner editor", "expected or, and or but not after a relation")
}
