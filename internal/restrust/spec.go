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

package restrust

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/rustname"
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

type portRef struct {
	Declared bool
	Kind     string
	Name     string
	Methods  []portMethod
}

type portMethod struct {
	Name    string `json:"name"`
	Request string `json:"request"`
	Reply   string `json:"reply"`
}

func (p *portRef) UnmarshalJSON(raw []byte) error {
	var name string
	if err := json.Unmarshal(raw, &name); err == nil {
		p.Name = name

		return nil
	}

	var declared struct {
		Kind    string       `json:"kind"`
		Name    string       `json:"name"`
		Methods []portMethod `json:"methods"`
	}

	if err := json.Unmarshal(raw, &declared); err != nil {
		return fmt.Errorf("reading an x-ports entry: an entry is either a port name or an object naming kind, name and methods: %w", err)
	}

	p.Declared = true
	p.Kind = declared.Kind
	p.Name = declared.Name
	p.Methods = declared.Methods

	return nil
}

type operation struct {
	OperationID string    `json:"operationId"`
	Summary     string    `json:"summary"`
	Controller  string    `json:"x-controller"`
	Ports       []portRef `json:"x-ports"`
	Auth        string    `json:"x-auth"`
	Stream      string    `json:"x-stream"`
	Parameters  []struct {
		Name     string `json:"name"`
		In       string `json:"in"`
		Required bool   `json:"required"`
		Schema   schema `json:"schema"`
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

type QueryParam struct {
	Name     string
	Ident    string
	Kind     string
	Required bool
}

type HandMethod struct {
	Name    string
	Ident   string
	Request string
	Reply   string
}

type HandPort struct {
	Name    string
	Snake   string
	Methods []HandMethod
}

type Operation struct {
	ID            string
	Ident         string
	Summary       string
	Method        string
	MethodLower   string
	Path          string
	Params        []Param
	Query         []QueryParam
	Body          string
	Response      string
	Status        int
	InvalidStatus int
	Controller    string
	Ports         []string
	Auth          bool
	Stream        bool
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
	Events      []TypeDef
	HandPorts   []HandPort
	Controllers []Controller
	Operations  []Operation
	Auth        bool
}

var methods = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}

const forgeDevSpecSchema = "Spec"

const AuthBearer = "bearer"

const StreamEvents = "events"

const StorePortSuffix = "Store"

const SubscribePortSuffix = "Subscribe"

const HandPortKind = "hand"

const eventStreamContent = "text/event-stream"

const jsonContent = "application/json"

func checkName(what, name string) error {
	if rustname.IsSnakeIdent(rustname.Snake(name)) {
		return nil
	}

	return fmt.Errorf("reading %s %q: its snake form %q is not a name Rust or SQL can spell, use letters, digits and underscores and start with a letter", what, name, rustname.Snake(name))
}

var pathParamPattern = regexp.MustCompile(`\{([^}]+)\}`)

var pascalIdentPattern = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)

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

	operations, hands, err := parseOperations(parsed.Paths, types, stores)
	if err != nil {
		return nil, err
	}

	events := eventTypes(operations, types)
	auth := false

	for _, op := range operations {
		auth = auth || op.Auth
	}

	return &Spec{
		Types:       types,
		Stores:      stores,
		Events:      events,
		HandPorts:   hands,
		Controllers: groupControllers(operations),
		Operations:  operations,
		Auth:        auth,
	}, nil
}

func eventTypes(operations []Operation, types []TypeDef) []TypeDef {
	streamed := map[string]bool{}

	for _, op := range operations {
		if op.Stream {
			streamed[op.Response] = true
		}
	}

	events := []TypeDef{}

	for _, t := range types {
		if streamed[t.Name] {
			events = append(events, t)
		}
	}

	return events
}

