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

package grpcrust

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

var scalarTypes = map[string]bool{
	"double": true, "float": true, "int32": true, "int64": true, "uint32": true, "uint64": true,
	"sint32": true, "sint64": true, "fixed32": true, "fixed64": true, "sfixed32": true, "sfixed64": true,
	"bool": true, "string": true, "bytes": true,
}

type tokenKind string

const (
	tokIdent tokenKind = "ident"
	tokPunct tokenKind = "punct"
	tokNum   tokenKind = "num"
	tokStr   tokenKind = "str"
)

type token struct {
	kind tokenKind
	text string
}

func lex(doc []byte) ([]token, error) {
	tokens := []token{}
	runes := []rune(string(doc))

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		switch {
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			continue
		case r == '/' && i+1 < len(runes) && runes[i+1] == '/':
			for i < len(runes) && runes[i] != '\n' {
				i++
			}
		case r == '/' && i+1 < len(runes) && runes[i+1] == '*':
			i += 2

			for i+1 < len(runes) && (runes[i] != '*' || runes[i+1] != '/') {
				i++
			}

			i++
		case r == '"' || r == '\'':
			quote := r
			j := i + 1

			for j < len(runes) && runes[j] != quote {
				j++
			}

			if j >= len(runes) {
				return nil, fmt.Errorf("parsing the proto document: unterminated string literal")
			}

			tokens = append(tokens, token{kind: tokStr, text: string(runes[i+1 : j])})
			i = j
		case unicode.IsLetter(r) || r == '_':
			j := i

			for j < len(runes) && (unicode.IsLetter(runes[j]) || unicode.IsDigit(runes[j]) || runes[j] == '_') {
				j++
			}

			tokens = append(tokens, token{kind: tokIdent, text: string(runes[i:j])})
			i = j - 1
		case unicode.IsDigit(r):
			j := i

			for j < len(runes) && (unicode.IsDigit(runes[j]) || runes[j] == '.') {
				j++
			}

			tokens = append(tokens, token{kind: tokNum, text: string(runes[i:j])})
			i = j - 1
		case strings.ContainsRune("{}();=.,[]<>", r):
			tokens = append(tokens, token{kind: tokPunct, text: string(r)})
		default:
			return nil, fmt.Errorf("parsing the proto document: unexpected character %q", string(r))
		}
	}

	return tokens, nil
}

type parser struct {
	tokens []token
	pos    int
}

func (p *parser) atEnd() bool {
	return p.pos >= len(p.tokens)
}

func (p *parser) peek() (token, bool) {
	if p.atEnd() {
		return token{}, false
	}

	return p.tokens[p.pos], true
}

func (p *parser) next() (token, error) {
	t, ok := p.peek()
	if !ok {
		return token{}, fmt.Errorf("parsing the proto document: unexpected end of input")
	}

	p.pos++

	return t, nil
}

func (p *parser) expectPunct(text string) error {
	t, err := p.next()
	if err != nil {
		return err
	}

	if t.kind != tokPunct || t.text != text {
		return fmt.Errorf("parsing the proto document: expected %q, got %q", text, t.text)
	}

	return nil
}

func (p *parser) expectIdent() (string, error) {
	t, err := p.next()
	if err != nil {
		return "", err
	}

	if t.kind != tokIdent {
		return "", fmt.Errorf("parsing the proto document: expected an identifier, got %q", t.text)
	}

	return t.text, nil
}

// Parse reads a proto3 document and returns its package, messages and
// services. It refuses imports, streaming, nested types, oneofs, maps,
// enums, repeated fields and anything else a unary service does not need.
func Parse(doc []byte) (*Spec, error) {
	tokens, err := lex(doc)
	if err != nil {
		return nil, err
	}

	p := &parser{tokens: tokens}
	spec := &Spec{}

	for !p.atEnd() {
		t, err := p.next()
		if err != nil {
			return nil, err
		}

		if t.kind != tokIdent {
			return nil, fmt.Errorf("parsing the proto document: unexpected token %q at the top level", t.text)
		}

		switch t.text {
		case "syntax":
			if err := parseSyntax(p); err != nil {
				return nil, err
			}
		case "package":
			pkg, err := parsePackage(p)
			if err != nil {
				return nil, err
			}

			spec.Package = pkg
		case "message":
			msg, err := parseMessage(p)
			if err != nil {
				return nil, err
			}

			spec.Messages = append(spec.Messages, msg)
		case "service":
			svc, err := parseService(p)
			if err != nil {
				return nil, err
			}

			spec.Services = append(spec.Services, svc)
		case "import":
			return nil, fmt.Errorf("parsing the proto document: import is not supported, inline every message and service in one file")
		case "option":
			return nil, fmt.Errorf("parsing the proto document: option is not supported")
		case "enum":
			return nil, fmt.Errorf("parsing the proto document: enum is not supported")
		case "extend":
			return nil, fmt.Errorf("parsing the proto document: extend is not supported")
		default:
			return nil, fmt.Errorf("parsing the proto document: unexpected top level declaration %q", t.text)
		}
	}

	if err := validateReferences(spec); err != nil {
		return nil, err
	}

	return spec, nil
}

