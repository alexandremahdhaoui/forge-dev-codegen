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
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/hexrust"
)

type view struct {
	Header           string
	Crate            string
	Controllers      []controllerView
	TypeImports      []importView
	Tests            []testView
	NeedsBodyMatcher bool
	HasDatagrams     bool
	Datagram         datagramServiceView
	DatagramTests    []datagramTestView
}

type importView struct {
	Snake string
	Name  string
}

type controllerView struct {
	Snake  string
	Pascal string
	Ops    []opSignature
}

type opSignature struct {
	Ident      string
	ArgNames   []string
	ArgTypes   []string
	ReturnType string
}

func (s opSignature) TraitArgs() string {
	parts := make([]string, len(s.ArgNames))
	for i := range s.ArgNames {
		parts[i] = s.ArgNames[i] + ": " + s.ArgTypes[i]
	}

	return strings.Join(parts, ", ")
}

func (s opSignature) ClosureParams() string {
	if len(s.ArgNames) == 0 {
		return ""
	}

	parts := make([]string, len(s.ArgNames))
	for i, name := range s.ArgNames {
		parts[i] = "_" + name
	}

	return strings.Join(parts, ", ")
}

type testView struct {
	Name                     string
	ArmController            string
	ArmVar                   string
	ArmExpectMethod          string
	ArmClosureParams         string
	ArmReturning             string
	HasWith                  bool
	WithPredicates           string
	DriverArgs               []string
	Method                   string
	URI                      string
	HasBody                  bool
	BodyLiteral              string
	ExpectedStatus           int
	HasExpectedBody          bool
	ExpectedBodyLiteral      string
	HasExpectedSubstring     bool
	ExpectedSubstringLiteral string
}

func buildOpSignature(op hexrust.Operation) opSignature {
	sig := opSignature{Ident: op.Ident, ReturnType: "()"}
	if op.Response != "" {
		sig.ReturnType = op.Response
	}

	for _, p := range op.Params {
		sig.ArgNames = append(sig.ArgNames, p.Ident)
		sig.ArgTypes = append(sig.ArgTypes, paramArgType(p.Kind))
	}

	if op.Body != "" {
		sig.ArgNames = append(sig.ArgNames, "body")
		sig.ArgTypes = append(sig.ArgTypes, op.Body)
	}

	return sig
}

func paramArgType(kind string) string {
	if kind == "string" {
		return "&str"
	}

	return "i64"
}

func buildView(spec *hexrust.Spec, vectors *VectorsFile, datagrams *datagramService, opts Options) (view, error) {
	v := view{
		Header: header,
		Crate:  hexrust.Snake(opts.Service),
	}

	opsByID := map[string]hexrust.Operation{}
	controllerByName := map[string]hexrust.Controller{}
	usedTypes := map[string]bool{}

	for _, c := range spec.Controllers {
		controllerByName[c.Name] = c

		cv := controllerView{Snake: c.Snake, Pascal: c.Pascal}
		for _, op := range c.Operations {
			opsByID[op.ID] = op
			cv.Ops = append(cv.Ops, buildOpSignature(op))

			if op.Body != "" {
				usedTypes[op.Body] = true
			}

			if op.Response != "" {
				usedTypes[op.Response] = true
			}
		}

		v.Controllers = append(v.Controllers, cv)
	}

	for _, name := range sortedStrings(usedTypes) {
		v.TypeImports = append(v.TypeImports, importView{Snake: hexrust.Snake(name), Name: name})
	}

	for _, c := range vectors.Cases {
		op, ok := opsByID[c.Operation]
		if !ok {
			return view{}, fmt.Errorf("reading vector %q: it names operation %q, which the spec does not declare", c.Case, c.Operation)
		}

		owner := controllerByName[op.Controller]

		tv, err := buildTest(c, op, owner)
		if err != nil {
			return view{}, err
		}

		v.Tests = append(v.Tests, tv)
	}

	for i := range v.Tests {
		v.Tests[i].DriverArgs = driverArgs(v.Controllers, v.Tests[i].ArmController, v.Tests[i].ArmVar)

		if v.Tests[i].HasExpectedBody {
			v.NeedsBodyMatcher = true
		}
	}

	if datagrams != nil && len(vectors.UdpCases) > 0 {
		v.HasDatagrams = true
		v.Datagram = buildDatagramServiceView(datagrams)

		for _, c := range vectors.UdpCases {
			tv, err := buildDatagramTest(c, datagrams)
			if err != nil {
				return view{}, err
			}

			v.DatagramTests = append(v.DatagramTests, tv)
		}
	}

	return v, nil
}

