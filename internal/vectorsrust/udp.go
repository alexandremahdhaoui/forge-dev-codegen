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

package vectorsrust

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/grpcrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/udprust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/rustname"
)

const datagramOperationPrefix = "udp_"

const sessionIDField = "sessionId"

const sessionIDLength = 16

const droppedTimeoutMs = 300

const pushTimeoutMs = 2000

const tickIntervalMs = 10

const (
	gateAdmit  = "admit"
	gateRefuse = "refuse"
)

const (
	sessionRegistered = "registered"
	sessionUnknown    = "unknown"
)

const (
	kindReply     = "reply"
	kindDropped   = "dropped"
	kindReconnect = "reconnect"
	kindPush      = "push"
)

type datagramRpc struct {
	Name    string
	Ident   string
	Pascal  string
	Request grpcrust.Message
	Reply   grpcrust.Message
	Silent  bool
	Hello   bool
	Push    bool
}

type datagramService struct {
	Pascal      string
	Snake       string
	Cell        string
	ClientTrait string
	Session     bool
	Hello       datagramRpc
	Rpcs        map[string]datagramRpc
}

type datagramMockOp struct {
	Ident   string
	Request string
	Reply   string
}

type datagramServiceView struct {
	Pascal          string
	Snake           string
	Cell            string
	ClientTrait     string
	ClientStruct    string
	DriverStruct    string
	ControllerVar   string
	Ops             []datagramMockOp
	TypeImports     []string
	Session         bool
	HelloIdent      string
	HelloRequest    string
	HelloReply      string
	HelloSilent     bool
	GateTrait       string
	GateError       string
	BroadcastStruct string
	BroadcastConfig string
	PeerTableTrait  string
	PeerTableStruct string
	PeerTableConfig string
	TickStruct      string
	TickConfig      string
	PushEnum        string
	PushModule      string
	ControllerError string
}

type datagramTestView struct {
	Name            string
	Kind            string
	Session         bool
	ServicePascal   string
	ServiceSnake    string
	ControllerVar   string
	ControllerError string
	ExpectMethod    string
	RpcPascal       string
	RequestLiteral  string
	ReplyLiteral    string
	ExpectedLiteral string
	SessionLiteral  string
	SessionLiterals []string
	HelloLiteral    string
	HelloIdent      string
	HelloReply      string
	GateAdmits      bool
	Registered      bool
	IsHello         bool
	PushVariant     string
	PushLiteral     string
	PushEnum        string
	ClientStruct    string
	ClientConfig    string
	DriverStruct    string
	DriverConfig    string
	TimeoutMs       int
	TickIntervalMs  int
	ClientMethod    string
	Cell            string
}

func readDatagramService(proto []byte, cell, hello string, push []string) (*datagramService, error) {
	if len(proto) == 0 {
		return nil, nil
	}

	spec, err := grpcrust.Parse(proto)
	if err != nil {
		return nil, err
	}

	if len(spec.Services) != 1 {
		return nil, fmt.Errorf("reading the datagram proto: it must declare exactly one service, got %d", len(spec.Services))
	}

	if _, err := udprust.SchemaVersion(spec.Package); err != nil {
		return nil, err
	}

	svc := spec.Services[0]

	if err := checkSessionNames(svc, hello, push); err != nil {
		return nil, err
	}

	byHash := map[uint8]string{}
	rpcs := map[string]datagramRpc{}

	out := &datagramService{
		Pascal:      rustname.Pascal(svc.Name),
		Snake:       rustname.Snake(svc.Name),
		Cell:        cell,
		ClientTrait: rustname.Pascal(svc.Name) + "Client",
		Session:     hello != "",
		Rpcs:        rpcs,
	}

	for _, r := range svc.Rpcs {
		fullMethod := spec.Package + "." + svc.Name + "/" + r.Name
		hash := udprust.FunctionHash(fullMethod)

		if other, taken := byHash[hash]; taken {
			return nil, fmt.Errorf("hashing the methods of service %q: %s and %s both fold to the function hash %d, rename one of them", svc.Name, other, fullMethod, hash)
		}

		byHash[hash] = fullMethod

		request, err := messageNamed(spec, r.Request)
		if err != nil {
			return nil, err
		}

		reply, err := messageNamed(spec, r.Response)
		if err != nil {
			return nil, err
		}

		rpc := datagramRpc{
			Name:    r.Name,
			Ident:   rustname.RustIdent(r.Name),
			Pascal:  rustname.Pascal(r.Name),
			Request: request,
			Reply:   reply,
			Silent:  rustname.Pascal(r.Response) == udprust.NothingMessage,
			Hello:   hello != "" && r.Name == hello,
			Push:    contains(push, r.Name),
		}

		if rpc.Hello {
			out.Hello = rpc
		}

		rpcs[datagramOperationPrefix+rustname.Snake(r.Name)] = rpc
	}

	return out, nil
}

