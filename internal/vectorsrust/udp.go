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
)

const datagramOperationPrefix = "udp_"

const sessionIDField = "sessionId"

const sessionIDLength = 16

type datagramRpc struct {
	Ident   string
	Pascal  string
	Request grpcrust.Message
	Reply   grpcrust.Message
}

type datagramService struct {
	Pascal      string
	Snake       string
	Cell        string
	ClientTrait string
	Rpcs        map[string]datagramRpc
}

type datagramMockOp struct {
	Ident   string
	Request string
	Reply   string
}

type datagramServiceView struct {
	Pascal        string
	Snake         string
	Cell          string
	ClientTrait   string
	ClientStruct  string
	DriverStruct  string
	ControllerVar string
	Ops           []datagramMockOp
	TypeImports   []string
}

type datagramTestView struct {
	Name            string
	ServicePascal   string
	ControllerVar   string
	ExpectMethod    string
	RequestLiteral  string
	ReplyLiteral    string
	ExpectedLiteral string
	SessionLiteral  string
	ClientStruct    string
	DriverStruct    string
	ClientMethod    string
	Cell            string
}

func readDatagramService(proto []byte, cell string) (*datagramService, error) {
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

	byHash := map[uint8]string{}
	rpcs := map[string]datagramRpc{}

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

		rpcs[datagramOperationPrefix+grpcrust.Snake(r.Name)] = datagramRpc{
			Ident:   grpcrust.RustIdent(r.Name),
			Pascal:  grpcrust.Pascal(r.Name),
			Request: request,
			Reply:   reply,
		}
	}

	return &datagramService{
		Pascal:      grpcrust.Pascal(svc.Name),
		Snake:       grpcrust.Snake(svc.Name),
		Cell:        cell,
		ClientTrait: grpcrust.Pascal(svc.Name) + "Client",
		Rpcs:        rpcs,
	}, nil
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
		Pascal:        svc.Pascal,
		Snake:         svc.Snake,
		Cell:          svc.Cell,
		ClientTrait:   svc.ClientTrait,
		ClientStruct:  svc.Pascal + "UdpClient",
		DriverStruct:  svc.Pascal + "UdpDriver",
		ControllerVar: svc.Snake + "_controller",
	}

	types := map[string]bool{}

	for _, operation := range sortedKeys(svc.Rpcs) {
		rpc := svc.Rpcs[operation]

		view.Ops = append(view.Ops, datagramMockOp{
			Ident:   rpc.Ident,
			Request: grpcrust.Pascal(rpc.Request.Name),
			Reply:   grpcrust.Pascal(rpc.Reply.Name),
		})

		types[grpcrust.Pascal(rpc.Request.Name)] = true
		types[grpcrust.Pascal(rpc.Reply.Name)] = true
	}

	view.TypeImports = sortedStrings(types)

	return view
}

func buildDatagramTest(c VectorCase, svc *datagramService) (datagramTestView, error) {
	rpc, ok := svc.Rpcs[c.Operation]
	if !ok {
		return datagramTestView{}, fmt.Errorf("reading vector %q: operation %q names no rpc of %s", c.Case, c.Operation, svc.Pascal)
	}

	session, err := sessionLiteral(c)
	if err != nil {
		return datagramTestView{}, err
	}

	request, err := messageLiteral(rpc.Request, c.Input)
	if err != nil {
		return datagramTestView{}, fmt.Errorf("reading vector %q: reading input: %w", c.Case, err)
	}

	reply, err := messageLiteral(rpc.Reply, c.ControllerReply)
	if err != nil {
		return datagramTestView{}, fmt.Errorf("reading vector %q: reading controllerReply: %w", c.Case, err)
	}

	expected, err := messageLiteral(rpc.Reply, c.ExpectedBody)
	if err != nil {
		return datagramTestView{}, fmt.Errorf("reading vector %q: reading expectedBody: %w", c.Case, err)
	}

	return datagramTestView{
		Name:            c.Case,
		ServicePascal:   svc.Pascal,
		ControllerVar:   svc.Snake + "_controller",
		ExpectMethod:    "expect_" + rpc.Ident,
		RequestLiteral:  request,
		ReplyLiteral:    reply,
		ExpectedLiteral: expected,
		SessionLiteral:  session,
		ClientStruct:    svc.Pascal + "UdpClient",
		DriverStruct:    svc.Pascal + "UdpDriver",
		ClientMethod:    rpc.Ident,
		Cell:            svc.Cell,
	}, nil
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

	return "*b" + strconv.Quote(value), nil
}

func messageLiteral(m grpcrust.Message, raw json.RawMessage) (string, error) {
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

		parts = append(parts, grpcrust.RustIdent(f.Name)+": "+literal)
	}

	if len(parts) == 0 {
		return grpcrust.Pascal(m.Name) + " {}", nil
	}

	return grpcrust.Pascal(m.Name) + " { " + strings.Join(parts, ", ") + " }", nil
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
