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
	"fmt"
	"sort"
	"strconv"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/grpcrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/rustname"
)

const callOperationPrefix = "grpc_"

const defaultGrpcCell = "grpc"

type callRpc struct {
	Name    string
	Ident   string
	Pascal  string
	Request grpcrust.Message
	Reply   grpcrust.Message
}

type callService struct {
	Pascal string
	Snake  string
	Cell   string
	Rpcs   map[string]callRpc
}

type callMockOp struct {
	Ident   string
	Request string
	Reply   string
}

type callServiceView struct {
	Pascal          string
	Snake           string
	Cell            string
	TypesModule     string
	TypesAlias      string
	ControllerTrait string
	ControllerError string
	DriverStruct    string
	DriverConfig    string
	DriverModule    string
	ClientStruct    string
	ClientConfig    string
	ClientModule    string
	ClientTrait     string
	ClientPortMod   string
	Ops             []callMockOp
}

type callTestView struct {
	Name                     string
	ServicePascal            string
	ControllerVar            string
	ControllerError          string
	ExpectMethod             string
	ClientMethod             string
	RequestLiteral           string
	ReplyLiteral             string
	Returning                string
	DriverStruct             string
	DriverConfig             string
	ClientStruct             string
	ClientConfig             string
	HasExpectedSubstring     bool
	ExpectedSubstringLiteral string
}

func readCallService(proto []byte, cell string) (*callService, error) {
	if len(proto) == 0 {
		return nil, nil
	}

	if cell == "" {
		cell = defaultGrpcCell
	}

	if !rustname.IsModuleName(cell) {
		return nil, fmt.Errorf("reading the grpc proto: grpc cell %q is not a name Rust can spell as a module, use lowercase letters, digits and underscores and start with a letter", cell)
	}

	spec, err := grpcrust.Parse(proto)
	if err != nil {
		return nil, fmt.Errorf("reading the grpc proto: %w", err)
	}

	if len(spec.Services) != 1 {
		return nil, fmt.Errorf("reading the grpc proto: it must declare exactly one service, got %d", len(spec.Services))
	}

	svc := spec.Services[0]

	out := &callService{
		Pascal: rustname.Pascal(svc.Name),
		Snake:  rustname.Snake(svc.Name),
		Cell:   cell,
		Rpcs:   map[string]callRpc{},
	}

	for _, r := range svc.Rpcs {
		request, err := callMessageNamed(spec, r.Request)
		if err != nil {
			return nil, err
		}

		reply, err := callMessageNamed(spec, r.Response)
		if err != nil {
			return nil, err
		}

		rpc := callRpc{
			Name:    r.Name,
			Ident:   rustname.Snake(r.Name),
			Pascal:  rustname.Pascal(r.Name),
			Request: request,
			Reply:   reply,
		}

		out.Rpcs[callOperationPrefix+r.Name] = rpc
	}

	return out, nil
}

func callMessageNamed(spec *grpcrust.Spec, name string) (grpcrust.Message, error) {
	for _, m := range spec.Messages {
		if m.Name == name {
			return m, nil
		}
	}

	return grpcrust.Message{}, fmt.Errorf("reading the grpc proto: message %q is not defined", name)
}

func buildCallServiceView(svc *callService) callServiceView {
	view := callServiceView{
		Pascal:          svc.Pascal,
		Snake:           svc.Snake,
		Cell:            svc.Cell,
		TypesModule:     svc.Snake + "_messages",
		TypesAlias:      svc.Snake + "_grpc_messages",
		ControllerTrait: svc.Pascal + "Controller",
		ControllerError: svc.Pascal + "ControllerError",
		DriverStruct:    svc.Pascal + "GrpcDriver",
		DriverConfig:    svc.Pascal + "GrpcDriverConfig",
		DriverModule:    svc.Snake + "_grpc_driver",
		ClientStruct:    svc.Pascal + "GrpcClient",
		ClientConfig:    svc.Pascal + "GrpcClientConfig",
		ClientModule:    svc.Snake + "_grpc_client",
		ClientTrait:     svc.Pascal + "Client",
		ClientPortMod:   svc.Snake + "_client",
	}

	for _, operation := range sortedCallKeys(svc.Rpcs) {
		rpc := svc.Rpcs[operation]

		view.Ops = append(view.Ops, callMockOp{
			Ident:   rpc.Ident,
			Request: view.TypesAlias + "::" + rustname.Pascal(rpc.Request.Name),
			Reply:   view.TypesAlias + "::" + rustname.Pascal(rpc.Reply.Name),
		})
	}

	return view
}

func buildCallTest(c VectorCase, svc *callService, view callServiceView) (callTestView, error) {
	rpc, ok := svc.Rpcs[c.Operation]
	if !ok {
		return callTestView{}, fmt.Errorf("reading vector %q: operation %q names no rpc of %s", c.Case, c.Operation, svc.Pascal)
	}

	request, err := messageLiteral(rpc.Request, c.Input, view.TypesAlias)
	if err != nil {
		return callTestView{}, fmt.Errorf("reading vector %q: reading input: %w", c.Case, err)
	}

	tv := callTestView{
		Name:                     c.Case,
		ServicePascal:            svc.Pascal,
		ControllerVar:            svc.Snake + "_controller",
		ControllerError:          view.ControllerError,
		ExpectMethod:             "expect_" + rpc.Ident,
		ClientMethod:             rpc.Ident,
		RequestLiteral:           request,
		DriverStruct:             view.DriverStruct,
		DriverConfig:             view.DriverConfig,
		ClientStruct:             view.ClientStruct,
		ClientConfig:             view.ClientConfig,
		HasExpectedSubstring:     c.ExpectedErrorSubstring != "",
		ExpectedSubstringLiteral: strconv.Quote(c.ExpectedErrorSubstring),
	}

	if tv.HasExpectedSubstring {
		tv.Returning = fmt.Sprintf(
			"Err(%s::Invalid { field: %s.to_string(), reason: \"generated by vectors-rust\".to_string() })",
			view.ControllerError, tv.ExpectedSubstringLiteral,
		)

		return tv, nil
	}

	reply, err := messageLiteral(rpc.Reply, c.ControllerReply, view.TypesAlias)
	if err != nil {
		return callTestView{}, fmt.Errorf("reading vector %q: reading controllerReply: %w", c.Case, err)
	}

	tv.ReplyLiteral = reply
	tv.Returning = "Ok(controller_reply.clone())"

	return tv, nil
}

func sortedCallKeys(m map[string]callRpc) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	sort.Strings(out)

	return out
}
