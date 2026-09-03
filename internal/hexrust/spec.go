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

package hexrust

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"sigs.k8s.io/yaml"
)

type document struct {
	Paths      map[string]map[string]json.RawMessage `json:"paths"`
	Components struct {
		Schemas map[string]schema `json:"schemas"`
	} `json:"components"`
}

type schema struct {
	Type       typeName          `json:"type"`
	Ref        string            `json:"$ref"`
	Store      bool              `json:"x-store"`
	Required   []string          `json:"required"`
	Properties map[string]schema `json:"properties"`
	Items      *schema           `json:"items"`
}

type typeName string

func (t *typeName) UnmarshalJSON(raw []byte) error {
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		*t = typeName(single)

		return nil
	}

	var many []string
	if err := json.Unmarshal(raw, &many); err != nil {
		return fmt.Errorf("reading a schema type: %w", err)
	}

	for _, name := range many {
		if name != "null" {
			*t = typeName(name)

			return nil
		}
	}

	return nil
}

type content struct {
	Schema schema `json:"schema"`
}

type operation struct {
	OperationID string   `json:"operationId"`
	Summary     string   `json:"summary"`
	Controller  string   `json:"x-controller"`
	Ports       []string `json:"x-ports"`
	Parameters  []struct {
		Name   string `json:"name"`
		In     string `json:"in"`
		Schema schema `json:"schema"`
	} `json:"parameters"`
	RequestBody struct {
		Content map[string]content `json:"content"`
	} `json:"requestBody"`
	Responses map[string]struct {
		Content map[string]content `json:"content"`
	} `json:"responses"`
}

type Field struct {
	Name     string
	Ident    string
	Renamed  bool
	Optional bool
	Type     fieldType
}

type fieldType struct {
	Kind string
	Ref  string
	Item *fieldType
}

type TypeDef struct {
	Name   string
	Snake  string
	Store  bool
	Fields []Field
}

type Param struct {
	Name  string
	Ident string
	Kind  string
}

type Operation struct {
	ID          string
	Ident       string
	Summary     string
	Method      string
	MethodLower string
	Path        string
	Params      []Param
	Body        string
	Response    string
	Status      int
	Controller  string
	Ports       []string
}

type Controller struct {
	Name       string
	Snake      string
	Pascal     string
	Ports      []string
	Operations []Operation
}

type Spec struct {
	Types       []TypeDef
	Stores      []TypeDef
	Controllers []Controller
	Operations  []Operation
}

var methods = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}

var pathParamPattern = regexp.MustCompile(`\{([^}]+)\}`)

func Parse(doc []byte) (*Spec, error) {
	var parsed document
	if err := yaml.Unmarshal(doc, &parsed); err != nil {
		return nil, fmt.Errorf("parsing the OpenAPI document: %w", err)
	}

	types, err := parseTypes(parsed.Components.Schemas)
	if err != nil {
		return nil, err
	}

	stores := []TypeDef{}
	for _, t := range types {
		if t.Store {
			stores = append(stores, t)
		}
	}

	operations, err := parseOperations(parsed.Paths, types, stores)
	if err != nil {
		return nil, err
	}

	return &Spec{
		Types:       types,
		Stores:      stores,
		Controllers: groupControllers(operations),
		Operations:  operations,
	}, nil
}

func parseTypes(schemas map[string]schema) ([]TypeDef, error) {
	names := sortedKeys(schemas)
	types := make([]TypeDef, 0, len(names))

	for _, name := range names {
		s := schemas[name]

		if s.Type != "object" && s.Type != "" {
			return nil, fmt.Errorf("reading schema %q: only object schemas become types, got %q", name, s.Type)
		}

		fields, err := parseFields(name, s, schemas)
		if err != nil {
			return nil, err
		}

		if s.Store && !hasStringID(fields) {
			return nil, fmt.Errorf("reading schema %q: an x-store schema needs a required string property named id", name)
		}

		types = append(types, TypeDef{Name: name, Snake: Snake(name), Store: s.Store, Fields: fields})
	}

	return types, nil
}

