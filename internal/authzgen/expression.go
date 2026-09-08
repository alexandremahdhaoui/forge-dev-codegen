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
	"fmt"
	"strings"
)

const (
	operatorOr     = "or"
	operatorAnd    = "and"
	operatorButNot = "but not"
)

type expression struct {
	operator string
	terms    []term
}

type term struct {
	relation string
	tupleset string
	group    *expression
}

type expressionParser struct {
	tokens []string
	pos    int
}

func parseExpression(text string) (*expression, error) {
	p := &expressionParser{tokens: tokenize(text)}

	if len(p.tokens) == 0 {
		return nil, fmt.Errorf("the expression is empty, name the relations it is computed from")
	}

	e, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	if p.pos < len(p.tokens) {
		return nil, fmt.Errorf("unexpected %q, a closing parenthesis has no opening one", p.tokens[p.pos])
	}

	return e, nil
}

func tokenize(text string) []string {
	spaced := strings.NewReplacer("(", " ( ", ")", " ) ").Replace(text)

	return strings.Fields(spaced)
}

func (p *expressionParser) next() (string, bool) {
	if p.pos >= len(p.tokens) {
		return "", false
	}

	tok := p.tokens[p.pos]
	p.pos++

	return tok, true
}

func (p *expressionParser) peek() string {
	if p.pos >= len(p.tokens) {
		return ""
	}

	return p.tokens[p.pos]
}

func (p *expressionParser) parseExpression() (*expression, error) {
	first, err := p.parseTerm()
	if err != nil {
		return nil, err
	}

	e := &expression{terms: []term{first}}

	for p.pos < len(p.tokens) && p.peek() != ")" {
		op, err := p.parseOperator()
		if err != nil {
			return nil, err
		}

		if e.operator != "" && e.operator != op {
			return nil, fmt.Errorf("%q and %q mix in one expression, group one side with parentheses", e.operator, op)
		}

		if op == operatorButNot && len(e.terms) == 2 {
			return nil, fmt.Errorf("but not takes one relation on each side, group the rest with parentheses")
		}

		e.operator = op

		t, err := p.parseTerm()
		if err != nil {
			return nil, err
		}

		e.terms = append(e.terms, t)
	}

	return e, nil
}

func (p *expressionParser) parseOperator() (string, error) {
	tok, _ := p.next()

	switch tok {
	case operatorOr, operatorAnd:
		return tok, nil
	case "but":
		if not, ok := p.next(); !ok || not != "not" {
			return "", fmt.Errorf("but is followed by not, write but not")
		}

		return operatorButNot, nil
	default:
		return "", fmt.Errorf("unexpected %q, expected or, and or but not after a relation", tok)
	}
}

func (p *expressionParser) parseTerm() (term, error) {
	tok, ok := p.next()
	if !ok {
		return term{}, fmt.Errorf("the expression ends where a relation is expected")
	}

	if tok == "(" {
		group, err := p.parseExpression()
		if err != nil {
			return term{}, err
		}

		if closing, ok := p.next(); !ok || closing != ")" {
			return term{}, fmt.Errorf("a parenthesis is never closed")
		}

		return term{group: group}, nil
	}

	if !ident.MatchString(tok) {
		return term{}, fmt.Errorf("%q is not a relation name, a relation is a lower case identifier", tok)
	}

	if p.peek() != "from" {
		return term{relation: tok}, nil
	}

	p.pos++

	tupleset, ok := p.next()
	if !ok || !ident.MatchString(tupleset) {
		return term{}, fmt.Errorf("%q from names no relation, write %s from <relation>", tok, tok)
	}

	return term{relation: tok, tupleset: tupleset}, nil
}

func (e *expression) String() string {
	parts := make([]string, 0, len(e.terms))

	for _, t := range e.terms {
		parts = append(parts, t.String())
	}

	return strings.Join(parts, " "+e.operator+" ")
}

func (t term) String() string {
	if t.group != nil {
		return "(" + t.group.String() + ")"
	}

	if t.tupleset != "" {
		return t.relation + " from " + t.tupleset
	}

	return t.relation
}

func (e *expression) walk(visit func(term) error) error {
	for _, t := range e.terms {
		if t.group != nil {
			if err := t.group.walk(visit); err != nil {
				return err
			}

			continue
		}

		if err := visit(t); err != nil {
			return err
		}
	}

	return nil
}
