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

package testenvstack

import (
	"context"
	"strings"
	"testing"
)

func TestAfterValidateRefusesAnExportShapeThatNamesNoValueOrTwo(t *testing.T) {
	tests := []struct {
		after  After
		reason string
	}{
		{after: After{Command: "seed", Regex: "(.*)"}, reason: "export is required"},
		{after: After{Command: "seed", Export: "ID"}, reason: "one of regex and jsonPath is required and neither is set"},
		{after: After{Command: "seed", Export: "ID", Regex: "(.*)", JSONPath: "store.id"}, reason: "only one may be set"},
		{after: After{Command: "seed", Export: "ID", Regex: "(["}, reason: "reading the regex"},
		{after: After{Command: "seed", Export: "ID", Regex: "id"}, reason: `the regex "id" has 0 capturing groups and exactly one is required`},
		{after: After{Command: "seed", Export: "ID", Regex: "(a)(b)"}, reason: "has 2 capturing groups"},
	}

	for _, tt := range tests {
		err := tt.after.Validate()
		if err == nil || !strings.Contains(err.Error(), tt.reason) {
			t.Errorf("%+v: got %v", tt.after, err)
		}
	}
}

func TestRunAfterCapturesTheValueTheRegexGroupNamesOutOfStdout(t *testing.T) {
	value, err := RunAfter(context.Background(), t.TempDir(), nil, Placeholders{}, After{
		Command: "printf",
		Args:    []string{`{"model_id":"01ABC"}`},
		Export:  "MODEL_ID",
		Regex:   `"model_id":"([^"]+)"`,
	})
	if err != nil || value != "01ABC" {
		t.Errorf("got %q %v", value, err)
	}
}

func TestRunAfterCapturesTheValueTheJsonPathNamesOutOfStdout(t *testing.T) {
	value, err := RunAfter(context.Background(), t.TempDir(), nil, Placeholders{}, After{
		Command:  "printf",
		Args:     []string{`{"store":{"id":"01STORE","name":"songe"}}`},
		Export:   "STORE_ID",
		JSONPath: "store.id",
	})
	if err != nil || value != "01STORE" {
		t.Errorf("got %q %v", value, err)
	}
}

func TestRunAfterSubstitutesThePlaceholdersIntoItsArgs(t *testing.T) {
	placeholders := Placeholders{Ports: map[string]int{"http": 4242}, TmpDir: "/tmp/stack"}

	value, err := RunAfter(context.Background(), t.TempDir(), nil, placeholders, After{
		Command: "printf",
		Args:    []string{"url=http://127.0.0.1:@port.http@ db=@tmpDir@/x.db"},
		Export:  "URL",
		Regex:   `url=(\S+)`,
	})
	if err != nil || value != "http://127.0.0.1:4242" {
		t.Errorf("got %q %v", value, err)
	}

	_, err = RunAfter(context.Background(), t.TempDir(), nil, placeholders, After{
		Command: "printf",
		Args:    []string{"@port.grpc@"},
		Export:  "URL",
		Regex:   `(.+)`,
	})
	if err == nil || !strings.Contains(err.Error(), `port "grpc" is not declared`) {
		t.Errorf("got %v", err)
	}
}

func TestRunAfterReadsTheEnvironmentTheStackHasExportedSoFar(t *testing.T) {
	value, err := RunAfter(context.Background(), t.TempDir(), map[string]string{"STORE_ID": "01STORE"}, Placeholders{}, After{
		Command: "sh",
		Args:    []string{"-c", `printf "id=%s" "$STORE_ID"`},
		Export:  "MODEL_ID",
		Regex:   `id=(\S+)`,
	})
	if err != nil || value != "01STORE" {
		t.Errorf("got %q %v", value, err)
	}
}

func TestRunAfterReportsACommandThatFailsAndAPatternThatMatchesNothing(t *testing.T) {
	_, err := RunAfter(context.Background(), t.TempDir(), nil, Placeholders{}, After{
		Command: "/nonexistent/command",
		Export:  "ID",
		Regex:   `(.+)`,
	})
	if err == nil || !strings.Contains(err.Error(), `running the after command "/nonexistent/command"`) {
		t.Errorf("got %v", err)
	}

	_, err = RunAfter(context.Background(), t.TempDir(), nil, Placeholders{}, After{
		Command: "printf",
		Args:    []string{"nothing here"},
		Export:  "ID",
		Regex:   `id=(\S+)`,
	})
	if err == nil || !strings.Contains(err.Error(), `exporting ID: the regex "id=(\\S+)" matched nothing in the answer "nothing here"`) {
		t.Errorf("got %v", err)
	}
}

func TestRunAfterReportsAnAnswerThatIsNotJsonAndAPathThatNamesNothing(t *testing.T) {
	_, err := RunAfter(context.Background(), t.TempDir(), nil, Placeholders{}, After{
		Command: "printf", Args: []string{"not json"}, Export: "ID", JSONPath: "store.id",
	})
	if err == nil || !strings.Contains(err.Error(), "as json") {
		t.Errorf("got %v", err)
	}

	_, err = RunAfter(context.Background(), t.TempDir(), nil, Placeholders{}, After{
		Command: "printf", Args: []string{`{"store":{"name":"songe"}}`}, Export: "ID", JSONPath: "store.id",
	})
	if err == nil || !strings.Contains(err.Error(), `the json path "store.id": "id" names nothing; the keys are name`) {
		t.Errorf("got %v", err)
	}

	_, err = RunAfter(context.Background(), t.TempDir(), nil, Placeholders{}, After{
		Command: "printf", Args: []string{`{}`}, Export: "ID", JSONPath: "id",
	})
	if err == nil || !strings.Contains(err.Error(), "the keys are none") {
		t.Errorf("an empty answer must say so, got %v", err)
	}
}

func TestAJsonPathWalksArraysByIndexAndRendersEveryScalar(t *testing.T) {
	document := map[string]any{
		"stores": []any{map[string]any{"id": "one", "size": float64(3), "live": true}},
	}

	tests := []struct {
		path string
		want string
	}{
		{path: "stores.0.id", want: "one"},
		{path: "stores.0.size", want: "3"},
		{path: "stores.0.live", want: "true"},
	}

	for _, tt := range tests {
		got, err := walk(document, tt.path)
		if err != nil || got != tt.want {
			t.Errorf("%s -> %q %v", tt.path, got, err)
		}
	}
}

func TestAJsonPathRefusesAnIndexOutOfRangeAScalarItReachesIntoAndAValueNoEnvironmentTakes(t *testing.T) {
	document := map[string]any{"stores": []any{map[string]any{"id": "one"}}}

	tests := []struct {
		path   string
		reason string
	}{
		{path: "stores.7.id", reason: `"7" is not an index of the 1 item array`},
		{path: "stores.0.id.deeper", reason: `"deeper" reaches into a string which holds no field`},
		{path: "stores", reason: "answers a []interface {} and an environment variable takes a string, a number or a bool"},
	}

	for _, tt := range tests {
		_, err := walk(document, tt.path)
		if err == nil || !strings.Contains(err.Error(), tt.reason) {
			t.Errorf("%s: got %v", tt.path, err)
		}
	}
}
