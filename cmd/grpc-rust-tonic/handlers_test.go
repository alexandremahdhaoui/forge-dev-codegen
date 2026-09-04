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

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const smallProto = `
syntax = "proto3";

package songe.hello.v1;

service Hello {
  rpc Ping(PingRequest) returns (PingReply);
}

message PingRequest {
  string message = 1;
}

message PingReply {
  string message = 1;
}
`

func TestTheEngineFillsTheGrpcRustCellOnly(t *testing.T) {
	generate := NewHandlers().Generate

	if _, err := generate(context.Background(), GenerateInput{Name: "svc", Kind: "rest-api", ProtoSpec: smallProto}); err == nil {
		t.Error("the rest-api kind must be refused")
	}

	if _, err := generate(context.Background(), GenerateInput{Name: "svc", Kind: "grpc", Language: "go", ProtoSpec: smallProto}); err == nil {
		t.Error("the go language must be refused")
	}

	out, err := generate(context.Background(), GenerateInput{Name: "svc", Kind: "grpc", Language: "rust", ProtoSpec: smallProto})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	if len(out.Files) == 0 {
		t.Fatal("no files answered")
	}
}

func TestTheOutputRootsComeFromTheTopLevelOrTheSurface(t *testing.T) {
	out, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "grpc", ProtoSpec: smallProto,
		Surface: map[string]interface{}{"coreDir": "../svc-core", "appDir": "../svc-app"},
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	for _, f := range out.Files {
		if !strings.HasPrefix(f.Path, "../svc-core/") && !strings.HasPrefix(f.Path, "../svc-app/") {
			t.Errorf("%s ignores the surface roots", f.Path)
		}
	}

	out, err = NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "grpc", ProtoSpec: smallProto, CoreDir: "c", AppDir: "a",
		Surface: map[string]interface{}{"coreDir": "../svc-core"},
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	for _, f := range out.Files {
		if !strings.HasPrefix(f.Path, "c/") && !strings.HasPrefix(f.Path, "a/") {
			t.Errorf("%s ignores the top level roots", f.Path)
		}
	}
}

func TestTheSurfaceSideAnswersOneCrateAtItsOwnRoot(t *testing.T) {
	tests := []struct {
		side   string
		dirKey string
		want   []string
		reject []string
	}{
		{
			side:   "core",
			dirKey: "coreDir",
			want:   []string{"src/controller/zz_generated_hello_controller.rs", "src/hand/hello_controller.rs"},
			reject: []string{"src/adapter/zz_generated_hello_grpc_client.rs", "zz_generated_build.rs"},
		},
		{
			side:   "app",
			dirKey: "appDir",
			want:   []string{"src/driver/zz_generated_hello_grpc_driver.rs", "zz_generated_build.rs"},
			reject: []string{"src/port/zz_generated_hello_client.rs", "src/hand/hello_controller.rs"},
		},
	}

	for _, tt := range tests {
		t.Run("the "+tt.side+" side answers only its own files", func(t *testing.T) {
			out, err := NewHandlers().Generate(context.Background(), GenerateInput{
				Name: "svc", Kind: "grpc", ProtoSpec: smallProto,
				Surface: map[string]interface{}{"side": tt.side, tt.dirKey: "."},
			})
			if err != nil {
				t.Fatalf("generating: %v", err)
			}

			got := map[string]bool{}
			for _, f := range out.Files {
				got[f.Path] = true
			}

			for _, path := range tt.want {
				if !got[path] {
					t.Errorf("the %s side did not answer %s", tt.side, path)
				}
			}

			for _, path := range tt.reject {
				if got[path] {
					t.Errorf("the %s side answered %s, which belongs to the other crate", tt.side, path)
				}
			}
		})
	}
}

func TestASurfaceSideThatNamesNeitherCrateIsRefused(t *testing.T) {
	_, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "grpc", ProtoSpec: smallProto,
		Surface: map[string]interface{}{"side": "both"},
	})
	if err == nil || !strings.Contains(err.Error(), "side must be core, app or empty") {
		t.Fatalf("want an error refusing the side, got %v", err)
	}
}

func TestAHandControllerThatExistsIsNeverAnsweredAgain(t *testing.T) {
	srcDir := t.TempDir()
	hand := filepath.Join(srcDir, "core", "src", "hand", "hello_controller.rs")

	if err := os.MkdirAll(filepath.Dir(hand), 0o755); err != nil {
		t.Fatalf("making the hand dir: %v", err)
	}

	if err := os.WriteFile(hand, []byte("the author's body"), 0o644); err != nil {
		t.Fatalf("writing the hand file: %v", err)
	}

	out, err := NewHandlers().Generate(context.Background(), GenerateInput{
		Name: "svc", Kind: "grpc", ProtoSpec: smallProto, SrcDir: srcDir,
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	for _, f := range out.Files {
		if strings.Contains(f.Path, "/hand/") {
			t.Errorf("%s was answered although it exists", f.Path)
		}
	}
}
