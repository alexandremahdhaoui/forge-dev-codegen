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
	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/rustname"
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
	Header           string
	Package          string
	Cell             string
	CratePath        string
	ModulePrefix     string
	SchemaVersion    int
	ServicePascal    string
	ServiceSnake     string
	ClientTrait      string
	ClientError      string
	ClientStruct     string
	ClientConfig     string
	ClientModule     string
	ClientName       string
	DriverStruct     string
	DriverConfig     string
	DriverError      string
	DriverModule     string
	DriverName       string
	DefaultAddress   string
	DefaultEndpoint  string
	DefaultTimeoutMs int
	CodecError       string
	RequestEnum      string
	ControllerSnake  string
	ControllerTrait  string
	ControllerError  string
	Messages         []messageView
	Rpcs             []rpcView
	TraitTypes       []string
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
	mv := messageView{Name: rustname.Pascal(m.Name)}

	for _, f := range m.Fields {
		rustType := grpcrust.ScalarRustType(f.Scalar)
		if f.Kind == grpcrust.FieldMessage {
			rustType = "Option<" + rustname.Pascal(f.Message) + ">"
		}

		mv.Fields = append(mv.Fields, fieldView{
			Ident:    rustname.RustIdent(f.Name),
			RustType: rustType,
			Prost:    prostAttribute(f),
		})
	}

	return mv
}

func buildServiceView(spec *grpcrust.Spec, svc grpcrust.Service, opts Options, only bool) (serviceView, error) {
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

	driverName := opts.Cell
	clientName := opts.Cell + "_client"

	if !only {
		driverName = opts.Cell + "_" + rustname.Snake(svc.Name)
		clientName = opts.Cell + "_" + rustname.Snake(svc.Name) + "_client"
	}

	sv := serviceView{
		Header:           header,
		Package:          spec.Package,
		Cell:             opts.Cell,
		CratePath:        "crate::" + opts.Cell + "::",
		ModulePrefix:     opts.Cell + "::",
		SchemaVersion:    version,
		ServicePascal:    rustname.Pascal(svc.Name),
		ServiceSnake:     rustname.Snake(svc.Name),
		ClientTrait:      rustname.Pascal(svc.Name) + "Client",
		ClientError:      rustname.Pascal(svc.Name) + "ClientError",
		ClientStruct:     rustname.Pascal(svc.Name) + "UdpClient",
		ClientConfig:     rustname.Pascal(svc.Name) + "UdpClientConfig",
		ClientModule:     rustname.Snake(svc.Name) + "_udp_client",
		ClientName:       clientName,
		DriverStruct:     rustname.Pascal(svc.Name) + "UdpDriver",
		DriverConfig:     rustname.Pascal(svc.Name) + "UdpDriverConfig",
		DriverError:      rustname.Pascal(svc.Name) + "UdpDriverError",
		DriverModule:     rustname.Snake(svc.Name) + "_udp_driver",
		DriverName:       driverName,
		DefaultAddress:   DefaultAddress,
		DefaultEndpoint:  DefaultEndpoint,
		DefaultTimeoutMs: DefaultTimeoutMs,
		CodecError:       rustname.Pascal(svc.Name) + "CodecError",
		RequestEnum:      rustname.Pascal(svc.Name) + "Request",
		ControllerSnake:  rustname.Snake(svc.Name),
		ControllerTrait:  rustname.Pascal(svc.Name) + "Controller",
		ControllerError:  rustname.Pascal(svc.Name) + "ControllerError",
	}

	for _, m := range messages {
		sv.Messages = append(sv.Messages, buildMessageView(m))
	}

	seenTrait := map[string]bool{}

	for _, r := range svc.Rpcs {
		fullMethod := spec.Package + "." + svc.Name + "/" + r.Name
		hash := FunctionHash(fullMethod)

		sv.Rpcs = append(sv.Rpcs, rpcView{
			Ident:      rustname.RustIdent(r.Name),
			Pascal:     rustname.Pascal(r.Name),
			Upper:      rustname.Upper(r.Name),
			Request:    rustname.Pascal(r.Request),
			Reply:      rustname.Pascal(r.Response),
			FullMethod: fullMethod,
			Hash:       hash,
			Silent:     rustname.Pascal(r.Response) == NothingMessage,
		})

		for _, t := range []string{rustname.Pascal(r.Request), rustname.Pascal(r.Response)} {
			if !seenTrait[t] {
				seenTrait[t] = true

				sv.TraitTypes = append(sv.TraitTypes, t)
			}
		}
	}

	return sv, nil
}
