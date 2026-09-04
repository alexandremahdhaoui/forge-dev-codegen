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

package testenvsqlite

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type Row struct {
	ID   string
	Body string
}

type vector struct {
	Case            string          `json:"case"`
	Operation       string          `json:"operation"`
	ControllerReply json.RawMessage `json:"controllerReply"`
}

type vectorsFile struct {
	Cases []vector `json:"cases"`
}

func Seeds(vectors []byte, stores []Store) (map[string][]Row, error) {
	var file vectorsFile
	if err := json.Unmarshal(vectors, &file); err != nil {
		return nil, fmt.Errorf("reading the vectors file: %w", err)
	}

	rows := map[string][]Row{}

	for _, v := range file.Cases {
		if !strings.HasPrefix(strings.ToLower(v.Operation), "create") || len(v.ControllerReply) == 0 {
			continue
		}

		var reply map[string]json.RawMessage
		if err := json.Unmarshal(v.ControllerReply, &reply); err != nil {
			return nil, fmt.Errorf("reading the controllerReply of case %q: %w", v.Case, err)
		}

		store, ok := storeOf(v.Operation, reply, stores)
		if !ok {
			continue
		}

		var id string
		if err := json.Unmarshal(reply["id"], &id); err != nil {
			return nil, fmt.Errorf("reading the id of case %q: %w", v.Case, err)
		}

		var body bytes.Buffer
		if err := json.Compact(&body, v.ControllerReply); err != nil {
			return nil, fmt.Errorf("compacting the controllerReply of case %q: %w", v.Case, err)
		}

		rows[store.Snake] = append(rows[store.Snake], Row{ID: id, Body: body.String()})
	}

	return rows, nil
}

func storeOf(operation string, reply map[string]json.RawMessage, stores []Store) (Store, bool) {
	if _, ok := reply["id"]; !ok {
		return Store{}, false
	}

	named := strings.ToLower(strings.TrimPrefix(operation, "create"))

	for _, s := range stores {
		if strings.ToLower(s.Name) == named && covers(reply, s.Required) {
			return s, true
		}
	}

	for _, s := range stores {
		if covers(reply, s.Required) {
			return s, true
		}
	}

	return Store{}, false
}

func covers(reply map[string]json.RawMessage, required []string) bool {
	for _, key := range required {
		if _, ok := reply[key]; !ok {
			return false
		}
	}

	return true
}
