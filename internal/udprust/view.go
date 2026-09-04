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

package udprust

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/grpcrust"
)

const NothingMessage = "Nothing"

type fieldView struct {
	Ident    string
	RustType string
	Prost    string
}

type messageView struct {
	Name   string
	Fields []fieldView
}

type rpcView struct {
	Ident       string
	Pascal      string
	Request     string
	Reply       string
	Tag         int
	EnvelopeTag int
	Silent      bool
}

type serviceView struct {
	Header          string
	Package         string
	Cell            string
	CoreCrate       string
	ServicePascal   string
	ServiceSnake    string
	ClientTrait     string
	ClientError     string
	ClientStruct    string
	DriverStruct    string
	DriverError     string
	CodecModule     string
	CodecError      string
	KindEnum        string
	RequestEnum     string
	EnvelopeStruct  string
	EnvelopeEnum    string
	ControllerSnake string
	ControllerTrait string
	ControllerError string
	Messages        []messageView
	Rpcs            []rpcView
	TraitTypes      []string
	EnvelopeTags    string
}

func prostAttribute(f grpcrust.Field) string {
	tag := strconv.Itoa(f.Number)

	if f.Kind == grpcrust.FieldMessage {
		return `#[prost(message, optional, tag = "` + tag + `")]`
	}

	if f.Scalar == "bytes" {
		return `#[prost(bytes = "vec", tag = "` + tag + `")]`
	}

	return `#[prost(` + f.Scalar + `, tag = "` + tag + `")]`
}

func buildMessageView(m grpcrust.Message) messageView {
	mv := messageView{Name: grpcrust.Pascal(m.Name)}

	for _, f := range m.Fields {
		rustType := grpcrust.ScalarRustType(f.Scalar)
		if f.Kind == grpcrust.FieldMessage {
			rustType = "Option<" + grpcrust.Pascal(f.Message) + ">"
		}

		mv.Fields = append(mv.Fields, fieldView{
			Ident:    grpcrust.RustIdent(f.Name),
			RustType: rustType,
			Prost:    prostAttribute(f),
		})
	}

	return mv
}

func buildServiceView(spec *grpcrust.Spec, svc grpcrust.Service, opts Options) (serviceView, error) {
	roots := []string{}
	for _, r := range svc.Rpcs {
		roots = append(roots, r.Request, r.Response)
	}

	sort.Strings(roots)

	messages, err := grpcrust.Closure(spec, roots)
	if err != nil {
		return serviceView{}, fmt.Errorf("building service %q: %w", svc.Name, err)
	}

	sv := serviceView{
		Header:          header,
		Package:         spec.Package,
		Cell:            opts.Cell,
		CoreCrate:       grpcrust.Snake(opts.Service) + "_core",
		ServicePascal:   grpcrust.Pascal(svc.Name),
		ServiceSnake:    grpcrust.Snake(svc.Name),
		ClientTrait:     grpcrust.Pascal(svc.Name) + "Client",
		ClientError:     grpcrust.Pascal(svc.Name) + "ClientError",
		ClientStruct:    grpcrust.Pascal(svc.Name) + "UdpClient",
		DriverStruct:    grpcrust.Pascal(svc.Name) + "UdpDriver",
		DriverError:     grpcrust.Pascal(svc.Name) + "UdpDriverError",
		CodecModule:     grpcrust.Snake(svc.Name) + "_codec",
		CodecError:      grpcrust.Pascal(svc.Name) + "CodecError",
		KindEnum:        grpcrust.Pascal(svc.Name) + "Kind",
		RequestEnum:     grpcrust.Pascal(svc.Name) + "Request",
		EnvelopeStruct:  grpcrust.Pascal(svc.Name) + "Envelope",
		EnvelopeEnum:    grpcrust.Pascal(svc.Name) + "EnvelopeKind",
		ControllerSnake: grpcrust.Snake(svc.Name),
		ControllerTrait: grpcrust.Pascal(svc.Name) + "Controller",
		ControllerError: grpcrust.Pascal(svc.Name) + "ControllerError",
	}

	for _, m := range messages {
		sv.Messages = append(sv.Messages, buildMessageView(m))
	}

	seenTrait := map[string]bool{}
	envelopeTags := ""

	for i, r := range svc.Rpcs {
		if i > 0 {
			envelopeTags += ", "
		}

		envelopeTags += strconv.Itoa(i + 1)

		sv.Rpcs = append(sv.Rpcs, rpcView{
			Ident:       grpcrust.RustIdent(r.Name),
			Pascal:      grpcrust.Pascal(r.Name),
			Request:     grpcrust.Pascal(r.Request),
			Reply:       grpcrust.Pascal(r.Response),
			Tag:         i,
			EnvelopeTag: i + 1,
			Silent:      grpcrust.Pascal(r.Response) == NothingMessage,
		})

		for _, t := range []string{grpcrust.Pascal(r.Request), grpcrust.Pascal(r.Response)} {
			if !seenTrait[t] {
				seenTrait[t] = true

				sv.TraitTypes = append(sv.TraitTypes, t)
			}
		}
	}

	sv.EnvelopeTags = envelopeTags

	if len(svc.Rpcs) > maxRpcs {
		return serviceView{}, fmt.Errorf("building service %q: a datagram tag is one byte, so a service declares at most %d rpcs, got %d", svc.Name, maxRpcs, len(svc.Rpcs))
	}

	return sv, nil
}