func checkSessionNames(svc grpcrust.Service, hello string, push []string) error {
	names := map[string]bool{}
	for _, r := range svc.Rpcs {
		names[r.Name] = true
	}

	if hello != "" && !names[hello] {
		return fmt.Errorf("naming the hello rpc: %q is not an rpc of service %q", hello, svc.Name)
	}

	for _, name := range push {
		if !names[name] {
			return fmt.Errorf("naming the push rpcs: %q is not an rpc of service %q", name, svc.Name)
		}

		if name == hello {
			return fmt.Errorf("naming the push rpcs: %q is the hello rpc", name)
		}
	}

	if len(push) > 0 && hello == "" {
		return fmt.Errorf("naming the push rpcs: %s pushed and no layout.hello named", strings.Join(push, ", "))
	}

	return nil
}

func contains(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}

	return false
}

func messageNamed(spec *grpcrust.Spec, name string) (grpcrust.Message, error) {
	for _, m := range spec.Messages {
		if m.Name == name {
			return m, nil
		}
	}

	return grpcrust.Message{}, fmt.Errorf("reading the datagram proto: message %q is not defined", name)
}

func buildDatagramServiceView(svc *datagramService) datagramServiceView {
	view := datagramServiceView{
		Pascal:          svc.Pascal,
		Snake:           svc.Snake,
		Cell:            svc.Cell,
		ClientTrait:     svc.ClientTrait,
		ClientStruct:    svc.Pascal + "UdpClient",
		DriverStruct:    svc.Pascal + "UdpDriver",
		ControllerVar:   svc.Snake + "_controller",
		ControllerError: svc.Pascal + "ControllerError",
		Session:         svc.Session,
		GateTrait:       svc.Pascal + "SessionGate",
		GateError:       svc.Pascal + "SessionGateError",
		BroadcastStruct: svc.Pascal + "UdpBroadcast",
		BroadcastConfig: svc.Pascal + "UdpBroadcastConfig",
		PeerTableTrait:  svc.Pascal + "PeerTable",
		PeerTableStruct: svc.Pascal + "UdpPeerTable",
		PeerTableConfig: svc.Pascal + "UdpPeerTableConfig",
		TickStruct:      svc.Pascal + "TickDriver",
		TickConfig:      svc.Pascal + "TickDriverConfig",
		PushEnum:        svc.Pascal + "Push",
		PushModule:      svc.Snake + "_push",
	}

	if svc.Session {
		view.HelloIdent = svc.Hello.Ident
		view.HelloRequest = rustname.Pascal(svc.Hello.Request.Name)
		view.HelloReply = rustname.Pascal(svc.Hello.Reply.Name)
		view.HelloSilent = svc.Hello.Silent
	}

	types := map[string]bool{}

	for _, operation := range sortedKeys(svc.Rpcs) {
		rpc := svc.Rpcs[operation]

		if rpc.Push {
			types[rustname.Pascal(rpc.Request.Name)] = true

			continue
		}

		view.Ops = append(view.Ops, datagramMockOp{
			Ident:   rpc.Ident,
			Request: rustname.Pascal(rpc.Request.Name),
			Reply:   rustname.Pascal(rpc.Reply.Name),
		})

		types[rustname.Pascal(rpc.Request.Name)] = true
		types[rustname.Pascal(rpc.Reply.Name)] = true
	}

	view.TypeImports = sortedStrings(types)

	return view
}

