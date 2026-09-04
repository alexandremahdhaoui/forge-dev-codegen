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

// FieldKind names what a message field holds.
type FieldKind string

const (
	FieldScalar  FieldKind = "scalar"
	FieldMessage FieldKind = "message"
)

// Field is one field of a Message.
type Field struct {
	Name    string
	Number  int
	Kind    FieldKind
	Scalar  string
	Message string
}

// Message is one proto3 message with scalar and message fields only.
type Message struct {
	Name   string
	Fields []Field
}

// Rpc is one unary RPC of a Service.
type Rpc struct {
	Name     string
	Request  string
	Response string
}

// Service is one proto3 service with unary RPCs only.
type Service struct {
	Name string
	Rpcs []Rpc
}

// Spec is the parsed proto3 document.
type Spec struct {
	Package  string
	Messages []Message
	Services []Service
}

// Options controls where Generate answers its files.
type Options struct {
	Service string
	CoreDir string
	AppDir  string
}

// File is one answered file.
type File struct {
	Path    string
	Content string
}

func messageByName(messages []Message, name string) (Message, bool) {
	for _, m := range messages {
		if m.Name == name {
			return m, true
		}
	}

	return Message{}, false
}
