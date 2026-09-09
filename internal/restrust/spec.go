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
	Store      json.RawMessage   `json:"x-store"`
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
	Instant  string
	Span     string
	Adapters *[]string
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
		Kind     string       `json:"kind"`
		Name     string       `json:"name"`
		Methods  []portMethod `json:"methods"`
		Instant  string       `json:"instant"`
		Span     string       `json:"span"`
		Adapters *[]string    `json:"adapters"`
	}

	if err := json.Unmarshal(raw, &declared); err != nil {
		return fmt.Errorf("reading an x-ports entry: an entry is either a port name or an object naming kind, name and what that kind needs: %w", err)
	}

	p.Declared = true
	p.Kind = declared.Kind
	p.Name = declared.Name
	p.Methods = declared.Methods
	p.Instant = declared.Instant
	p.Span = declared.Span
	p.Adapters = declared.Adapters

	return nil
}

type operation struct {
	OperationID string    `json:"operationId"`
	Summary     string    `json:"summary"`
	Controller  string    `json:"x-controller"`
	Ports       []portRef `json:"x-ports"`
	Auth        string          `json:"x-auth"`
	Stream      json.RawMessage `json:"x-stream"`
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
	Store  *Store
	Fields []Field
}

type Store struct {
	Key      string
	KeyIdent string
	Lookups  []Lookup
	Adapters []string
}

type Lookup struct {
	By      string
	Ident   string
	Answers string
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
	Name         string
	Snake        string
	Kind         string
	Instant      string
	InstantField string
	Span         string
	SpanField    string
	Adapters     []string
	Methods      []HandMethod
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
	Controller     string
	Ports          []string
	Auth           bool
	Stream         bool
	StreamFrom     string
	StreamAdapters []string
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
	Events      []Event
	HandPorts   []HandPort
	Controllers []Controller
	Operations  []Operation
	Auth        bool
}

type Event struct {
	TypeDef

	From     TypeDef
	Adapters []string
}

var methods = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}

const forgeDevSpecSchema = "Spec"

const AuthBearer = "bearer"

const FeedAdapterMemory = "memory"

func FeedAdapterKinds() []string {
	return []string{FeedAdapterMemory}
}

const StorePortSuffix = "Store"

const SubscribePortSuffix = "Subscribe"

const ClockPortKind = "clock"

const (
	ClockAdapterMemory = "memory"
	ClockAdapterSystem = "system"
)

func PortKinds() []string {
	return []string{ClockPortKind}
}

func ClockAdapterKinds() []string {
	return []string{ClockAdapterMemory, ClockAdapterSystem}
}

const (
	LookupOne  = "one"
	LookupPage = "page"
)

const (
	StoreAdapterSqlite = "sqlite"
	StoreAdapterMemory = "memory"
)

func LookupWords() []string {
	return []string{LookupOne, LookupPage}
}

func StoreAdapterKinds() []string {
	return []string{StoreAdapterMemory, StoreAdapterSqlite}
}

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
		if t.Store != nil {
			stores = append(stores, t)
		}
	}

	operations, hands, err := parseOperations(parsed.Paths, types, stores)
	if err != nil {
		return nil, err
	}

	events, err := eventTypes(operations, types)
	if err != nil {
		return nil, err
	}

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

func eventTypes(operations []Operation, types []TypeDef) ([]Event, error) {
	byName := map[string]TypeDef{}
	for _, t := range types {
		byName[t.Name] = t
	}

	streamed := map[string]Operation{}

	for _, op := range operations {
		if !op.Stream {
			continue
		}

		known, seen := streamed[op.Response]
		if seen && known.StreamFrom != op.StreamFrom {
			return nil, fmt.Errorf("reading the x-stream operations: %q and %q both carry %q and read it from %q and %q, one event has one source", known.ID, op.ID, op.Response, known.StreamFrom, op.StreamFrom)
		}

		streamed[op.Response] = op
	}

	events := []Event{}

	for _, t := range types {
		op, ok := streamed[t.Name]
		if !ok {
			continue
		}

		events = append(events, Event{TypeDef: t, From: byName[op.StreamFrom], Adapters: op.StreamAdapters})
	}

	return events, nil
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

		store, err := parseStore(name, s.Store, fields)
		if err != nil {
			return nil, err
		}

		types = append(types, TypeDef{Name: name, Snake: rustname.Snake(name), Store: store, Fields: fields})
	}

	return types, nil
}

