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
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/grpcrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/rustname"
)

const NothingMessage = "Nothing"

var pascalIdent = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)

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
	Hello      bool
	Push       bool
}

type portView struct {
	Name               string
	Snake              string
	Kind               string
	Methods            []string
	Generated          bool
	HasMemory          bool
	MemoryStruct       string
	MemoryConfigStruct string
	MemoryAdapterName  string
	MemoryModule       string
}

type serviceView struct {
	Header           string
	Ports            []portView
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
	Inbound          []rpcView
	Pushes           []rpcView
	TraitTypes       []string
	CodecTypes       []string
	PushTypes        []string
	Session          bool
	HelloRpc         rpcView
	GateTrait        string
	GateError        string
	GateModule       string
	GateSecret       bool
	GateSecretField  string
	GateSecretName   string
	GateSecretStruct string
	GateSecretConfig string
	GateSecretModule string
	GateSnake        string
	BroadcastTrait   string
	BroadcastError   string
	BroadcastModule  string
	BroadcastSnake   string
	BroadcastStruct  string
	BroadcastConfig  string
	BroadcastAdapter string
	BroadcastName    string
	PeerTableTrait   string
	PeerTableError   string
	PeerTableModule  string
	PeerTableSnake   string
	PeerTableStruct  string
	PeerTableConfig  string
	PeerTableAdapter string
	PeerTableName    string
	SenderType       string
	PushEnum         string
	PushModule       string
	TickStruct       string
	TickConfig       string
	TickError        string
	TickModule       string
	TickName         string
	DefaultSessions  int
	DefaultTickMs    int
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

func rpcNames(spec *grpcrust.Spec) map[string]bool {
	names := map[string]bool{}

	for _, svc := range spec.Services {
		for _, r := range svc.Rpcs {
			names[r.Name] = true
		}
	}

	return names
}

func checkSessionNames(spec *grpcrust.Spec, opts Options) error {
	names := rpcNames(spec)

	if opts.Hello != "" && !names[opts.Hello] {
		return fmt.Errorf("naming the hello rpc: %q is not an rpc of package %q, layout.hello names one rpc of the proto service block", opts.Hello, spec.Package)
	}

	for _, name := range opts.Push {
		if !names[name] {
			return fmt.Errorf("naming the push rpcs: %q is not an rpc of package %q, layout.push names rpcs of the proto service block", name, spec.Package)
		}

		if name == opts.Hello {
			return fmt.Errorf("naming the push rpcs: %q is the hello rpc, a hello is inbound and cannot be pushed", name)
		}
	}

	if len(opts.Push) > 0 && opts.Hello == "" {
		return fmt.Errorf("naming the push rpcs: %s pushed and no layout.hello named, a push reaches the peers a hello admitted", strings.Join(opts.Push, ", "))
	}

	return nil
}

func buildPortViews(opts Options) ([]portView, error) {
	views := make([]portView, 0, len(opts.Ports))
	seen := map[string]bool{}

	for _, spec := range opts.Ports {
		if !pascalIdent.MatchString(spec.Name) {
			return nil, fmt.Errorf("naming the controller ports: %q is not a Pascal case Rust ident, layout.ports names port traits", spec.Name)
		}

		if seen[spec.Name] {
			return nil, fmt.Errorf("naming the controller ports: %q is listed twice", spec.Name)
		}

		seen[spec.Name] = true

		view, err := buildPortView(spec)
		if err != nil {
			return nil, err
		}

		views = append(views, view)
	}

	return views, nil
}

func applyGate(sv *serviceView, opts Options) error {
	if opts.Gate.Adapters == nil {
		if opts.Gate.Field != "" {
			return fmt.Errorf("declaring the session gate: it names field %q and no adapters, a field says what a secret adapter compares and adapters says to build one", opts.Gate.Field)
		}

		return nil
	}

	if len(*opts.Gate.Adapters) == 0 {
		return fmt.Errorf("declaring the session gate: the adapters list is empty, a gate nobody can build is a gate nobody can use")
	}

	for _, kind := range *opts.Gate.Adapters {
		if kind != GateAdapterSecret {
			return fmt.Errorf("declaring the session gate: adapter kind %q, a gate adapter is one of %s", kind, strings.Join(GateAdapterKinds(), ", "))
		}
	}

	if opts.Gate.Field == "" {
		return fmt.Errorf("declaring the session gate: the secret adapter names no field, field names the property of %s it compares to the configured secret", sv.HelloRpc.Request)
	}

	ident, err := gateField(sv, opts.Gate.Field)
	if err != nil {
		return err
	}

	sv.GateSecret = true
	sv.GateSecretField = ident
	sv.GateSecretName = sv.ServiceSnake + "_session_gate_" + GateAdapterSecret
	sv.GateSecretStruct = sv.ServicePascal + "SessionGateSecret"
	sv.GateSecretConfig = sv.ServicePascal + "SessionGateSecretConfig"
	sv.GateSecretModule = sv.ServiceSnake + "_session_gate_secret"

	return nil
}

func gateField(sv *serviceView, field string) (string, error) {
	ident := rustname.RustIdent(field)

	for _, message := range sv.Messages {
		if message.Name != sv.HelloRpc.Request {
			continue
		}

		declared := []string{}

		for _, f := range message.Fields {
			declared = append(declared, f.Ident)

			if f.Ident != ident {
				continue
			}

			if f.RustType != "String" {
				return "", fmt.Errorf("declaring the session gate: the secret adapter compares field %q of %s, which is a %s and a secret is a string", field, message.Name, f.RustType)
			}

			return ident, nil
		}

		return "", fmt.Errorf("declaring the session gate: the secret adapter compares field %q, which %s does not declare, it declares %s", field, message.Name, strings.Join(declared, ", "))
	}

	return "", fmt.Errorf("declaring the session gate: the hello request %s is not a message of this proto", sv.HelloRpc.Request)
}

func buildPortView(spec PortSpec) (portView, error) {
	snake := rustname.Snake(spec.Name)

	if spec.Kind == "" {
		if spec.Adapters != nil {
			return portView{}, fmt.Errorf("naming the controller ports: %q names adapters and no kind, adapters says how to build a port and kind says what the port is", spec.Name)
		}

		return portView{Name: spec.Name, Snake: snake}, nil
	}

	if spec.Kind != CounterPortKind {
		return portView{}, fmt.Errorf("naming the controller ports: %q is of kind %q, the declared kinds are %s, an entry with no kind names a port another cell provides", spec.Name, spec.Kind, strings.Join(PortKinds(), ", "))
	}

	if spec.Adapters == nil {
		return portView{}, fmt.Errorf("naming the controller ports: counter port %q names no adapters, adapters lists which of %s the engine emits", spec.Name, strings.Join(CounterAdapterKinds(), ", "))
	}

	if len(*spec.Adapters) == 0 {
		return portView{}, fmt.Errorf("naming the controller ports: counter port %q names an empty adapters list, a counter nobody can build is a counter nobody can use", spec.Name)
	}

	view := portView{
		Name:               spec.Name,
		Snake:              snake,
		Kind:               spec.Kind,
		Methods:            []string{"fn next(&self) -> u64;"},
		Generated:          true,
		MemoryStruct:       spec.Name + "Memory",
		MemoryConfigStruct: spec.Name + "MemoryConfig",
		MemoryAdapterName:  snake + "_memory",
		MemoryModule:       snake + "_memory",
	}

	for _, kind := range *spec.Adapters {
		if kind != CounterAdapterMemory {
			return portView{}, fmt.Errorf("naming the controller ports: counter port %q names adapter kind %q, a counter adapter is one of %s", spec.Name, kind, strings.Join(CounterAdapterKinds(), ", "))
		}

		view.HasMemory = true
	}

	return view, nil
}

func isPush(name string, opts Options) bool {
	for _, push := range opts.Push {
		if push == name {
			return true
		}
	}

	return false
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

	ports, err := buildPortViews(opts)
	if err != nil {
		return serviceView{}, err
	}

	driverName := opts.Cell
	clientName := opts.Cell + "_client"
	broadcastName := opts.Cell + "_broadcast"
	peerTableName := opts.Cell + "_peer_table"
	tickName := "tick"

	if !only {
		driverName = opts.Cell + "_" + rustname.Snake(svc.Name)
		clientName = opts.Cell + "_" + rustname.Snake(svc.Name) + "_client"
		broadcastName = opts.Cell + "_" + rustname.Snake(svc.Name) + "_broadcast"
		peerTableName = opts.Cell + "_" + rustname.Snake(svc.Name) + "_peer_table"
		tickName = "tick_" + rustname.Snake(svc.Name)
	}

	pascal := rustname.Pascal(svc.Name)
	snake := rustname.Snake(svc.Name)

	sv := serviceView{
		Header:           header,
		Ports:            ports,
		Package:          spec.Package,
		Cell:             opts.Cell,
		CratePath:        "crate::" + opts.Cell + "::",
		ModulePrefix:     opts.Cell + "::",
		SchemaVersion:    version,
		ServicePascal:    pascal,
		ServiceSnake:     snake,
		ClientTrait:      pascal + "Client",
		ClientError:      pascal + "ClientError",
		ClientStruct:     pascal + "UdpClient",
		ClientConfig:     pascal + "UdpClientConfig",
		ClientModule:     snake + "_udp_client",
		ClientName:       clientName,
		DriverStruct:     pascal + "UdpDriver",
		DriverConfig:     pascal + "UdpDriverConfig",
		DriverError:      pascal + "UdpDriverError",
		DriverModule:     snake + "_udp_driver",
		DriverName:       driverName,
		DefaultAddress:   DefaultAddress,
		DefaultEndpoint:  DefaultEndpoint,
		DefaultTimeoutMs: DefaultTimeoutMs,
		CodecError:       pascal + "CodecError",
		RequestEnum:      pascal + "Request",
		ControllerSnake:  snake,
		ControllerTrait:  pascal + "Controller",
		ControllerError:  pascal + "ControllerError",
		GateTrait:        pascal + "SessionGate",
		GateError:        pascal + "SessionGateError",
		GateModule:       snake + "_session_gate",
		GateSnake:        snake + "_session_gate",
		BroadcastTrait:   pascal + "Broadcast",
		BroadcastError:   pascal + "BroadcastError",
		BroadcastModule:  snake + "_broadcast",
		BroadcastSnake:   snake + "_broadcast",
		BroadcastStruct:  pascal + "UdpBroadcast",
		BroadcastConfig:  pascal + "UdpBroadcastConfig",
		BroadcastAdapter: snake + "_udp_broadcast",
		BroadcastName:    broadcastName,
		PeerTableTrait:   pascal + "PeerTable",
		PeerTableError:   pascal + "PeerTableError",
		PeerTableModule:  snake + "_peer_table",
		PeerTableSnake:   snake + "_peer_table",
		PeerTableStruct:  pascal + "UdpPeerTable",
		PeerTableConfig:  pascal + "UdpPeerTableConfig",
		PeerTableAdapter: snake + "_udp_peer_table",
		PeerTableName:    peerTableName,
		SenderType:       pascal + "Sender",
		PushEnum:         pascal + "Push",
		PushModule:       snake + "_push",
		TickStruct:       pascal + "TickDriver",
		TickConfig:       pascal + "TickDriverConfig",
		TickError:        pascal + "TickDriverError",
		TickModule:       snake + "_tick_driver",
		TickName:         tickName,
		DefaultSessions:  DefaultSessions,
		DefaultTickMs:    DefaultTickMs,
	}

	for _, m := range messages {
		sv.Messages = append(sv.Messages, buildMessageView(m))
	}

	seenTrait := map[string]bool{}
	seenCodec := map[string]bool{}
	seenPush := map[string]bool{}

	for _, r := range svc.Rpcs {
		fullMethod := spec.Package + "." + svc.Name + "/" + r.Name
		hash := FunctionHash(fullMethod)

		rv := rpcView{
			Ident:      rustname.RustIdent(r.Name),
			Pascal:     rustname.Pascal(r.Name),
			Upper:      rustname.Upper(r.Name),
			Request:    rustname.Pascal(r.Request),
			Reply:      rustname.Pascal(r.Response),
			FullMethod: fullMethod,
			Hash:       hash,
			Silent:     rustname.Pascal(r.Response) == NothingMessage,
			Hello:      opts.Hello != "" && r.Name == opts.Hello,
			Push:       isPush(r.Name, opts),
		}

		sv.Rpcs = append(sv.Rpcs, rv)

		if rv.Hello {
			sv.Session = true
			sv.HelloRpc = rv

			if err := applyGate(&sv, opts); err != nil {
				return serviceView{}, err
			}
		}

		if rv.Push {
			sv.Pushes = append(sv.Pushes, rv)

			if !seenPush[rv.Request] {
				seenPush[rv.Request] = true

				sv.PushTypes = append(sv.PushTypes, rv.Request)
			}

			continue
		}

		sv.Inbound = append(sv.Inbound, rv)

		for _, t := range []string{rv.Request, rv.Reply} {
			if !seenCodec[t] {
				seenCodec[t] = true

				sv.CodecTypes = append(sv.CodecTypes, t)
			}

			if !seenTrait[t] {
				seenTrait[t] = true

				sv.TraitTypes = append(sv.TraitTypes, t)
			}
		}
	}

	for _, t := range sv.PushTypes {
		if !seenCodec[t] {
			seenCodec[t] = true

			sv.CodecTypes = append(sv.CodecTypes, t)
		}
	}

	if len(sv.Pushes) > 0 && !sv.Session {
		return serviceView{}, fmt.Errorf("building service %q: it pushes %s and holds no hello rpc, a push reaches the peers a hello admitted", svc.Name, pushNames(sv.Pushes))
	}

	return sv, nil
}

func pushNames(pushes []rpcView) string {
	names := make([]string, 0, len(pushes))
	for _, p := range pushes {
		names = append(names, p.Pascal)
	}

	return strings.Join(names, ", ")
}
