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
	"strings"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/grpcrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/taxonomy"
	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/rustname"
)

const callOperationPrefix = "grpc_"

const runtimeVariant = "Runtime"

const defaultGrpcCell = "grpc"

type callRpc struct {
	Name    string
	Ident   string
	Pascal  string
	Request grpcrust.Message
	Reply   grpcrust.Message
}

type callService struct {
	Pascal   string
	Snake    string
	Cell     string
	Rpcs     map[string]callRpc
	Messages map[string]grpcrust.Message
}

func (s *callService) scope(prefix string) scope {
	return scope{messages: s.Messages, prefix: prefix}
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
	Refused                  bool
	ExpectedCode             string
	ExpectedMember           string
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
		Pascal:   rustname.Pascal(svc.Name),
		Snake:    rustname.Snake(svc.Name),
		Cell:     cell,
		Rpcs:     map[string]callRpc{},
		Messages: messagesByName(spec),
	}

	for _, r := range svc.Rpcs {
		request, err := messageNamed(spec, r.Request)
		if err != nil {
			return nil, err
		}

		reply, err := messageNamed(spec, r.Response)
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

	request, err := messageLiteral(svc.scope(view.TypesAlias), rpc.Request, c.Input)
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
		Refused:                  c.ExpectedError != "",
		HasExpectedSubstring:     c.ExpectedErrorSubstring != "",
		ExpectedSubstringLiteral: strconv.Quote(c.ExpectedErrorSubstring),
	}

	if tv.Refused {
		member, ok := taxonomy.ByVariant(c.ExpectedError)
		if !ok || member.Variant == runtimeVariant {
			return callTestView{}, fmt.Errorf(
				"reading vector %q: expectedError %q names no member a controller answers over grpc, the members are %s",
				c.Case, c.ExpectedError, strings.Join(taxonomy.DetailedVariantNames(), ", "),
			)
		}

		subject := c.ExpectedErrorSubstring
		if subject == "" {
			subject = rpc.Name
		}

		tv.ExpectedCode = member.GrpcCode
		tv.ExpectedMember = member.Variant
		tv.Returning = "Err(" + member.Literal(view.ControllerError, subject, taxonomy.VectorFiller) + ")"

		return tv, nil
	}

	reply, err := messageLiteral(svc.scope(view.TypesAlias), rpc.Reply, c.ControllerReply)
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