func parseStore(name string, raw json.RawMessage, fields []Field) (*Store, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	var flag bool
	if err := json.Unmarshal(raw, &flag); err == nil {
		return nil, fmt.Errorf("reading schema %q: x-store is a boolean, it is an object naming key, lookups and adapters", name)
	}

	var declared struct {
		Key      string `json:"key"`
		Lookups  *[]struct {
			By      string `json:"by"`
			Answers string `json:"answers"`
		} `json:"lookups"`
		Adapters *[]string `json:"adapters"`
	}

	if err := json.Unmarshal(raw, &declared); err != nil {
		return nil, fmt.Errorf("reading schema %q: x-store is an object naming key, lookups and adapters: %w", name, err)
	}

	if declared.Key == "" {
		return nil, fmt.Errorf("reading schema %q: x-store names no key, key names the required string property every row is stored under", name)
	}

	if declared.Lookups == nil {
		return nil, fmt.Errorf("reading schema %q: x-store names no lookups, write an empty list when the key is the only way in", name)
	}

	if declared.Adapters == nil {
		return nil, fmt.Errorf("reading schema %q: x-store names no adapters, adapters lists which of %s the engine emits", name, list(StoreAdapterKinds()))
	}

	if err := checkStoreField(name, "key", declared.Key, fields); err != nil {
		return nil, err
	}

	store := &Store{Key: declared.Key, KeyIdent: rustname.RustIdent(declared.Key)}

	seen := map[string]bool{}

	for _, entry := range *declared.Lookups {
		if err := checkStoreField(name, "lookup", entry.By, fields); err != nil {
			return nil, err
		}

		if entry.By == declared.Key {
			return nil, fmt.Errorf("reading schema %q: x-store declares a lookup by %q, which is the key and is always reachable", name, entry.By)
		}

		if seen[entry.By] {
			return nil, fmt.Errorf("reading schema %q: x-store declares a lookup by %q twice", name, entry.By)
		}

		seen[entry.By] = true

		if entry.Answers != LookupOne && entry.Answers != LookupPage {
			return nil, fmt.Errorf("reading schema %q: the lookup by %q answers %q, a lookup answers one of %s", name, entry.By, entry.Answers, list(LookupWords()))
		}

		store.Lookups = append(store.Lookups, Lookup{
			By:      entry.By,
			Ident:   rustname.RustIdent(entry.By),
			Answers: entry.Answers,
		})
	}

	if len(*declared.Adapters) == 0 {
		return nil, fmt.Errorf("reading schema %q: x-store names an empty adapters list, a store nobody can build is a store nobody can use", name)
	}

	for _, kind := range *declared.Adapters {
		if kind != StoreAdapterSqlite && kind != StoreAdapterMemory {
			return nil, fmt.Errorf("reading schema %q: x-store names adapter kind %q, a store adapter is one of %s", name, kind, list(StoreAdapterKinds()))
		}

		store.Adapters = append(store.Adapters, kind)
	}

	return store, nil
}

func checkStoreField(name, what, field string, fields []Field) error {
	for _, f := range fields {
		if f.Name != field {
			continue
		}

		if f.Optional || f.Type.Kind != "string" {
			return fmt.Errorf("reading schema %q: the x-store %s names property %q, which must be a required string property", name, what, field)
		}

		return nil
	}

	return fmt.Errorf("reading schema %q: the x-store %s names property %q, which the schema does not declare", name, what, field)
}

func list(values []string) string {
	return strings.Join(values, ", ")
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
	byName := map[string]TypeDef{}

	for _, t := range types {
		schemas[t.Name] = schema{}
		byName[t.Name] = t
	}

	storeNames := map[string]bool{}
	storeTypes := map[string]TypeDef{}

	for _, s := range stores {
		storeNames[s.Name+"Store"] = true
		storeTypes[s.Name] = s
	}

	hands, err := collectHandPorts(paths, byName, storeNames)
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

			op, err := parseOperation(path, method, raw, schemas, byName, storeTypes, storeNames, handNames)
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

func collectHandPorts(paths map[string]map[string]json.RawMessage, byName map[string]TypeDef, storeNames map[string]bool) ([]HandPort, error) {
	handsByName := map[string]HandPort{}

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

				hand, err := parseHandPort(where, ref, byName, storeNames)
				if err != nil {
					return nil, err
				}

				known, seen := handsByName[hand.Name]
				if seen && !reflect.DeepEqual(known, hand) {
					return nil, fmt.Errorf("reading %s: x-ports declares port %q a second time with a different declaration, declare it once and name it by its name everywhere else", where, hand.Name)
				}

				handsByName[hand.Name] = hand
			}
		}
	}

	hands := make([]HandPort, 0, len(handsByName))
	for _, name := range sortedKeys(handsByName) {
		hands = append(hands, handsByName[name])
	}

	return hands, nil
}