func buildDatagramTest(c VectorCase, svc *datagramService) (datagramTestView, error) {
	rpc, ok := svc.Rpcs[c.Operation]
	if !ok {
		return datagramTestView{}, fmt.Errorf("reading vector %q: operation %q names no rpc of %s", c.Case, c.Operation, svc.Pascal)
	}

	tv := datagramTestView{
		Name:            c.Case,
		Kind:            kindReply,
		Session:         svc.Session,
		ServicePascal:   svc.Pascal,
		ServiceSnake:    svc.Snake,
		ControllerVar:   svc.Snake + "_controller",
		ControllerError: svc.Pascal + "ControllerError",
		ExpectMethod:    "expect_" + rpc.Ident,
		RpcPascal:       rpc.Pascal,
		ClientStruct:    svc.Pascal + "UdpClient",
		ClientConfig:    svc.Pascal + "UdpClientConfig",
		DriverStruct:    svc.Pascal + "UdpDriver",
		DriverConfig:    svc.Pascal + "UdpDriverConfig",
		TimeoutMs:       udprust.DefaultTimeoutMs,
		TickIntervalMs:  tickIntervalMs,
		ClientMethod:    rpc.Ident,
		Cell:            svc.Cell,
		IsHello:         rpc.Hello,
		PushEnum:        svc.Pascal + "Push",
		GateAdmits:      true,
		Registered:      true,
	}

	if err := readSessionFlags(c, svc, &tv); err != nil {
		return datagramTestView{}, err
	}

	if c.IsPush() {
		return buildPushTest(c, svc, rpc, tv)
	}

	if rpc.Push {
		return datagramTestView{}, fmt.Errorf("reading vector %q: %s is a push rpc, a push case carries expectPush and no input", c.Case, rpc.Pascal)
	}

	session, err := sessionLiteral(c)
	if err != nil {
		return datagramTestView{}, err
	}

	tv.SessionLiteral = session

	request, err := messageLiteral(rpc.Request, c.Input)
	if err != nil {
		return datagramTestView{}, fmt.Errorf("reading vector %q: reading input: %w", c.Case, err)
	}

	tv.RequestLiteral = request

	if c.ExpectDropped {
		tv.Kind = kindDropped
		tv.TimeoutMs = droppedTimeoutMs

		if len(c.ControllerReply) > 0 || len(c.ExpectedBody) > 0 {
			return datagramTestView{}, fmt.Errorf("reading vector %q: a dropped case expects no reply, drop controllerReply and expectedBody", c.Case)
		}

		return tv, nil
	}

	reply, err := messageLiteral(rpc.Reply, c.ControllerReply)
	if err != nil {
		return datagramTestView{}, fmt.Errorf("reading vector %q: reading controllerReply: %w", c.Case, err)
	}

	expected, err := messageLiteral(rpc.Reply, c.ExpectedBody)
	if err != nil {
		return datagramTestView{}, fmt.Errorf("reading vector %q: reading expectedBody: %w", c.Case, err)
	}

	tv.ReplyLiteral = reply
	tv.ExpectedLiteral = expected

	if c.Reconnect {
		if !rpc.Hello {
			return datagramTestView{}, fmt.Errorf("reading vector %q: reconnect is a hello sent again from a new address, and %s is not the hello rpc", c.Case, rpc.Pascal)
		}

		tv.Kind = kindReconnect
	}

	return tv, nil
}

func readSessionFlags(c VectorCase, svc *datagramService, tv *datagramTestView) error {
	sessionFields := c.Gate != "" || c.Session != "" || len(c.Hello) > 0 || c.Reconnect || c.ExpectDropped || c.IsPush()

	if sessionFields && !svc.Session {
		return fmt.Errorf("reading vector %q: it uses a session field and the cell names no layout.hello", c.Case)
	}

	switch c.Gate {
	case "", gateAdmit:
		tv.GateAdmits = true
	case gateRefuse:
		tv.GateAdmits = false
	default:
		return fmt.Errorf("reading vector %q: gate %q is neither %s nor %s", c.Case, c.Gate, gateAdmit, gateRefuse)
	}

	switch c.Session {
	case "", sessionRegistered:
		tv.Registered = true
	case sessionUnknown:
		tv.Registered = false
	default:
		return fmt.Errorf("reading vector %q: session %q is neither %s nor %s", c.Case, c.Session, sessionRegistered, sessionUnknown)
	}

	if !svc.Session {
		return nil
	}

	hello, err := messageLiteral(svc.Hello.Request, c.Hello)
	if err != nil {
		return fmt.Errorf("reading vector %q: reading hello: %w", c.Case, err)
	}

	tv.HelloLiteral = hello
	tv.HelloIdent = svc.Hello.Ident
	tv.HelloReply = rustname.Pascal(svc.Hello.Reply.Name)

	return nil
}

