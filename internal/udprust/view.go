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
	"strings"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/grpcrust"
)

const NothingMessage = "Nothing"

const (
	fnvOffsetBasis uint32 = 2166136261
	fnvPrime       uint32 = 16777619
)

func FunctionHash(fullMethod string) uint8 {
	hash := fnvOffsetBasis

	for i := 0; i < len(fullMethod); i++ {
		hash ^= uint32(fullMethod[i])
		hash *= fnvPrime
	}

	return uint8(hash) ^ uint8(hash>>8) ^ uint8(hash>>16) ^ uint8(hash>>24)
}

func SchemaVersion(protoPackage string) (int, error) {
	segments := strings.Split(protoPackage, ".")

	last := segments[len(segments)-1]
	if !strings.HasPrefix(last, "v") {
		return 0, fmt.Errorf("reading the schema version of package %q: the last segment must be a version like v1, got %q", protoPackage, last)
	}

	version, err := strconv.Atoi(strings.TrimPrefix(last, "v"))
	if err != nil {
		return 0, fmt.Errorf("reading the schema version of package %q: the last segment must be a version like v1, got %q", protoPackage, last)
	}

	if version < 0 || version > 255 {
		return 0, fmt.Errorf("reading the schema version of package %q: the version byte holds 0 to 255, got %d", protoPackage, version)
	}

	return version, nil
}

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
	Ident      string
	Pascal     string
	Upper      string
	Request    string
	Reply      string
	FullMethod string
	Hash       uint8
	Silent     bool
}

type serviceView struct {
	Header          string
	Package         string
	Cell            string
	CoreCrate       string
	SchemaVersion   int
	ServicePascal   string
	ServiceSnake    string
	ClientTrait     string
	ClientError     string
	ClientStruct    string
	DriverStruct    string
	DriverError     string
	CodecError      string
	RequestEnum     string
	ControllerSnake string
	ControllerTrait string
	ControllerError string
	Messages        []messageView
	Rpcs            []rpcView
	TraitTypes      []string
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
	version, err := SchemaVersion(spec.Package)
	if err != nil {
		return serviceView{}, err
	}

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
		SchemaVersion:   version,
		ServicePascal:   grpcrust.Pascal(svc.Name),
		ServiceSnake:    grpcrust.Snake(svc.Name),
		ClientTrait:     grpcrust.Pascal(svc.Name) + "Client",
		ClientError:     grpcrust.Pascal(svc.Name) + "ClientError",
		ClientStruct:    grpcrust.Pascal(svc.Name) + "UdpClient",
		DriverStruct:    grpcrust.Pascal(svc.Name) + "UdpDriver",
		DriverError:     grpcrust.Pascal(svc.Name) + "UdpDriverError",
		CodecError:      grpcrust.Pascal(svc.Name) + "CodecError",
		RequestEnum:     grpcrust.Pascal(svc.Name) + "Request",
		ControllerSnake: grpcrust.Snake(svc.Name),
		ControllerTrait: grpcrust.Pascal(svc.Name) + "Controller",
		ControllerError: grpcrust.Pascal(svc.Name) + "ControllerError",
	}

	for _, m := range messages {
		sv.Messages = append(sv.Messages, buildMessageView(m))
	}

	seenTrait := map[string]bool{}
	byHash := map[uint8]string{}

	for _, r := range svc.Rpcs {
		fullMethod := spec.Package + "." + svc.Name + "/" + r.Name
		hash := FunctionHash(fullMethod)

		if other, taken := byHash[hash]; taken {
			return serviceView{}, fmt.Errorf("hashing the methods of service %q: %s and %s both fold to the function hash %d, rename one of them", svc.Name, other, fullMethod, hash)
		}

		byHash[hash] = fullMethod

		sv.Rpcs = append(sv.Rpcs, rpcView{
			Ident:      grpcrust.RustIdent(r.Name),
			Pascal:     grpcrust.Pascal(r.Name),
			Upper:      grpcrust.Upper(r.Name),
			Request:    grpcrust.Pascal(r.Request),
			Reply:      grpcrust.Pascal(r.Response),
			FullMethod: fullMethod,
			Hash:       hash,
			Silent:     grpcrust.Pascal(r.Response) == NothingMessage,
		})

		for _, t := range []string{grpcrust.Pascal(r.Request), grpcrust.Pascal(r.Response)} {
			if !seenTrait[t] {
				seenTrait[t] = true

				sv.TraitTypes = append(sv.TraitTypes, t)
			}
		}
	}

	return sv, nil
}