func parseHandPort(where string, ref portRef, byName map[string]TypeDef, storeNames map[string]bool) (HandPort, error) {
	if ref.Kind != ClockPortKind {
		return HandPort{}, fmt.Errorf("reading %s: x-ports declares a port of kind %q, the declared kinds are %s, every other entry is the name of a store or subscribe port", where, ref.Kind, list(PortKinds()))
	}

	return parseClockPort(where, ref, byName, storeNames)
}

func parseClockPort(where string, ref portRef, byName map[string]TypeDef, storeNames map[string]bool) (HandPort, error) {
	if err := checkPortName(where, ClockPortKind, ref.Name, storeNames); err != nil {
		return HandPort{}, err
	}

	if len(ref.Methods) > 0 {
		return HandPort{}, fmt.Errorf("reading %s: clock port %q declares methods, a clock answers now and elapsed and the engine writes both", where, ref.Name)
	}

	instantField, err := clockField(where, ref.Name, "instant", ref.Instant, byName)
	if err != nil {
		return HandPort{}, err
	}

	spanField, err := clockField(where, ref.Name, "span", ref.Span, byName)
	if err != nil {
		return HandPort{}, err
	}

	if ref.Instant == ref.Span {
		return HandPort{}, fmt.Errorf("reading %s: clock port %q names %q as both its instant and its span, a moment and a duration are two schemas", where, ref.Name, ref.Instant)
	}

	if ref.Adapters == nil {
		return HandPort{}, fmt.Errorf("reading %s: clock port %q names no adapters, adapters lists which of %s the engine emits", where, ref.Name, list(ClockAdapterKinds()))
	}

	if len(*ref.Adapters) == 0 {
		return HandPort{}, fmt.Errorf("reading %s: clock port %q names an empty adapters list, a clock nobody can build is a clock nobody can use", where, ref.Name)
	}

	for _, kind := range *ref.Adapters {
		if !holds(ClockAdapterKinds(), kind) {
			return HandPort{}, fmt.Errorf("reading %s: clock port %q names adapter kind %q, a clock adapter is one of %s", where, ref.Name, kind, list(ClockAdapterKinds()))
		}
	}

	return HandPort{
		Name:         ref.Name,
		Snake:        rustname.Snake(ref.Name),
		Kind:         ClockPortKind,
		Instant:      ref.Instant,
		InstantField: instantField,
		Span:         ref.Span,
		SpanField:    spanField,
		Adapters:     *ref.Adapters,
		Methods: []HandMethod{
			{Name: "now", Ident: "now", Reply: ref.Instant},
			{Name: "elapsed", Ident: "elapsed", Request: ref.Instant, Reply: ref.Span},
		},
	}, nil
}

func clockField(where, port, label, name string, byName map[string]TypeDef) (string, error) {
	if name == "" {
		return "", fmt.Errorf("reading %s: clock port %q names no %s, a clock declaration names the two schemas it speaks", where, port, label)
	}

	declared, ok := byName[name]
	if !ok {
		return "", fmt.Errorf("reading %s: clock port %q names %s %q, which is not a schema of components.schemas", where, port, label, name)
	}

	if len(declared.Fields) != 1 {
		return "", fmt.Errorf("reading %s: clock port %q names %s %q, which declares %d properties, a clock reads and writes one number so its %s carries one property and the engine takes that property's name from the schema", where, port, label, name, len(declared.Fields), label)
	}

	field := declared.Fields[0]
	if field.Type.Kind != "integer" || field.Optional {
		return "", fmt.Errorf("reading %s: clock port %q names %s %q, whose one property %q is %s, a clock counts in whole units so that property is a required integer", where, port, label, name, field.Name, describeClockField(field))
	}

	return field.Ident, nil
}

func holds(declared []string, name string) bool {
	for _, entry := range declared {
		if entry == name {
			return true
		}
	}

	return false
}