type armKind int

const (
	armKindOK armKind = iota
	armKindNotFound
	armKindInvalid
	armKindNotImplemented
)

type arm struct {
	kind armKind
	id   string
}

func chooseArm(c VectorCase, op hexrust.Operation) (arm, error) {
	isOK := len(c.ControllerReply) > 0
	is2xx := c.ExpectedStatus >= 200 && c.ExpectedStatus < 300

	if isOK && !is2xx {
		return arm{}, fmt.Errorf("reading vector %q: controllerReply is present but expectedStatus is %d, a success case needs a 2xx status", c.Case, c.ExpectedStatus)
	}

	if !isOK && is2xx {
		return arm{}, fmt.Errorf("reading vector %q: expectedStatus is %d but no controllerReply is present, a 2xx status needs a success case", c.Case, c.ExpectedStatus)
	}

	if isOK {
		return arm{kind: armKindOK}, nil
	}

	switch c.ExpectedStatus {
	case 404:
		return arm{kind: armKindNotFound, id: c.ExpectedErrorSubstring}, nil
	case op.InvalidStatus:
		return arm{kind: armKindInvalid, id: c.ExpectedErrorSubstring}, nil
	case 501:
		return arm{kind: armKindNotImplemented, id: c.ExpectedErrorSubstring}, nil
	default:
		return arm{}, fmt.Errorf(
			"reading vector %q: expectedStatus %d matches none of NotFound (404), Invalid (%d) or NotImplemented (501), vectors-rust cannot choose which controller error to mock",
			c.Case, c.ExpectedStatus, op.InvalidStatus,
		)
	}
}

func buildTest(c VectorCase, op hexrust.Operation, owner hexrust.Controller) (testView, error) {
	sig := buildOpSignature(op)

	uri, hasBody, bodyLiteral, err := buildRequest(op, c.Input)
	if err != nil {
		return testView{}, fmt.Errorf("reading vector %q: %w", c.Case, err)
	}

	withPredicates, err := buildWithPredicates(op, c.Input)
	if err != nil {
		return testView{}, fmt.Errorf("reading vector %q: %w", c.Case, err)
	}

	chosen, err := chooseArm(c, op)
	if err != nil {
		return testView{}, err
	}

	returning, err := buildReturning(chosen, c, op, owner)
	if err != nil {
		return testView{}, fmt.Errorf("reading vector %q: %w", c.Case, err)
	}

	tv := testView{
		Name:                 c.Case,
		ArmController:        owner.Pascal,
		ArmVar:               owner.Snake + "_controller",
		ArmExpectMethod:      "expect_" + sig.Ident,
		ArmClosureParams:     sig.ClosureParams(),
		ArmReturning:         returning,
		HasWith:              len(withPredicates) > 0,
		WithPredicates:       strings.Join(withPredicates, ", "),
		Method:               op.Method,
		URI:                  uri,
		HasBody:              hasBody,
		BodyLiteral:          bodyLiteral,
		ExpectedStatus:       c.ExpectedStatus,
		HasExpectedBody:      len(c.ExpectedBody) > 0,
		HasExpectedSubstring: c.ExpectedErrorSubstring != "",
	}

	if tv.HasExpectedBody {
		compact, err := compactJSON(c.ExpectedBody)
		if err != nil {
			return testView{}, fmt.Errorf("reading vector %q: reading expectedBody: %w", c.Case, err)
		}

		tv.ExpectedBodyLiteral = strconv.Quote(compact)
	}

	if tv.HasExpectedSubstring {
		tv.ExpectedSubstringLiteral = strconv.Quote(c.ExpectedErrorSubstring)
	}

	return tv, nil
}