func buildPushTest(c VectorCase, svc *datagramService, rpc datagramRpc, tv datagramTestView) (datagramTestView, error) {
	if !rpc.Push {
		return datagramTestView{}, fmt.Errorf("reading vector %q: expectPush names %s and the cell does not list it under layout.push", c.Case, rpc.Pascal)
	}

	if c.ExpectPush.Rpc != "" && rustname.Pascal(c.ExpectPush.Rpc) != rpc.Pascal {
		return datagramTestView{}, fmt.Errorf("reading vector %q: expectPush names rpc %q and the operation names %s", c.Case, c.ExpectPush.Rpc, rpc.Pascal)
	}

	if len(c.Input) > 0 && string(c.Input) != "null" {
		return datagramTestView{}, fmt.Errorf("reading vector %q: a push case carries no input, the server sends it on a tick", c.Case)
	}

	if len(c.ExpectPush.SessionIds) == 0 {
		return datagramTestView{}, fmt.Errorf("reading vector %q: expectPush needs sessionIds, the registered peers the push must reach", c.Case)
	}

	for _, id := range c.ExpectPush.SessionIds {
		if len(id) != sessionIDLength {
			return datagramTestView{}, fmt.Errorf("reading vector %q: expectPush sessionId %q must be %d bytes, got %d", c.Case, id, sessionIDLength, len(id))
		}

		tv.SessionLiterals = append(tv.SessionLiterals, strconv.Quote(id))
	}

	payload, err := messageLiteral(rpc.Request, c.ExpectPush.Payload)
	if err != nil {
		return datagramTestView{}, fmt.Errorf("reading vector %q: reading expectPush.payload: %w", c.Case, err)
	}

	tv.Kind = kindPush
	tv.PushVariant = rpc.Pascal
	tv.PushLiteral = payload
	tv.TimeoutMs = pushTimeoutMs

	return tv, nil
}

func sessionLiteral(c VectorCase) (string, error) {
	fields, err := parseInput(c.Input)
	if err != nil {
		return "", fmt.Errorf("reading vector %q: %w", c.Case, err)
	}

	raw, ok := fields[sessionIDField]
	if !ok {
		return "", fmt.Errorf("reading vector %q: input needs a %s, the session id the client stamps on every datagram", c.Case, sessionIDField)
	}

	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("reading vector %q: %s must be a JSON string: %w", c.Case, sessionIDField, err)
	}

	if len(value) != sessionIDLength {
		return "", fmt.Errorf("reading vector %q: %s must be %d bytes, got %d", c.Case, sessionIDField, sessionIDLength, len(value))
	}

	return strconv.Quote(value), nil
}

func messageLiteral(m grpcrust.Message, raw json.RawMessage) (string, error) {
	if string(raw) == "null" {
		raw = nil
	}

	fields, err := parseInput(raw)
	if err != nil {
		return "", err
	}

	parts := make([]string, 0, len(m.Fields))

	for _, f := range m.Fields {
		if f.Kind != grpcrust.FieldScalar {
			return "", fmt.Errorf("field %q holds a message, a datagram vector reads scalar fields only", f.Name)
		}

		literal, err := scalarLiteral(f, fields[f.Name])
		if err != nil {
			return "", err
		}

		parts = append(parts, rustname.RustIdent(f.Name)+": "+literal)
	}

	if len(parts) == 0 {
		return rustname.Pascal(m.Name) + " {}", nil
	}

	return rustname.Pascal(m.Name) + " { " + strings.Join(parts, ", ") + " }", nil
}

func scalarLiteral(f grpcrust.Field, raw json.RawMessage) (string, error) {
	if f.Scalar == "bytes" {
		return "", fmt.Errorf("field %q holds bytes, a datagram vector reads strings, numbers and booleans only", f.Name)
	}

	if f.Scalar == "string" {
		if len(raw) == 0 {
			return "String::new()", nil
		}

		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", fmt.Errorf("field %q must be a JSON string: %w", f.Name, err)
		}

		return strconv.Quote(value) + ".to_string()", nil
	}

	if f.Scalar == "bool" {
		if len(raw) == 0 {
			return "false", nil
		}

		var value bool
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", fmt.Errorf("field %q must be a JSON boolean: %w", f.Name, err)
		}

		return strconv.FormatBool(value), nil
	}

	if len(raw) == 0 {
		return "0", nil
	}

	var value json.Number
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("field %q must be a JSON number: %w", f.Name, err)
	}

	return value.String(), nil
}

func sortedKeys(m map[string]datagramRpc) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	sort.Strings(out)

	return out
}