func describeClockField(field Field) string {
	if field.Optional {
		return "optional and of type " + field.Type.Kind
	}

	return "of type " + field.Type.Kind
}

func checkPortName(where, kind, name string, storeNames map[string]bool) error {
	if !pascalIdentPattern.MatchString(name) {
		return fmt.Errorf("reading %s: %s port %q is not a Pascal case Rust ident, a port name starts with an upper case letter and holds letters and digits", where, kind, name)
	}

	if storeNames[name] || strings.HasSuffix(name, SubscribePortSuffix) {
		return fmt.Errorf("reading %s: %s port %q takes the name of a store or subscribe port the engine already emits, name it something else", where, kind, name)
	}

	return nil
}

func parseOperation(path, method string, raw json.RawMessage, schemas map[string]schema, byName, storeTypes map[string]TypeDef, storeNames, handNames map[string]bool) (Operation, error) {
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

	stream, streamFrom, streamAdapters, err := parseStream(where, method, op, storeTypes)
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

	if stream {
		if err := checkStreamMapping(where, response, streamFrom, byName); err != nil {
			return Operation{}, err
		}
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
		Controller:     op.Controller,
		Ports:          ports,
		Auth:           auth,
		Stream:         stream,
		StreamFrom:     streamFrom.Name,
		StreamAdapters: streamAdapters,
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

func parseStream(where, method string, op operation, stores map[string]TypeDef) (bool, TypeDef, []string, error) {
	if len(op.Stream) == 0 {
		return false, TypeDef{}, nil, nil
	}

	var word string
	if err := json.Unmarshal(op.Stream, &word); err == nil {
		return false, TypeDef{}, nil, fmt.Errorf("reading %s: x-stream is %q, it is an object naming from and adapters", where, word)
	}

	var declared struct {
		From     string    `json:"from"`
		Adapters *[]string `json:"adapters"`
	}

	if err := json.Unmarshal(op.Stream, &declared); err != nil {
		return false, TypeDef{}, nil, fmt.Errorf("reading %s: x-stream is an object naming from and adapters: %w", where, err)
	}

	if method != "get" {
		return false, TypeDef{}, nil, fmt.Errorf("reading %s: x-stream is only allowed on a GET operation", where)
	}

	if declared.From == "" {
		return false, TypeDef{}, nil, fmt.Errorf("reading %s: x-stream names no from, from names the x-store schema whose saves feed this stream", where)
	}

	source, stored := stores[declared.From]
	if !stored {
		return false, TypeDef{}, nil, fmt.Errorf("reading %s: x-stream reads from %q, which is not an x-store schema, the stores are %s", where, declared.From, list(sortedKeys(stores)))
	}

	if declared.Adapters == nil {
		return false, TypeDef{}, nil, fmt.Errorf("reading %s: x-stream names no adapters, adapters lists which of %s the engine emits", where, list(FeedAdapterKinds()))
	}

	if len(*declared.Adapters) == 0 {
		return false, TypeDef{}, nil, fmt.Errorf("reading %s: x-stream names an empty adapters list, a feed nobody can build is a feed nobody can use", where)
	}

	for _, kind := range *declared.Adapters {
		if kind != FeedAdapterMemory {
			return false, TypeDef{}, nil, fmt.Errorf("reading %s: x-stream names adapter kind %q, a feed adapter is one of %s", where, kind, list(FeedAdapterKinds()))
		}
	}

	return true, source, *declared.Adapters, nil
}

func checkStreamMapping(where, event string, source TypeDef, byName map[string]TypeDef) error {
	from := source.Name
	carried := map[string]Field{}
	for _, f := range source.Fields {
		carried[f.Name] = f
	}

	for _, f := range byName[event].Fields {
		held, ok := carried[f.Name]
		if !ok {
			return fmt.Errorf("reading %s: the stream carries %q whose property %q is not a property of %q, a saved record fills the event it publishes field by field", where, event, f.Name, from)
		}

		if held.Type.Kind != f.Type.Kind {
			return fmt.Errorf("reading %s: the stream carries %q whose property %q is a %s and %q declares it a %s", where, event, f.Name, f.Type.Kind, from, held.Type.Kind)
		}
	}

	return nil
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

		return nil, fmt.Errorf("reading %s: x-ports names %q, which is not <Name>Store of an x-store schema and no operation declares it with a kind", where, port)
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
