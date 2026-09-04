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
	"sort"
)

type fieldView struct {
	Ident    string
	CoreType string
	PbType   string
	ToCore   string
	ToPb     string
}

type messageView struct {
	Name   string
	Fields []fieldView
}

type rpcView struct {
	Ident      string
	Pascal     string
	Request    string
	Response   string
	PbRequest  string
	PbResponse string
}

type serviceView struct {
	Header          string
	Package         string
	CoreCrate       string
	ServicePascal   string
	ServiceSnake    string
	ClientTrait     string
	ClientError     string
	ClientStruct    string
	DriverStruct    string
	ControllerSnake string
	ControllerTrait string
	ControllerError string
	PbClientMod     string
	PbServerMod     string
	Messages        []messageView
	Rpcs            []rpcView
	TraitTypes      []string
	AllTypes        []string
}

func scalarRustType(kind string) string {
	switch kind {
	case "double":
		return "f64"
	case "float":
		return "f32"
	case "int32", "sint32", "sfixed32":
		return "i32"
	case "int64", "sint64", "sfixed64":
		return "i64"
	case "uint32", "fixed32":
		return "u32"
	case "uint64", "fixed64":
		return "u64"
	case "bool":
		return "bool"
	case "string":
		return "String"
	case "bytes":
		return "Vec<u8>"
	default:
		return "String"
	}
}

func buildMessageView(m Message) messageView {
	mv := messageView{Name: Pascal(m.Name)}

	for _, f := range m.Fields {
		ident := rustIdent(f.Name)

		var core, pb string

		switch f.Kind {
		case FieldScalar:
			t := scalarRustType(f.Scalar)
			core, pb = t, t
		case FieldMessage:
			t := "Option<" + Pascal(f.Message) + ">"
			pbT := "Option<pb::" + Pascal(f.Message) + ">"
			core, pb = t, pbT
		}

		toCore := "v." + ident
		toPb := "v." + ident

		if f.Kind == FieldMessage {
			toCore = "v." + ident + ".map(Into::into)"
			toPb = "v." + ident + ".map(Into::into)"
		}

		mv.Fields = append(mv.Fields, fieldView{
			Ident:    ident,
			CoreType: core,
			PbType:   pb,
			ToCore:   toCore,
			ToPb:     toPb,
		})
	}

	return mv
}

func closure(spec *Spec, roots []string) ([]Message, error) {
	byName := map[string]Message{}
	for _, m := range spec.Messages {
		byName[m.Name] = m
	}

	seen := map[string]bool{}
	order := []string{}

	var visit func(name string) error
	visit = func(name string) error {
		if seen[name] {
			return nil
		}

		seen[name] = true

		m, ok := byName[name]
		if !ok {
			return fmt.Errorf("resolving message %q: not defined", name)
		}

		order = append(order, name)

		for _, f := range m.Fields {
			if f.Kind == FieldMessage {
				if err := visit(f.Message); err != nil {
					return err
				}
			}
		}

		return nil
	}

	for _, r := range roots {
		if err := visit(r); err != nil {
			return nil, err
		}
	}

	messages := make([]Message, 0, len(order))
	for _, name := range order {
		messages = append(messages, byName[name])
	}

	return messages, nil
}

func buildServiceView(spec *Spec, svc Service, opts Options) (serviceView, error) {
	roots := []string{}
	for _, r := range svc.Rpcs {
		roots = append(roots, r.Request, r.Response)
	}

	sort.Strings(roots)

	messages, err := closure(spec, roots)
	if err != nil {
		return serviceView{}, fmt.Errorf("building service %q: %w", svc.Name, err)
	}

	sv := serviceView{
		Header:          header,
		Package:         spec.Package,
		CoreCrate:       Snake(opts.Service) + "_core",
		ServicePascal:   Pascal(svc.Name),
		ServiceSnake:    Snake(svc.Name),
		ClientTrait:     Pascal(svc.Name) + "Client",
		ClientError:     Pascal(svc.Name) + "ClientError",
		ClientStruct:    Pascal(svc.Name) + "GrpcClient",
		DriverStruct:    Pascal(svc.Name) + "GrpcDriver",
		ControllerSnake: Snake(svc.Name),
		ControllerTrait: Pascal(svc.Name) + "Controller",
		ControllerError: Pascal(svc.Name) + "ControllerError",
		PbClientMod:     Snake(svc.Name) + "_client",
		PbServerMod:     Snake(svc.Name) + "_server",
	}

	for _, m := range messages {
		sv.Messages = append(sv.Messages, buildMessageView(m))
		sv.AllTypes = append(sv.AllTypes, Pascal(m.Name))
	}

	seenTrait := map[string]bool{}

	for _, r := range svc.Rpcs {
		sv.Rpcs = append(sv.Rpcs, rpcView{
			Ident:      Snake(r.Name),
			Pascal:     Pascal(r.Name),
			Request:    Pascal(r.Request),
			Response:   Pascal(r.Response),
			PbRequest:  "pb::" + Pascal(r.Request),
			PbResponse: "pb::" + Pascal(r.Response),
		})

		for _, t := range []string{Pascal(r.Request), Pascal(r.Response)} {
			if !seenTrait[t] {
				seenTrait[t] = true

				sv.TraitTypes = append(sv.TraitTypes, t)
			}
		}
	}

	return sv, nil
}