func parseSyntax(p *parser) error {
	if err := p.expectPunct("="); err != nil {
		return err
	}

	t, err := p.next()
	if err != nil {
		return err
	}

	if t.kind != tokStr || t.text != "proto3" {
		return fmt.Errorf("parsing the proto document: only syntax %q is supported, got %q", "proto3", t.text)
	}

	return p.expectPunct(";")
}

func parsePackage(p *parser) (string, error) {
	parts := []string{}

	name, err := p.expectIdent()
	if err != nil {
		return "", err
	}

	parts = append(parts, name)

	for {
		t, ok := p.peek()
		if !ok || t.kind != tokPunct || t.text != "." {
			break
		}

		if _, err := p.next(); err != nil {
			return "", err
		}

		name, err := p.expectIdent()
		if err != nil {
			return "", err
		}

		parts = append(parts, name)
	}

	if err := p.expectPunct(";"); err != nil {
		return "", err
	}

	return strings.Join(parts, "."), nil
}

func parseMessage(p *parser) (Message, error) {
	name, err := p.expectIdent()
	if err != nil {
		return Message{}, err
	}

	if err := p.expectPunct("{"); err != nil {
		return Message{}, err
	}

	msg := Message{Name: name}

	for {
		t, ok := p.peek()
		if !ok {
			return Message{}, fmt.Errorf("parsing message %q: unterminated message body", name)
		}

		if t.kind == tokPunct && t.text == "}" {
			if _, err := p.next(); err != nil {
				return Message{}, err
			}

			break
		}

		field, err := parseField(p, name)
		if err != nil {
			return Message{}, err
		}

		msg.Fields = append(msg.Fields, field)
	}

	return msg, nil
}

func parseField(p *parser, messageName string) (Field, error) {
	t, err := p.next()
	if err != nil {
		return Field{}, err
	}

	if t.kind != tokIdent {
		return Field{}, fmt.Errorf("parsing message %q: expected a field type, got %q", messageName, t.text)
	}

	switch t.text {
	case "repeated":
		return Field{}, fmt.Errorf("parsing message %q: repeated fields are not supported", messageName)
	case "map":
		return Field{}, fmt.Errorf("parsing message %q: map fields are not supported", messageName)
	case "oneof":
		return Field{}, fmt.Errorf("parsing message %q: oneof is not supported", messageName)
	case "message":
		return Field{}, fmt.Errorf("parsing message %q: nested messages are not supported", messageName)
	case "enum":
		return Field{}, fmt.Errorf("parsing message %q: nested enums are not supported", messageName)
	case "reserved":
		return Field{}, fmt.Errorf("parsing message %q: reserved is not supported", messageName)
	}

	typeName := t.text

	if next, ok := p.peek(); ok && next.kind == tokPunct && next.text == "." {
		return Field{}, fmt.Errorf("parsing message %q: qualified field type %q is not supported, inline every message in one file", messageName, typeName)
	}

	fieldName, err := p.expectIdent()
	if err != nil {
		return Field{}, err
	}

	if err := p.expectPunct("="); err != nil {
		return Field{}, err
	}

	numTok, err := p.next()
	if err != nil {
		return Field{}, err
	}

	if numTok.kind != tokNum {
		return Field{}, fmt.Errorf("parsing field %q of message %q: expected a field number, got %q", fieldName, messageName, numTok.text)
	}

	number, err := strconv.Atoi(numTok.text)
	if err != nil {
		return Field{}, fmt.Errorf("parsing field %q of message %q: %w", fieldName, messageName, err)
	}

	if err := skipFieldOptions(p); err != nil {
		return Field{}, err
	}

	if err := p.expectPunct(";"); err != nil {
		return Field{}, err
	}

	field := Field{Name: fieldName, Number: number}

	if scalarTypes[typeName] {
		field.Kind = FieldScalar
		field.Scalar = typeName
	} else {
		field.Kind = FieldMessage
		field.Message = typeName
	}

	return field, nil
}

