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
	"regexp"
	"strings"
)

type VectorsFile struct {
	Cases     []VectorCase `json:"cases"`
	UdpCases  []VectorCase `json:"-"`
	GrpcCases []VectorCase `json:"-"`
}

type VectorCase struct {
	Case                   string          `json:"case"`
	Operation              string          `json:"operation"`
	Input                  json.RawMessage `json:"input"`
	ControllerReply        json.RawMessage `json:"controllerReply"`
	ExpectedStatus         int             `json:"expectedStatus"`
	ExpectedBody           json.RawMessage `json:"expectedBody"`
	ExpectedError          string          `json:"expectedError"`
	ExpectedErrorSubstring string          `json:"expectedErrorSubstring"`
	Bearer                 string          `json:"bearer"`
	Subject                string          `json:"subject"`
	Gate                   string          `json:"gate"`
	Session                string          `json:"session"`
	Hello                  json.RawMessage `json:"hello"`
	Reconnect              bool            `json:"reconnect"`
	ExpectDropped          bool            `json:"expectDropped"`
	ExpectPush             *ExpectPush     `json:"expectPush"`
	Seed                   *int64          `json:"seed"`
}

type ExpectPush struct {
	Rpc        string          `json:"rpc"`
	SessionIds []string        `json:"sessionIds"`
	Payload    json.RawMessage `json:"payload"`
}

func (c VectorCase) IsPush() bool {
	return c.ExpectPush != nil
}

var rustTestIdent = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

type declared struct {
	operation func(string) bool
	datagram  func(string) bool
	grpc      func(string) bool
	surfaces  []string
	rng       *rngPort
}

func checkSeed(c VectorCase, rng *rngPort) error {
	if c.Seed == nil {
		return nil
	}

	if *c.Seed < 0 {
		return fmt.Errorf("reading vector %q: seed %d is below zero, a seed is the number the mocked rng port answers", c.Case, *c.Seed)
	}

	if rng == nil {
		return fmt.Errorf("reading vector %q: it carries a seed and the cell names no rng port under layout.rng, name the trait, its module and its one method there", c.Case)
	}

	if !c.IsPush() {
		return fmt.Errorf("reading vector %q: seed belongs to a case whose controller pushes, the mocked controller draws from the rng port while it answers", c.Case)
	}

	return nil
}

func parseVectors(doc []byte, d declared) (*VectorsFile, error) {
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
	calls := []VectorCase{}

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

		if err := checkSeed(c, d.rng); err != nil {
			return nil, err
		}

		if d.grpc(c.Operation) {
			if err := checkCall(c); err != nil {
				return nil, err
			}

			calls = append(calls, c)

			continue
		}

		if d.datagram(c.Operation) {
			if c.IsPush() || c.ExpectDropped {
				datagrams = append(datagrams, c)

				continue
			}

			if len(c.ControllerReply) == 0 {
				return nil, fmt.Errorf("reading vector %q: a datagram case needs controllerReply, the reply the mocked controller answers", c.Case)
			}

			if len(c.ExpectedBody) == 0 {
				return nil, fmt.Errorf("reading vector %q: a datagram case needs expectedBody, the reply the client reads back", c.Case)
			}

			datagrams = append(datagrams, c)

			continue
		}

		if !d.operation(c.Operation) {
			return nil, fmt.Errorf("reading vector %q: operation %q names nothing the cell declares, the cell declares %s", c.Case, c.Operation, strings.Join(d.surfaces, ", "))
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
	v.GrpcCases = calls

	return &v, nil
}

func checkCall(c VectorCase) error {
	if len(c.ControllerReply) == 0 && c.ExpectedError == "" {
		return fmt.Errorf("reading vector %q: a grpc case needs controllerReply, the reply the mocked controller answers, or expectedError, the taxonomy member the controller answers and the status code it maps to", c.Case)
	}

	if len(c.ControllerReply) > 0 && c.ExpectedError != "" {
		return fmt.Errorf("reading vector %q: a grpc case answers a reply or an error, never both, drop controllerReply or expectedError", c.Case)
	}

	if len(c.ControllerReply) > 0 && c.ExpectedErrorSubstring != "" {
		return fmt.Errorf("reading vector %q: a grpc case answers a reply or an error, never both, drop controllerReply or expectedErrorSubstring", c.Case)
	}

	if c.ExpectedStatus != 0 {
		return fmt.Errorf("reading vector %q: expectedStatus is an HTTP status and a grpc case carries none, use expectedErrorSubstring for a refusal", c.Case)
	}

	if len(c.ExpectedBody) > 0 {
		return fmt.Errorf("reading vector %q: a grpc case reads controllerReply back over the wire, drop expectedBody", c.Case)
	}

	return nil
}
