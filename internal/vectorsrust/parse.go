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

package vectorsrust

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
)

type VectorsFile struct {
	Cases    []VectorCase `json:"cases"`
	UdpCases []VectorCase `json:"-"`
}

type VectorCase struct {
	Case                   string          `json:"case"`
	Operation              string          `json:"operation"`
	Input                  json.RawMessage `json:"input"`
	ControllerReply        json.RawMessage `json:"controllerReply"`
	ExpectedStatus         int             `json:"expectedStatus"`
	ExpectedBody           json.RawMessage `json:"expectedBody"`
	ExpectedErrorSubstring string          `json:"expectedErrorSubstring"`
}

var rustTestIdent = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func parseVectors(doc []byte, declaresOperation, declaresDatagram func(string) bool) (*VectorsFile, error) {
	var v VectorsFile
	if err := json.Unmarshal(doc, &v); err != nil {
		return nil, fmt.Errorf("parsing the vectors document: %w", err)
	}

	if len(v.Cases) == 0 {
		return nil, fmt.Errorf("parsing the vectors document: it declares no cases")
	}

	seen := map[string]bool{}
	kept := make([]VectorCase, 0, len(v.Cases))
	datagrams := []VectorCase{}

	for i, c := range v.Cases {
		if c.Case == "" {
			return nil, fmt.Errorf("reading vector %d: case is required, it names the generated test", i)
		}

		if !rustTestIdent.MatchString(c.Case) {
			return nil, fmt.Errorf("reading vector %q: its name is not one Rust can spell for a test function, use letters, digits and underscores and start with a letter or underscore", c.Case)
		}

		if seen[c.Case] {
			return nil, fmt.Errorf("reading vector %q: two cases share this name", c.Case)
		}

		seen[c.Case] = true

		if c.Operation == "" {
			return nil, fmt.Errorf("reading vector %q: operation is required, it names the operationId the vector exercises", c.Case)
		}

		if declaresDatagram(c.Operation) {
			if len(c.ControllerReply) == 0 {
				return nil, fmt.Errorf("reading vector %q: a datagram case needs controllerReply, the reply the mocked controller answers", c.Case)
			}

			if len(c.ExpectedBody) == 0 {
				return nil, fmt.Errorf("reading vector %q: a datagram case needs expectedBody, the reply the client reads back", c.Case)
			}

			datagrams = append(datagrams, c)

			continue
		}

		if !declaresOperation(c.Operation) {
			log.Printf("vectors-rust: skipping vector %q: operation %q is neither an OpenAPI operation nor a datagram rpc, it belongs to another transport", c.Case, c.Operation)

			continue
		}

		if c.ExpectedStatus == 0 {
			return nil, fmt.Errorf("reading vector %q: expectedStatus is required", c.Case)
		}

		if len(c.ControllerReply) == 0 && c.ExpectedErrorSubstring == "" {
			return nil, fmt.Errorf("reading vector %q: an error case needs expectedErrorSubstring, and a success case needs controllerReply", c.Case)
		}

		kept = append(kept, c)
	}

	v.Cases = kept
	v.UdpCases = datagrams

	return &v, nil
}