func hasStringID(fields []Field) bool {
	for _, f := range fields {
		if f.Name == "id" && !f.Optional && f.Type.Kind == "string" {
			return true
		}
	}

	return false
}

func parseFields(typeName string, s schema, schemas map[string]schema) ([]Field, error) {
	required := map[string]bool{}
	for _, r := range s.Required {
		required[r] = true
	}

	names := sortedKeys(s.Properties)
	fields := make([]Field, 0, len(names))

	for _, name := range names {
		ft, err := parseFieldType(s.Properties[name], schemas)
		if err != nil {
			return nil, fmt.Errorf("reading property %q of schema %q: %w", name, typeName, err)
		}

		ident := Snake(name)
		if rustKeywords[ident] {
			ident = "r#" + ident
		}

		fields = append(fields, Field{
			Name:     name,
			Ident:    ident,
			Renamed:  ident != name,
			Optional: !required[name],
			Type:     ft,
		})
	}

	return fields, nil
}

func parseFieldType(s schema, schemas map[string]schema) (fieldType, error) {
	if s.Ref != "" {
		name, err := refName(s.Ref, schemas)
		if err != nil {
			return fieldType{}, err
		}

		return fieldType{Kind: "ref", Ref: name}, nil
	}

	switch s.Type {
	case "string", "integer", "number", "boolean":
		return fieldType{Kind: string(s.Type)}, nil
	case "array":
		if s.Items == nil {
			return fieldType{}, fmt.Errorf("an array needs items")
		}

		item, err := parseFieldType(*s.Items, schemas)
		if err != nil {
			return fieldType{}, err
		}

		return fieldType{Kind: "array", Item: &item}, nil
	default:
		return fieldType{Kind: "value"}, nil
	}
}

func refName(ref string, schemas map[string]schema) (string, error) {
	const prefix = "#/components/schemas/"

	if !strings.HasPrefix(ref, prefix) {
		return "", fmt.Errorf("resolving %q: only #/components/schemas refs are supported", ref)
	}

	name := strings.TrimPrefix(ref, prefix)
	if _, ok := schemas[name]; !ok {
		return "", fmt.Errorf("resolving %q: no such schema", ref)
	}

	return name, nil
}

func parseOperations(paths map[string]map[string]json.RawMessage, types, stores []TypeDef) ([]Operation, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("reading the OpenAPI paths: a service's surface is its paths, and there are none")
	}

	schemas := map[string]schema{}
	for _, t := range types {
		schemas[t.Name] = schema{}
	}

	storeNames := map[string]bool{}
	for _, s := range stores {
		storeNames[s.Name+"Store"] = true
	}

	ops := []Operation{}

	for path, item := range paths {
		for _, method := range methods {
			raw, ok := item[method]
			if !ok {
				continue
			}

			op, err := parseOperation(path, method, raw, schemas, storeNames)
			if err != nil {
				return nil, err
			}

			ops = append(ops, op)
		}
	}

	sort.Slice(ops, func(i, j int) bool {
		if ops[i].Path != ops[j].Path {
			return ops[i].Path < ops[j].Path
		}

		return ops[i].Method < ops[j].Method
	})

	return ops, nil
}

func parseOperation(path, method string, raw json.RawMessage, schemas map[string]schema, storeNames map[string]bool) (Operation, error) {
	where := fmt.Sprintf("%s %s", strings.ToUpper(method), path)

	var op operation
	if err := json.Unmarshal(raw, &op); err != nil {
		return Operation{}, fmt.Errorf("reading %s: %w", where, err)
	}

	if op.OperationID == "" {
		return Operation{}, fmt.Errorf("reading %s: operationId is required, it names the controller method", where)
	}

	if op.Controller == "" {
		return Operation{}, fmt.Errorf("reading %s: x-controller is required, it names the controller", where)
	}

	ports := append([]string{}, op.Ports...)
	sort.Strings(ports)

	for _, port := range ports {
		if !storeNames[port] {
			return Operation{}, fmt.Errorf("reading %s: x-ports names %q, which is not <Name>Store of an x-store schema", where, port)
		}
	}

	params, err := parseParams(where, path, op)
	if err != nil {
		return Operation{}, err
	}

	body, err := parseBody(where, op, schemas)
	if err != nil {
		return Operation{}, err
	}

	response, status, err := parseResponse(where, op, schemas)
	if err != nil {
		return Operation{}, err
	}

	return Operation{
		ID:          op.OperationID,
		Ident:       Snake(op.OperationID),
		Summary:     op.Summary,
		Method:      strings.ToUpper(method),
		MethodLower: method,
		Path:        path,
		Params:      params,
		Body:        body,
		Response:    response,
		Status:      status,
		Controller:  op.Controller,
		Ports:       ports,
	}, nil
}

