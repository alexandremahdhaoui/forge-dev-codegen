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

package grpcrust_test

import (
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/grpcrust"
)

const helloProto = `
syntax = "proto3";

package songe.hello.v1;

service Hello {
  rpc Ping(PingRequest) returns (PingReply);
}

message PingRequest {
  string message = 1;
  uint64 count = 2;
}

message PingReply {
  string message = 1;
  uint64 count = 2;
}
`

func TestParsingTheHelloProtoReadsThePackageMessagesAndService(t *testing.T) {
	spec, err := grpcrust.Parse([]byte(helloProto))
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	if spec.Package != "songe.hello.v1" {
		t.Fatalf("package = %q, want %q", spec.Package, "songe.hello.v1")
	}

	if len(spec.Messages) != 2 {
		t.Fatalf("got %d messages, want 2", len(spec.Messages))
	}

	if len(spec.Services) != 1 {
		t.Fatalf("got %d services, want 1", len(spec.Services))
	}

	svc := spec.Services[0]
	if svc.Name != "Hello" {
		t.Fatalf("service name = %q, want %q", svc.Name, "Hello")
	}

	if len(svc.Rpcs) != 1 {
		t.Fatalf("got %d rpcs, want 1", len(svc.Rpcs))
	}

	rpc := svc.Rpcs[0]
	if rpc.Name != "Ping" || rpc.Request != "PingRequest" || rpc.Response != "PingReply" {
		t.Fatalf("rpc = %+v, want Ping(PingRequest) returns (PingReply)", rpc)
	}

	for _, m := range spec.Messages {
		if len(m.Fields) != 2 {
			t.Fatalf("message %q has %d fields, want 2", m.Name, len(m.Fields))
		}

		if m.Fields[0].Name != "message" || m.Fields[0].Scalar != "string" {
			t.Fatalf("message %q field 0 = %+v, want string message", m.Name, m.Fields[0])
		}

		if m.Fields[1].Name != "count" || m.Fields[1].Scalar != "uint64" {
			t.Fatalf("message %q field 1 = %+v, want uint64 count", m.Name, m.Fields[1])
		}
	}
}

func TestParsingAMessageFieldReferencingAnotherMessageMarksItAMessageKindField(t *testing.T) {
	const doc = `
syntax = "proto3";

package demo;

message Address {
  string city = 1;
}

message Player {
  string name = 1;
  Address home = 2;
}

service Players {
  rpc GetPlayer(Player) returns (Player);
}
`

	spec, err := grpcrust.Parse([]byte(doc))
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	var player grpcrust.Message

	for _, m := range spec.Messages {
		if m.Name == "Player" {
			player = m
		}
	}

	if len(player.Fields) != 2 {
		t.Fatalf("Player has %d fields, want 2", len(player.Fields))
	}

	home := player.Fields[1]
	if home.Kind != grpcrust.FieldMessage || home.Message != "Address" {
		t.Fatalf("home field = %+v, want a message field referencing Address", home)
	}
}

func TestAProtoThatBreaksTheSupportedSubsetIsRefused(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want string
	}{
		{
			name: "an import is refused",
			doc: `
syntax = "proto3";
package demo;
import "other.proto";
service S {
  rpc Do(Empty) returns (Empty);
}
message Empty {}
`,
			want: "import is not supported",
		},
		{
			name: "an option is refused",
			doc: `
syntax = "proto3";
package demo;
option go_package = "demo";
message Empty {}
`,
			want: "option is not supported",
		},
		{
			name: "an enum is refused",
			doc: `
syntax = "proto3";
package demo;
enum Kind { UNKNOWN = 0; }
`,
			want: "enum is not supported",
		},
		{
			name: "a streaming rpc is refused",
			doc: `
syntax = "proto3";
package demo;
message Empty {}
service S {
  rpc Do(stream Empty) returns (Empty);
}
`,
			want: "streaming is not supported",
		},
		{
			name: "a nested message is refused",
			doc: `
syntax = "proto3";
package demo;
message Outer {
  message Inner {
    string a = 1;
  }
}
`,
			want: "nested messages are not supported",
		},
		{
			name: "a repeated field is refused",
			doc: `
syntax = "proto3";
package demo;
message Group {
  repeated string names = 1;
}
`,
			want: "repeated fields are not supported",
		},
		{
			name: "a oneof is refused",
			doc: `
syntax = "proto3";
package demo;
message Group {
  oneof choice {
    string a = 1;
  }
}
`,
			want: "oneof is not supported",
		},
		{
			name: "a map field is refused",
			doc: `
syntax = "proto3";
package demo;
message Group {
  map<string, string> tags = 1;
}
`,
			want: "map fields are not supported",
		},
		{
			name: "a qualified field type is refused",
			doc: `
syntax = "proto3";
package demo;
message Group {
  google.protobuf.Timestamp at = 1;
}
`,
			want: "qualified field type",
		},
		{
			name: "an undefined message reference is refused",
			doc: `
syntax = "proto3";
package demo;
service S {
  rpc Do(Missing) returns (Missing);
}
`,
			want: "undefined message",
		},
		{
			name: "a non proto3 syntax is refused",
			doc: `
syntax = "proto2";
package demo;
message Empty {}
`,
			want: `only syntax "proto3" is supported`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := grpcrust.Parse([]byte(tt.doc))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want an error containing %q, got %v", tt.want, err)
			}
		})
	}
}
