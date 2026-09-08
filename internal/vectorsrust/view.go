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
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/restrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/rustname"
)

type view struct {
	Header           string
	Crate            string
	RestCell         string
	Auth             bool
	HasStream        bool
	Controllers      []controllerView
	TypeImports      []importView
	Tests            []testView
	NeedsBodyMatcher bool
	HasDatagrams     bool
	Datagram         datagramServiceView
	DatagramTests    []datagramTestView
	HasPush          bool
	HasRequestPush   bool
	HasReconnect     bool
	NeedsRegister    bool
	HasCalls         bool
	Call             callServiceView
	CallTests        []callTestView
	HasRng           bool
	Rng              rngPortView
	DrawIdent        string
}

type rngPortView struct {
	Trait   string
	Module  string
	Method  string
	Returns string
	Builder string
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
	ControllerNever          bool
	HasWith                  bool
	WithPredicates           string
	DriverArgs               []string
	Method                   string
	URI                      string
	HasBody                  bool
	BodyLiteral              string
	HasBearer                bool
	BearerLiteral            string
	HasSubject               bool
	SubjectLiteral           string
	Stream                   bool
	ExpectedStatus           int
	HasExpectedBody          bool
	ExpectedBodyLiteral      string
	HasExpectedSubstring     bool
	ExpectedSubstringLiteral string
}

