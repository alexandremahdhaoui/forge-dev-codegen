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

type FieldKind string

const (
	FieldScalar  FieldKind = "scalar"
	FieldMessage FieldKind = "message"
)

type Field struct {
	Name    string
	Number  int
	Kind    FieldKind
	Scalar  string
	Message string
}

type Message struct {
	Name   string
	Fields []Field
}

type Rpc struct {
	Name     string
	Request  string
	Response string
}

type Service struct {
	Name string
	Rpcs []Rpc
}

type Spec struct {
	Package  string
	Messages []Message
	Services []Service
}

type Options struct {
	Service string
	CoreDir string
	AppDir  string
}

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