func skipFieldOptions(p *parser) error {
	t, ok := p.peek()
	if !ok || t.kind != tokPunct || t.text != "[" {
		return nil
	}

	depth := 0

	for {
		t, err := p.next()
		if err != nil {
			return err
		}

		if t.kind == tokPunct && t.text == "[" {
			depth++
		}

		if t.kind == tokPunct && t.text == "]" {
			depth--
			if depth == 0 {
				return nil
			}
		}
	}
}

func parseService(p *parser) (Service, error) {
	name, err := p.expectIdent()
	if err != nil {
		return Service{}, err
	}

	if err := p.expectPunct("{"); err != nil {
		return Service{}, err
	}

	svc := Service{Name: name}

	for {
		t, ok := p.peek()
		if !ok {
			return Service{}, fmt.Errorf("parsing service %q: unterminated service body", name)
		}

		if t.kind == tokPunct && t.text == "}" {
			if _, err := p.next(); err != nil {
				return Service{}, err
			}

			break
		}

		rpc, err := parseRpc(p, name)
		if err != nil {
			return Service{}, err
		}

		svc.Rpcs = append(svc.Rpcs, rpc)
	}

	return svc, nil
}

func parseRpc(p *parser, serviceName string) (Rpc, error) {
	kw, err := p.expectIdent()
	if err != nil {
		return Rpc{}, err
	}

	if kw != "rpc" {
		return Rpc{}, fmt.Errorf("parsing service %q: expected rpc, got %q", serviceName, kw)
	}

	name, err := p.expectIdent()
	if err != nil {
		return Rpc{}, err
	}

	if err := p.expectPunct("("); err != nil {
		return Rpc{}, err
	}

	request, err := parseRpcType(p, serviceName, name)
	if err != nil {
		return Rpc{}, err
	}

	if err := p.expectPunct(")"); err != nil {
		return Rpc{}, err
	}

	returns, err := p.expectIdent()
	if err != nil {
		return Rpc{}, err
	}

	if returns != "returns" {
		return Rpc{}, fmt.Errorf("parsing rpc %q of service %q: expected returns, got %q", name, serviceName, returns)
	}

	if err := p.expectPunct("("); err != nil {
		return Rpc{}, err
	}

	response, err := parseRpcType(p, serviceName, name)
	if err != nil {
		return Rpc{}, err
	}

	if err := p.expectPunct(")"); err != nil {
		return Rpc{}, err
	}

	if err := finishRpc(p); err != nil {
		return Rpc{}, err
	}

	return Rpc{Name: name, Request: request, Response: response}, nil
}

func parseRpcType(p *parser, serviceName, rpcName string) (string, error) {
	t, err := p.next()
	if err != nil {
		return "", err
	}

	if t.kind != tokIdent {
		return "", fmt.Errorf("parsing rpc %q of service %q: expected a message type, got %q", rpcName, serviceName, t.text)
	}

	if t.text == "stream" {
		return "", fmt.Errorf("parsing rpc %q of service %q: streaming is not supported", rpcName, serviceName)
	}

	typeName := t.text

	if next, ok := p.peek(); ok && next.kind == tokPunct && next.text == "." {
		return "", fmt.Errorf("parsing rpc %q of service %q: qualified type %q is not supported, inline every message in one file", rpcName, serviceName, typeName)
	}

	return typeName, nil
}

func finishRpc(p *parser) error {
	t, ok := p.peek()
	if !ok {
		return fmt.Errorf("parsing the proto document: unexpected end of input in an rpc declaration")
	}

	if t.kind == tokPunct && t.text == ";" {
		_, err := p.next()

		return err
	}

	if t.kind == tokPunct && t.text == "{" {
		if _, err := p.next(); err != nil {
			return err
		}

		return p.expectPunct("}")
	}

	return fmt.Errorf("parsing the proto document: expected ; or {} after an rpc declaration, got %q", t.text)
}

func validateReferences(spec *Spec) error {
	for _, m := range spec.Messages {
		for _, f := range m.Fields {
			if f.Kind != FieldMessage {
				continue
			}

			if _, ok := messageByName(spec.Messages, f.Message); !ok {
				return fmt.Errorf("reading message %q: field %q references undefined message %q", m.Name, f.Name, f.Message)
			}
		}
	}

	for _, s := range spec.Services {
		for _, r := range s.Rpcs {
			if _, ok := messageByName(spec.Messages, r.Request); !ok {
				return fmt.Errorf("reading service %q: rpc %q references undefined message %q", s.Name, r.Name, r.Request)
			}

			if _, ok := messageByName(spec.Messages, r.Response); !ok {
				return fmt.Errorf("reading service %q: rpc %q references undefined message %q", s.Name, r.Name, r.Response)
			}
		}
	}

	return nil
}