func buildOpSignature(op restrust.Operation) opSignature {
	sig := opSignature{Ident: op.Ident, ReturnType: "()"}
	if op.Response != "" {
		sig.ReturnType = op.Response
	}

	if op.Stream {
		sig.ReturnType = "std::sync::mpsc::Receiver<" + op.Response + ">"
	}

	if op.Auth {
		sig.ArgNames = append(sig.ArgNames, "subject")
		sig.ArgTypes = append(sig.ArgTypes, "Subject")
	}

	for _, p := range op.Params {
		sig.ArgNames = append(sig.ArgNames, p.Ident)
		sig.ArgTypes = append(sig.ArgTypes, paramArgType(p.Kind))
	}

	for _, q := range op.Query {
		sig.ArgNames = append(sig.ArgNames, q.Ident)
		sig.ArgTypes = append(sig.ArgTypes, queryArgType(q))
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

func queryArgType(q restrust.QueryParam) string {
	if q.Required {
		return paramArgType(q.Kind)
	}

	if q.Kind == "string" {
		return "Option<String>"
	}

	return "Option<i64>"
}

func buildView(spec *restrust.Spec, vectors *VectorsFile, datagrams *datagramService, calls *callService, rng *rngPort, opts Options) (view, error) {
	v := view{
		Header:    header,
		Crate:     rustname.Snake(opts.Service),
		RestCell:  opts.RestCell,
		Auth:      spec.Auth,
		DrawIdent: drawIdent,
	}

	if rng != nil {
		v.HasRng = true
		v.Rng = rngPortView{
			Trait:   rng.Trait,
			Module:  rng.Module,
			Method:  rng.Method,
			Returns: rng.Returns,
			Builder: "seeded_" + rustname.Snake(rng.Trait),
		}
	}

	opsByID := map[string]restrust.Operation{}
	controllerByName := map[string]restrust.Controller{}
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
		v.TypeImports = append(v.TypeImports, importView{Snake: rustname.Snake(name), Name: name})
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
		v.Tests[i].DriverArgs = driverArgs(v.Controllers, v.Tests[i].ArmController, v.Tests[i].ArmVar, spec.Auth)

		if v.Tests[i].HasExpectedBody {
			v.NeedsBodyMatcher = true
		}

		v.HasStream = v.HasStream || v.Tests[i].Stream
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

			v.HasPush = v.HasPush || tv.Kind == kindPush || tv.Kind == kindRequestPush
			v.HasRequestPush = v.HasRequestPush || tv.Kind == kindRequestPush
			v.HasReconnect = v.HasReconnect || tv.Kind == kindReconnect
			v.NeedsRegister = v.NeedsRegister || tv.Kind == kindPush || tv.Kind == kindRequestPush || tv.Kind == kindReconnect || (tv.Registered && !tv.IsHello)
		}
	}

	if calls != nil && len(vectors.GrpcCases) > 0 {
		v.HasCalls = true
		v.Call = buildCallServiceView(calls)

		for _, c := range vectors.GrpcCases {
			tv, err := buildCallTest(c, calls, v.Call)
			if err != nil {
				return view{}, err
			}

			v.CallTests = append(v.CallTests, tv)
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
	armKindUnauthenticated
	armKindAuthentication
	armKindAuthorization
	armKindSemantic
	armKindRateLimited
	armKindMissingQuery
)

type arm struct {
	kind armKind
	id   string
}

func chooseArm(c VectorCase, op restrust.Operation, missingQuery []string) (arm, error) {
	isOK := len(c.ControllerReply) > 0
	is2xx := c.ExpectedStatus >= 200 && c.ExpectedStatus < 300

	if isOK && !is2xx {
		return arm{}, fmt.Errorf("reading vector %q: controllerReply is present but expectedStatus is %d, a success case needs a 2xx status", c.Case, c.ExpectedStatus)
	}

	if !isOK && is2xx {
		return arm{}, fmt.Errorf("reading vector %q: expectedStatus is %d but no controllerReply is present, a 2xx status needs a success case", c.Case, c.ExpectedStatus)
	}

	if len(missingQuery) > 0 {
		if isOK {
			return arm{}, fmt.Errorf("reading vector %q: input names no value for the required query parameter %s, and the driver refuses before the controller answers", c.Case, strings.Join(missingQuery, ", "))
		}

		if c.ExpectedStatus != op.InvalidStatus {
			return arm{}, fmt.Errorf("reading vector %q: input names no value for the required query parameter %s, so the driver answers %d, not %d", c.Case, strings.Join(missingQuery, ", "), op.InvalidStatus, c.ExpectedStatus)
		}

		return arm{kind: armKindMissingQuery}, nil
	}

	if isOK {
		return arm{kind: armKindOK}, nil
	}

	if c.ExpectedStatus == 401 && op.Auth {
		return arm{kind: armKindUnauthenticated, id: c.ExpectedErrorSubstring}, nil
	}

	switch c.ExpectedStatus {
	case 401:
		return arm{kind: armKindAuthentication, id: c.ExpectedErrorSubstring}, nil
	case 403:
		return arm{kind: armKindAuthorization, id: c.ExpectedErrorSubstring}, nil
	case 404:
		return arm{kind: armKindNotFound, id: c.ExpectedErrorSubstring}, nil
	case op.InvalidStatus:
		return arm{kind: armKindInvalid, id: c.ExpectedErrorSubstring}, nil
	case 409:
		return arm{kind: armKindSemantic, id: c.ExpectedErrorSubstring}, nil
	case 429:
		return arm{kind: armKindRateLimited, id: c.ExpectedErrorSubstring}, nil
	case 501:
		return arm{kind: armKindNotImplemented, id: c.ExpectedErrorSubstring}, nil
	default:
		return arm{}, fmt.Errorf(
			"reading vector %q: expectedStatus %d matches none of Authentication (401), Authorization (403), NotFound (404), Invalid (%d), Semantic (409), RateLimited (429), NotImplemented (501) or a refused bearer on an x-auth operation (401), vectors-rust cannot choose which controller error to mock",
			c.Case, c.ExpectedStatus, op.InvalidStatus,
		)
	}
}

func checkBearer(c VectorCase, op restrust.Operation) error {
	if !op.Auth && (c.Bearer != "" || c.Subject != "") {
		return fmt.Errorf("reading vector %q: bearer and subject belong to an x-auth operation, and %q carries no x-auth", c.Case, op.ID)
	}

	if c.Subject != "" && c.Bearer == "" {
		return fmt.Errorf("reading vector %q: subject names what the verifier answers for bearer, and there is no bearer", c.Case)
	}

	if op.Auth && len(c.ControllerReply) > 0 && c.Subject == "" {
		return fmt.Errorf("reading vector %q: a success case on the x-auth operation %q needs a bearer and the subject the verifier answers for it", c.Case, op.ID)
	}

	return nil
}

func buildTest(c VectorCase, op restrust.Operation, owner restrust.Controller) (testView, error) {
	sig := buildOpSignature(op)

	if err := checkBearer(c, op); err != nil {
		return testView{}, err
	}

	missingQuery, err := missingRequiredQuery(op, c.Input)
	if err != nil {
		return testView{}, fmt.Errorf("reading vector %q: %w", c.Case, err)
	}

	uri, hasBody, bodyLiteral, err := buildRequest(op, c.Input)
	if err != nil {
		return testView{}, fmt.Errorf("reading vector %q: %w", c.Case, err)
	}

	chosen, err := chooseArm(c, op, missingQuery)
	if err != nil {
		return testView{}, err
	}

	withPredicates := []string{}

	if chosen.kind != armKindMissingQuery {
		withPredicates, err = buildWithPredicates(op, c)
		if err != nil {
			return testView{}, fmt.Errorf("reading vector %q: %w", c.Case, err)
		}
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
		ControllerNever:      chosen.kind == armKindUnauthenticated || chosen.kind == armKindMissingQuery,
		HasWith:              len(withPredicates) > 0,
		WithPredicates:       strings.Join(withPredicates, ", "),
		Method:               op.Method,
		URI:                  uri,
		HasBody:              hasBody,
		BodyLiteral:          bodyLiteral,
		HasBearer:            c.Bearer != "",
		BearerLiteral:        strconv.Quote(c.Bearer),
		HasSubject:           c.Subject != "",
		SubjectLiteral:       strconv.Quote(c.Subject),
		Stream:               op.Stream,
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

func buildRequest(op restrust.Operation, input json.RawMessage) (uri string, hasBody bool, bodyLiteral string, err error) {
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

	query, err := buildQuery(op, inputMap)
	if err != nil {
		return "", false, "", err
	}

	uri += query

	if op.Body == "" {
		return uri, false, "", nil
	}

	compact, compactErr := compactJSON(input)
	if compactErr != nil {
		return "", false, "", fmt.Errorf("reading input: %w", compactErr)
	}

	return uri, true, strconv.Quote(compact), nil
}

func buildWithPredicates(op restrust.Operation, c VectorCase) ([]string, error) {
	input := c.Input

	inputMap, err := parseInput(input)
	if err != nil {
		return nil, err
	}

	preds := make([]string, 0, len(op.Params)+2)

	if op.Auth {
		preds = append(preds, "mockall::predicate::eq(Subject { id: "+strconv.Quote(c.Subject)+".to_string() })")
	}

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

	for _, q := range op.Query {
		pred, err := queryPredicate(q, inputMap)
		if err != nil {
			return nil, err
		}

		preds = append(preds, pred)
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

func missingRequiredQuery(op restrust.Operation, input json.RawMessage) ([]string, error) {
	inputMap, err := parseInput(input)
	if err != nil {
		return nil, err
	}

	missing := []string{}

	for _, q := range op.Query {
		if _, named := inputMap[q.Name]; !named && q.Required {
			missing = append(missing, strconv.Quote(q.Name))
		}
	}

	return missing, nil
}

func buildQuery(op restrust.Operation, inputMap map[string]json.RawMessage) (string, error) {
	pairs := []string{}

	for _, q := range op.Query {
		value, named, err := queryValue(q, inputMap)
		if err != nil {
			return "", err
		}

		if !named {
			continue
		}

		pairs = append(pairs, url.QueryEscape(q.Name)+"="+url.QueryEscape(value))
	}

	if len(pairs) == 0 {
		return "", nil
	}

	return "?" + strings.Join(pairs, "&"), nil
}

func queryValue(q restrust.QueryParam, inputMap map[string]json.RawMessage) (string, bool, error) {
	raw, named := inputMap[q.Name]
	if !named {
		return "", false, nil
	}

	if q.Kind != "string" {
		var number int64
		if err := json.Unmarshal(raw, &number); err != nil {
			return "", false, fmt.Errorf("reading input: query parameter %q must be a JSON integer: %w", q.Name, err)
		}

		return strconv.FormatInt(number, 10), true, nil
	}

	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false, fmt.Errorf("reading input: query parameter %q must be a JSON string: %w", q.Name, err)
	}

	return value, true, nil
}

func queryPredicate(q restrust.QueryParam, inputMap map[string]json.RawMessage) (string, error) {
	value, named, err := queryValue(q, inputMap)
	if err != nil {
		return "", err
	}

	if q.Required {
		if q.Kind == "string" {
			return "mockall::predicate::eq(" + strconv.Quote(value) + ")", nil
		}

		return "mockall::predicate::eq(" + value + "i64)", nil
	}

	if q.Kind == "string" {
		if !named {
			return "mockall::predicate::eq(None::<String>)", nil
		}

		return "mockall::predicate::eq(Some(" + strconv.Quote(value) + ".to_string()))", nil
	}

	if !named {
		return "mockall::predicate::eq(None::<i64>)", nil
	}

	return "mockall::predicate::eq(Some(" + value + "i64))", nil
}

func paramValue(p restrust.Param, inputMap map[string]json.RawMessage) (string, error) {
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

func buildReturning(a arm, c VectorCase, op restrust.Operation, owner restrust.Controller) (string, error) {
	switch a.kind {
	case armKindOK:
		if op.Response == "" {
			return "Ok(())", nil
		}

		compact, err := compactJSON(c.ControllerReply)
		if err != nil {
			return "", fmt.Errorf("reading controllerReply: %w", err)
		}

		decoded := fmt.Sprintf("serde_json::from_str::<%s>(%s).expect(\"decoding controllerReply\")", op.Response, strconv.Quote(compact))

		if op.Stream {
			return "{ let (sender, receiver) = std::sync::mpsc::channel(); sender.send(" + decoded + ").expect(\"sending the event\"); Ok(receiver) }", nil
		}

		return "Ok(" + decoded + ")", nil
	case armKindUnauthenticated, armKindMissingQuery:
		return "", nil
	case armKindAuthentication:
		return fmt.Sprintf("Err(%sControllerError::Authentication { subject: %s.to_string(), reason: \"generated by vectors-rust\".to_string() })", owner.Pascal, strconv.Quote(a.id)), nil
	case armKindAuthorization:
		return fmt.Sprintf("Err(%sControllerError::Authorization { subject: %s.to_string(), reason: \"generated by vectors-rust\".to_string() })", owner.Pascal, strconv.Quote(a.id)), nil
	case armKindNotFound:
		return fmt.Sprintf("Err(%sControllerError::NotFound { id: %s.to_string() })", owner.Pascal, strconv.Quote(a.id)), nil
	case armKindInvalid:
		return fmt.Sprintf("Err(%sControllerError::Invalid { field: %s.to_string(), reason: \"generated by vectors-rust\".to_string() })", owner.Pascal, strconv.Quote(a.id)), nil
	case armKindSemantic:
		return fmt.Sprintf("Err(%sControllerError::Semantic { resource: %s.to_string(), reason: \"generated by vectors-rust\".to_string() })", owner.Pascal, strconv.Quote(a.id)), nil
	case armKindRateLimited:
		return fmt.Sprintf("Err(%sControllerError::RateLimited { subject: %s.to_string(), reason: \"generated by vectors-rust\".to_string() })", owner.Pascal, strconv.Quote(a.id)), nil
	case armKindNotImplemented:
		return fmt.Sprintf("Err(%sControllerError::NotImplemented { operation: %s.to_string() })", owner.Pascal, strconv.Quote(a.id)), nil
	default:
		return "", fmt.Errorf("choosing a mock arm: unhandled kind %v", a.kind)
	}
}

func driverArgs(controllers []controllerView, armedPascal, armedVar string, auth bool) []string {
	args := make([]string, 0, len(controllers)+1)

	for _, c := range controllers {
		if c.Pascal == armedPascal {
			args = append(args, "std::sync::Arc::new("+armedVar+")")

			continue
		}

		args = append(args, "std::sync::Arc::new(Mock"+c.Pascal+"Controller::new())")
	}

	if auth {
		args = append(args, "std::sync::Arc::new(ticket_verifier)")
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