func buildRequest(op hexrust.Operation, input json.RawMessage) (uri string, hasBody bool, bodyLiteral string, err error) {
	inputMap, err := parseInput(input)
	if err != nil {
		return "", false, "", err
	}

	uri = op.Path

	for _, p := range op.Params {
		value, err := paramValue(p, inputMap)
		if err != nil {
			return "", false, "", err
		}

		uri = strings.ReplaceAll(uri, "{"+p.Name+"}", value)
	}

	if op.Body == "" {
		return uri, false, "", nil
	}

	compact, compactErr := compactJSON(input)
	if compactErr != nil {
		return "", false, "", fmt.Errorf("reading input: %w", compactErr)
	}

	return uri, true, strconv.Quote(compact), nil
}

func buildWithPredicates(op hexrust.Operation, input json.RawMessage) ([]string, error) {
	inputMap, err := parseInput(input)
	if err != nil {
		return nil, err
	}

	preds := make([]string, 0, len(op.Params)+1)

	for _, p := range op.Params {
		value, err := paramValue(p, inputMap)
		if err != nil {
			return nil, err
		}

		if p.Kind == "string" {
			preds = append(preds, "mockall::predicate::eq("+strconv.Quote(value)+")")
		} else {
			preds = append(preds, "mockall::predicate::eq("+value+"i64)")
		}
	}

	if op.Body != "" {
		compact, err := compactJSON(input)
		if err != nil {
			return nil, fmt.Errorf("reading input: %w", err)
		}

		preds = append(preds, fmt.Sprintf(
			"mockall::predicate::eq(serde_json::from_str::<%s>(%s).expect(\"decoding input\"))",
			op.Body, strconv.Quote(compact),
		))
	}

	return preds, nil
}

func parseInput(input json.RawMessage) (map[string]json.RawMessage, error) {
	inputMap := map[string]json.RawMessage{}

	if len(input) == 0 {
		return inputMap, nil
	}

	if err := json.Unmarshal(input, &inputMap); err != nil {
		return nil, fmt.Errorf("reading input: %w", err)
	}

	return inputMap, nil
}

func paramValue(p hexrust.Param, inputMap map[string]json.RawMessage) (string, error) {
	raw, ok := inputMap[p.Name]
	if !ok {
		return "", fmt.Errorf("reading input: path parameter %q is missing", p.Name)
	}

	if p.Kind != "string" {
		return strings.TrimSpace(string(raw)), nil
	}

	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("reading input: path parameter %q must be a JSON string: %w", p.Name, err)
	}

	return value, nil
}

func buildReturning(a arm, c VectorCase, op hexrust.Operation, owner hexrust.Controller) (string, error) {
	switch a.kind {
	case armKindOK:
		if op.Response == "" {
			return "Ok(())", nil
		}

		compact, err := compactJSON(c.ControllerReply)
		if err != nil {
			return "", fmt.Errorf("reading controllerReply: %w", err)
		}

		return fmt.Sprintf("Ok(serde_json::from_str::<%s>(%s).expect(\"decoding controllerReply\"))", op.Response, strconv.Quote(compact)), nil
	case armKindNotFound:
		return fmt.Sprintf("Err(%sControllerError::NotFound { id: %s.to_string() })", owner.Pascal, strconv.Quote(a.id)), nil
	case armKindInvalid:
		return fmt.Sprintf("Err(%sControllerError::Invalid { field: %s.to_string(), reason: \"generated by vectors-rust\".to_string() })", owner.Pascal, strconv.Quote(a.id)), nil
	case armKindNotImplemented:
		return fmt.Sprintf("Err(%sControllerError::NotImplemented { operation: %s.to_string() })", owner.Pascal, strconv.Quote(a.id)), nil
	default:
		return "", fmt.Errorf("choosing a mock arm: unhandled kind %v", a.kind)
	}
}

func driverArgs(controllers []controllerView, armedPascal, armedVar string) []string {
	args := make([]string, 0, len(controllers))

	for _, c := range controllers {
		if c.Pascal == armedPascal {
			args = append(args, "std::sync::Arc::new("+armedVar+")")

			continue
		}

		args = append(args, "std::sync::Arc::new(Mock"+c.Pascal+"Controller::new())")
	}

	return args
}

func compactJSON(raw json.RawMessage) (string, error) {
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return "", fmt.Errorf("compacting JSON: %w", err)
	}

	return buf.String(), nil
}

func sortedStrings(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	sort.Strings(out)

	return out
}