func parseTypes(schemas map[string]schema) ([]TypeDef, error) {
	names := sortedKeys(schemas)
	types := make([]TypeDef, 0, len(names))

	for _, name := range names {
		if name == forgeDevSpecSchema {
			continue
		}

		if err := checkName("schema", name); err != nil {
			return nil, err
		}

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

		types = append(types, TypeDef{Name: name, Snake: rustname.Snake(name), Store: s.Store, Fields: fields})
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
		if err := checkName("property", name); err != nil {
			return nil, fmt.Errorf("reading schema %q: %w", typeName, err)
		}

		ft, err := parseFieldType(s.Properties[name], schemas)
		if err != nil {
			return nil, fmt.Errorf("reading property %q of schema %q: %w", name, typeName, err)
		}

		ident := rustname.RustIdent(name)

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

func parseOperations(paths map[string]map[string]json.RawMessage, types, stores []TypeDef) ([]Operation, []HandPort, error) {
	if len(paths) == 0 {
		return nil, nil, fmt.Errorf("reading the OpenAPI paths: a service's surface is its paths, and there are none")
	}

	schemas := map[string]schema{}
	for _, t := range types {
		schemas[t.Name] = schema{}
	}

	storeNames := map[string]bool{}
	for _, s := range stores {
		storeNames[s.Name+"Store"] = true
	}

	hands, err := collectHandPorts(paths, schemas, storeNames)
	if err != nil {
		return nil, nil, err
	}

	handNames := map[string]bool{}
	for _, h := range hands {
		handNames[h.Name] = true
	}

	ops := []Operation{}

	for path, item := range paths {
		for _, method := range methods {
			raw, ok := item[method]
			if !ok {
				continue
			}

			op, err := parseOperation(path, method, raw, schemas, storeNames, handNames)
			if err != nil {
				return nil, nil, err
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

	return ops, hands, nil
}

func collectHandPorts(paths map[string]map[string]json.RawMessage, schemas map[string]schema, storeNames map[string]bool) ([]HandPort, error) {
	byName := map[string]HandPort{}

	for _, path := range sortedKeys(paths) {
		for _, method := range methods {
			raw, ok := paths[path][method]
			if !ok {
				continue
			}

			where := fmt.Sprintf("%s %s", strings.ToUpper(method), path)

			var op operation
			if err := json.Unmarshal(raw, &op); err != nil {
				return nil, fmt.Errorf("reading %s: %w", where, err)
			}

			for _, ref := range op.Ports {
				if !ref.Declared {
					continue
				}

				hand, err := parseHandPort(where, ref, schemas, storeNames)
				if err != nil {
					return nil, err
				}

				known, seen := byName[hand.Name]
				if seen && !reflect.DeepEqual(known, hand) {
					return nil, fmt.Errorf("reading %s: x-ports declares hand port %q a second time with different methods, declare it once and name it by its name everywhere else", where, hand.Name)
				}

				byName[hand.Name] = hand
			}
		}
	}

	hands := make([]HandPort, 0, len(byName))
	for _, name := range sortedKeys(byName) {
		hands = append(hands, byName[name])
	}

	return hands, nil
}

func parseHandPort(where string, ref portRef, schemas map[string]schema, storeNames map[string]bool) (HandPort, error) {
	if ref.Kind != HandPortKind {
		return HandPort{}, fmt.Errorf("reading %s: x-ports declares a port of kind %q, the only declared kind is %q, every other entry is the name of a store or subscribe port", where, ref.Kind, HandPortKind)
	}

	if !pascalIdentPattern.MatchString(ref.Name) {
		return HandPort{}, fmt.Errorf("reading %s: hand port %q is not a Pascal case Rust ident, a port name starts with an upper case letter and holds letters and digits", where, ref.Name)
	}

	if storeNames[ref.Name] || strings.HasSuffix(ref.Name, SubscribePortSuffix) {
		return HandPort{}, fmt.Errorf("reading %s: hand port %q takes the name of a store or subscribe port the engine already emits, name it something else", where, ref.Name)
	}

	if len(ref.Methods) == 0 {
		return HandPort{}, fmt.Errorf("reading %s: hand port %q declares no method, a port the controller consumes has at least one", where, ref.Name)
	}

	hand := HandPort{Name: ref.Name, Snake: rustname.Snake(ref.Name)}
	seen := map[string]bool{}

	for _, m := range ref.Methods {
		if err := checkName("hand port method", m.Name); err != nil {
			return HandPort{}, fmt.Errorf("reading %s: hand port %q: %w", where, ref.Name, err)
		}

		if seen[m.Name] {
			return HandPort{}, fmt.Errorf("reading %s: hand port %q declares method %q twice", where, ref.Name, m.Name)
		}

		seen[m.Name] = true

		for label, name := range map[string]string{"request": m.Request, "reply": m.Reply} {
			if name == "" {
				continue
			}

			if _, ok := schemas[name]; !ok {
				return HandPort{}, fmt.Errorf("reading %s: hand port %q method %q names %s %q, which is not a schema of components.schemas", where, ref.Name, m.Name, label, name)
			}
		}

		hand.Methods = append(hand.Methods, HandMethod{
			Name:    m.Name,
			Ident:   rustname.Snake(m.Name),
			Request: m.Request,
			Reply:   m.Reply,
		})
	}

	return hand, nil
}

func parseOperation(path, method string, raw json.RawMessage, schemas map[string]schema, storeNames, handNames map[string]bool) (Operation, error) {
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

	if err := checkName("operationId", op.OperationID); err != nil {
		return Operation{}, fmt.Errorf("reading %s: %w", where, err)
	}

	if err := checkName("x-controller", op.Controller); err != nil {
		return Operation{}, fmt.Errorf("reading %s: %w", where, err)
	}

	auth, err := parseAuth(where, op)
	if err != nil {
		return Operation{}, err
	}

	stream, err := parseStream(where, method, op)
	if err != nil {
		return Operation{}, err
	}

	params, query, err := parseParams(where, path, op)
	if err != nil {
		return Operation{}, err
	}

	body, err := parseBody(where, op, schemas)
	if err != nil {
		return Operation{}, err
	}

	if stream && body != "" {
		return Operation{}, fmt.Errorf("reading %s: an x-stream operation takes no request body", where)
	}

	response, status, err := parseResponse(where, op, schemas, stream)
	if err != nil {
		return Operation{}, err
	}

	if stream && response == "" {
		return Operation{}, fmt.Errorf("reading %s: an x-stream operation needs a 2xx response with a %s schema, it is the event type", where, eventStreamContent)
	}

	ports, err := parsePorts(where, op, stream, response, storeNames, handNames)
	if err != nil {
		return Operation{}, err
	}

	return Operation{
		ID:            op.OperationID,
		Ident:         rustname.Snake(op.OperationID),
		Summary:       op.Summary,
		Method:        strings.ToUpper(method),
		MethodLower:   method,
		Path:          path,
		Params:        params,
		Query:         query,
		Body:          body,
		Response:      response,
		Status:        status,
		InvalidStatus: invalidStatus(op),
		Controller:    op.Controller,
		Ports:         ports,
		Auth:          auth,
		Stream:        stream,
	}, nil
}

func parseAuth(where string, op operation) (bool, error) {
	switch op.Auth {
	case "":
		return false, nil
	case AuthBearer:
		return true, nil
	default:
		return false, fmt.Errorf("reading %s: x-auth is %q, the only value is %q", where, op.Auth, AuthBearer)
	}
}

func parseStream(where, method string, op operation) (bool, error) {
	switch op.Stream {
	case "":
		return false, nil
	case StreamEvents:
		if method != "get" {
			return false, fmt.Errorf("reading %s: x-stream is only allowed on a GET operation", where)
		}

		return true, nil
	default:
		return false, fmt.Errorf("reading %s: x-stream is %q, the only value is %q", where, op.Stream, StreamEvents)
	}
}

func parsePorts(where string, op operation, stream bool, response string, storeNames, handNames map[string]bool) ([]string, error) {
	subscribe := ""
	if stream {
		subscribe = response + SubscribePortSuffix
	}

	ports := make([]string, 0, len(op.Ports))
	for _, ref := range op.Ports {
		ports = append(ports, ref.Name)
	}

	for _, port := range ports {
		if storeNames[port] || handNames[port] || (subscribe != "" && port == subscribe) {
			continue
		}

		if strings.HasSuffix(port, SubscribePortSuffix) {
			return nil, fmt.Errorf("reading %s: x-ports names %q, a subscribe port is %s of an x-stream operation's response", where, port, "<Event>"+SubscribePortSuffix)
		}

		return nil, fmt.Errorf("reading %s: x-ports names %q, which is not <Name>Store of an x-store schema and no operation declares it as a hand port", where, port)
	}

	if subscribe != "" {
		ports = union(ports, []string{subscribe})
	}

	sort.Strings(ports)

	return ports, nil
}

func parseParams(where, path string, op operation) ([]Param, []QueryParam, error) {
	declared := map[string]Param{}
	query := []QueryParam{}
	seenQuery := map[string]bool{}

	for _, p := range op.Parameters {
		if p.In != "path" && p.In != "query" {
			continue
		}

		kind := string(p.Schema.Type)
		if kind != "string" && kind != "integer" {
			return nil, nil, fmt.Errorf("reading %s: %s parameter %q must be a string or an integer, got %q", where, p.In, p.Name, kind)
		}

		if err := checkName(p.In+" parameter", p.Name); err != nil {
			return nil, nil, fmt.Errorf("reading %s: %w", where, err)
		}

		if p.In == "path" {
			declared[p.Name] = Param{Name: p.Name, Ident: rustname.Snake(p.Name), Kind: kind}

			continue
		}

		if seenQuery[p.Name] {
			return nil, nil, fmt.Errorf("reading %s: query parameter %q is declared twice", where, p.Name)
		}

		seenQuery[p.Name] = true

		query = append(query, QueryParam{
			Name:     p.Name,
			Ident:    rustname.Snake(p.Name),
			Kind:     kind,
			Required: p.Required,
		})
	}

	params := []Param{}

	for _, match := range pathParamPattern.FindAllStringSubmatch(path, -1) {
		p, ok := declared[match[1]]
		if !ok {
			return nil, nil, fmt.Errorf("reading %s: path parameter %q is not declared", where, match[1])
		}

		params = append(params, p)
	}

	for _, q := range query {
		if _, taken := declared[q.Name]; taken {
			return nil, nil, fmt.Errorf("reading %s: %q is declared both in path and in query, one name is one argument", where, q.Name)
		}
	}

	return params, query, nil
}

func parseBody(where string, op operation, schemas map[string]schema) (string, error) {
	c, ok := op.RequestBody.Content[jsonContent]
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

func parseResponse(where string, op operation, schemas map[string]schema, stream bool) (string, int, error) {
	codes := sortedKeys(op.Responses)

	contentType := jsonContent
	if stream {
		contentType = eventStreamContent
	}

	for _, code := range codes {
		var status int
		if _, err := fmt.Sscanf(code, "%d", &status); err != nil || status < 200 || status > 299 {
			continue
		}

		c, ok := op.Responses[code].Content[contentType]
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

func invalidStatus(op operation) int {
	for _, candidate := range []struct {
		code   string
		status int
	}{{"422", 422}, {"400", 400}} {
		if _, ok := op.Responses[candidate.code]; ok {
			return candidate.status
		}
	}

	return 400
}

func groupControllers(ops []Operation) []Controller {
	byName := map[string]*Controller{}

	for _, op := range ops {
		c, ok := byName[op.Controller]
		if !ok {
			c = &Controller{Name: op.Controller, Snake: rustname.Snake(op.Controller), Pascal: rustname.Pascal(op.Controller)}
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