func parseParams(where, path string, op operation) ([]Param, error) {
	declared := map[string]Param{}

	for _, p := range op.Parameters {
		if p.In != "path" {
			continue
		}

		kind := string(p.Schema.Type)
		if kind != "string" && kind != "integer" {
			return nil, fmt.Errorf("reading %s: path parameter %q must be a string or an integer, got %q", where, p.Name, kind)
		}

		declared[p.Name] = Param{Name: p.Name, Ident: Snake(p.Name), Kind: kind}
	}

	params := []Param{}

	for _, match := range pathParamPattern.FindAllStringSubmatch(path, -1) {
		p, ok := declared[match[1]]
		if !ok {
			return nil, fmt.Errorf("reading %s: path parameter %q is not declared", where, match[1])
		}

		params = append(params, p)
	}

	return params, nil
}

func parseBody(where string, op operation, schemas map[string]schema) (string, error) {
	c, ok := op.RequestBody.Content["application/json"]
	if !ok {
		return "", nil
	}

	if c.Schema.Ref == "" {
		return "", fmt.Errorf("reading %s: the request body must $ref a component schema", where)
	}

	name, err := refName(c.Schema.Ref, schemas)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", where, err)
	}

	return name, nil
}

func parseResponse(where string, op operation, schemas map[string]schema) (string, int, error) {
	codes := sortedKeys(op.Responses)

	for _, code := range codes {
		var status int
		if _, err := fmt.Sscanf(code, "%d", &status); err != nil || status < 200 || status > 299 {
			continue
		}

		c, ok := op.Responses[code].Content["application/json"]
		if !ok {
			return "", status, nil
		}

		if c.Schema.Ref == "" {
			return "", 0, fmt.Errorf("reading %s: the %s response must $ref a component schema", where, code)
		}

		name, err := refName(c.Schema.Ref, schemas)
		if err != nil {
			return "", 0, fmt.Errorf("reading %s: %w", where, err)
		}

		return name, status, nil
	}

	return "", 0, fmt.Errorf("reading %s: a 2xx response is required", where)
}

func groupControllers(ops []Operation) []Controller {
	byName := map[string]*Controller{}

	for _, op := range ops {
		c, ok := byName[op.Controller]
		if !ok {
			c = &Controller{Name: op.Controller, Snake: Snake(op.Controller), Pascal: Pascal(op.Controller)}
			byName[op.Controller] = c
		}

		c.Operations = append(c.Operations, op)
		c.Ports = union(c.Ports, op.Ports)
	}

	controllers := make([]Controller, 0, len(byName))
	for _, name := range sortedKeys(byName) {
		controllers = append(controllers, *byName[name])
	}

	return controllers
}

func union(a, b []string) []string {
	seen := map[string]bool{}
	out := []string{}

	for _, s := range append(append([]string{}, a...), b...) {
		if seen[s] {
			continue
		}

		seen[s] = true
		out = append(out, s)
	}

	sort.Strings(out)

	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}

var rustKeywords = map[string]bool{
	"as": true, "break": true, "const": true, "continue": true, "crate": true, "else": true, "enum": true,
	"extern": true, "false": true, "fn": true, "for": true, "if": true, "impl": true, "in": true, "let": true,
	"loop": true, "match": true, "mod": true, "move": true, "mut": true, "pub": true, "ref": true, "return": true,
	"self": true, "static": true, "struct": true, "super": true, "trait": true, "true": true, "type": true,
	"unsafe": true, "use": true, "where": true, "while": true, "async": true, "await": true, "dyn": true,
}
