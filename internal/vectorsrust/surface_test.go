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

package vectorsrust_test

import (
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/vectorsrust"
)

const nestedProto = `syntax = "proto3";

package songe.play.udp.v1;

service PlayDatagram {
  rpc Act(Act) returns (WorldState);
}

message Act {
  string kind = 1;
}

message Position {
  uint64 x = 1;
  uint64 y = 2;
}

message Character {
  string character_id = 1;
  Position at = 2;
  uint64 hp = 3;
}

message WorldState {
  Character player = 1;
  Character boar = 2;
  uint64 turn = 3;
}
`

const nestedCases = `{
  "cases": [
    {
      "case": "an_act_answers_the_world_state_with_both_characters",
      "operation": "udp_act",
      "input": { "sessionId": "0123456789abcdef", "kind": "attack" },
      "controllerReply": {
        "player": { "character_id": "c1", "at": { "x": 1, "y": 2 }, "hp": 10 },
        "boar": { "character_id": "b1", "at": { "x": 4, "y": 4 }, "hp": 3 },
        "turn": 2
      },
      "expectedBody": {
        "sessionId": "0123456789abcdef",
        "player": { "character_id": "c1", "at": { "x": 1, "y": 2 }, "hp": 10 },
        "boar": { "character_id": "b1", "at": { "x": 4, "y": 4 }, "hp": 3 },
        "turn": 2
      }
    }
  ]
}`

func generateNested(t *testing.T, proto, cases string) (string, error) {
	t.Helper()

	files, err := vectorsrust.Generate(nil, []byte(cases), vectorsrust.Options{
		Service: "songe-play",
		Proto:   []byte(proto),
	})
	if err != nil {
		return "", err
	}

	return files[0].Content, nil
}

func TestACellWithoutAnOpenapiDocumentEmitsTheDatagramVectorsAndNoAxumImport(t *testing.T) {
	content, err := generateNested(t, nestedProto, nestedCases)
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	for _, unwanted := range []string{"use axum::body::{to_bytes, Body};", "http_driver", "HttpDriverConfig", "tower::ServiceExt"} {
		if strings.Contains(content, unwanted) {
			t.Errorf("a service with no OpenAPI document still carries %q\n%s", unwanted, content)
		}
	}

	if !strings.Contains(content, "async fn an_act_answers_the_world_state_with_both_characters() {") {
		t.Fatalf("the datagram vector never reached the file\n%s", content)
	}
}

func TestACellWithoutAnOpenapiDocumentAndNoProtoIsRefusedByName(t *testing.T) {
	_, err := vectorsrust.Generate(nil, []byte(nestedCases), vectorsrust.Options{Service: "songe-play"})

	want := "the cell declares no surface to drive"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("generating reported %v, want %q", err, want)
	}
}

func TestACellWithOnlyAGrpcProtoEmitsTheCallVectors(t *testing.T) {
	cases := `{"cases": [{"case": "grpc_ping_answers", "operation": "grpc_Ping", "input": {"message": "songe", "count": 1}, "controllerReply": {"message": "songe", "count": 2}}]}`

	files, err := vectorsrust.Generate(nil, []byte(cases), vectorsrust.Options{
		Service:   "songe-hello",
		GrpcProto: []byte(callProto),
		GrpcCell:  "grpc",
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	if strings.Contains(files[0].Content, "use axum::body::{to_bytes, Body};") {
		t.Fatalf("a grpc only service still carries the axum imports\n%s", files[0].Content)
	}

	if !strings.Contains(files[0].Content, "async fn grpc_ping_answers() {") {
		t.Fatalf("the grpc vector never reached the file\n%s", files[0].Content)
	}
}

func TestANestedMessageIsReadAsSomeAndAnAbsentOneAsNone(t *testing.T) {
	content, err := generateNested(t, nestedProto, nestedCases)
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	want := `WorldState { player: Some(Character { character_id: "c1".to_string(), at: Some(Position { x: 1, y: 2 }), hp: 10 }), boar: Some(Character { character_id: "b1".to_string(), at: Some(Position { x: 4, y: 4 }), hp: 3 }), turn: 2 }`
	if !strings.Contains(content, want) {
		t.Fatalf("the nested literal is missing\n%s\nwant %s", content, want)
	}

	absent := `{"cases": [{"case": "an_act_answers_an_empty_world", "operation": "udp_act", "input": {"sessionId": "0123456789abcdef", "kind": "wait"}, "controllerReply": {"turn": 1}, "expectedBody": {"turn": 1}}]}`

	content, err = generateNested(t, nestedProto, absent)
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	if want := "WorldState { player: None, boar: None, turn: 1 }"; !strings.Contains(content, want) {
		t.Fatalf("an absent nested message is not None\n%s\nwant %s", content, want)
	}
}

func TestANestedFieldTheProtoDoesNotDeclareIsRefusedByName(t *testing.T) {
	cases := `{"cases": [{"case": "a", "operation": "udp_act", "input": {"sessionId": "0123456789abcdef", "kind": "wait"}, "controllerReply": {"player": {"characterId": "c1"}}, "expectedBody": {"turn": 0}}]}`

	_, err := generateNested(t, nestedProto, cases)

	want := `message "Character" declares no field named characterId, it declares character_id, at, hp`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("generating reported %v, want %q", err, want)
	}
}

func TestAFieldSpelledInCamelCaseIsRefusedInsteadOfSilentlyLeftAtItsDefault(t *testing.T) {
	cases := `{"cases": [{"case": "a", "operation": "udp_act", "input": {"sessionId": "0123456789abcdef", "kind": "wait"}, "controllerReply": {"player": {"character_id": "c1", "at": {"x": 1, "y": 2}, "hp": 1}, "boar": {"character_id": "b1", "at": {"x": 1, "y": 1}, "hp": 1}, "turnOwner": 2}, "expectedBody": {"turn": 0}}]}`

	_, err := generateNested(t, nestedProto, cases)

	want := `message "WorldState" declares no field named turnOwner, it declares player, boar, turn, a case spells a field the way the proto spells it`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("generating reported %v, want %q", err, want)
	}
}

func TestTheSessionIdOfTheFrameIsTheOneKeyNoMessageDeclares(t *testing.T) {
	if _, err := generateNested(t, nestedProto, nestedCases); err != nil {
		t.Fatalf("sessionId names the frame session and was refused: %v", err)
	}
}

func TestAMessageThatReachesItselfIsRefusedBeforeAnyVectorReadsIt(t *testing.T) {
	const cyclic = `syntax = "proto3";

package songe.play.udp.v1;

service PlayDatagram {
  rpc Act(Act) returns (Node);
}

message Act {
  string kind = 1;
}

message Node {
  string name = 1;
  Node next = 2;
}
`

	cases := `{"cases": [{"case": "a", "operation": "udp_act", "input": {"sessionId": "0123456789abcdef", "kind": "wait"}, "controllerReply": {"name": "a", "next": {"name": "b", "next": {"name": "c"}}}, "expectedBody": {"name": "a"}}]}`

	_, err := generateNested(t, cyclic, cases)

	want := "message cycle Node -> Node, a message cannot reach itself through message fields"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("generating reported %v, want %q", err, want)
	}
}

func TestA401OnAnXAuthOperationWithoutANamedErrorIsRefusedAsAmbiguous(t *testing.T) {
	cases := `{"cases": [{"case": "a", "operation": "countGreeting", "input": {"id": "g1"}, "expectedStatus": 401, "expectedErrorSubstring": "bearer"}]}`

	_, err := vectorsrust.Generate([]byte(guardedSpec), []byte(cases), vectorsrust.Options{Service: "songe-hello"})

	want := `name one in expectedError: Unauthenticated or Authentication`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("generating reported %v, want %q", err, want)
	}
}

func TestNamingInvalidReadsTheStatusTheOperationDeclaresAndNotAFixedOne(t *testing.T) {
	cases := `{"cases": [{"case": "a", "operation": "createGreeting", "input": {"name": ""}, "expectedStatus": 422, "expectedError": "Invalid", "expectedErrorSubstring": "name"}]}`

	files, err := vectorsrust.Generate([]byte(helloSpec), []byte(cases), vectorsrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	want := `Err(GreetingControllerError::Invalid { field: "name".to_string(), reason: "generated by vectors-rust".to_string() })`
	if !strings.Contains(files[0].Content, want) {
		t.Fatalf("the emitted test lacks %q\n%s", want, files[0].Content)
	}

	wrong := `{"cases": [{"case": "a", "operation": "createGreeting", "input": {"name": ""}, "expectedStatus": 400, "expectedError": "Invalid", "expectedErrorSubstring": "name"}]}`

	_, err = vectorsrust.Generate([]byte(helloSpec), []byte(wrong), vectorsrust.Options{Service: "songe-hello"})

	if err == nil || !strings.Contains(err.Error(), `expectedError Invalid answers 422 on "createGreeting" and expectedStatus is 400`) {
		t.Fatalf("generating reported %v, want the operation's invalid status", err)
	}
}

func TestAKeyOnAMessageThatDeclaresNoFieldIsRefusedByName(t *testing.T) {
	const emptyReply = `syntax = "proto3";

package songe.play.udp.v1;

service PlayDatagram {
  rpc Act(Act) returns (Nothing);
}

message Act {
  string kind = 1;
}

message Nothing {}
`

	cases := `{"cases": [{"case": "a", "operation": "udp_act", "input": {"sessionId": "0123456789abcdef", "kind": "wait"}, "controllerReply": {"turn": 1}, "expectedBody": {}}]}`

	_, err := generateNested(t, emptyReply, cases)

	want := `message "Nothing" declares no field and the case names turn`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("generating reported %v, want %q", err, want)
	}
}

func TestANestedFieldThatIsNotAJsonObjectIsRefused(t *testing.T) {
	cases := `{"cases": [{"case": "a", "operation": "udp_act", "input": {"sessionId": "0123456789abcdef", "kind": "wait"}, "controllerReply": {"player": 7}, "expectedBody": {"turn": 0}}]}`

	_, err := generateNested(t, nestedProto, cases)

	want := `reading field "player": reading input`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("generating reported %v, want %q", err, want)
	}
}

func TestANamedErrorThatDisagreesWithTheStatusIsRefused(t *testing.T) {
	cases := `{"cases": [{"case": "a", "operation": "getGreeting", "input": {"id": "g1"}, "expectedStatus": 404, "expectedError": "Semantic", "expectedErrorSubstring": "g1"}]}`

	_, err := vectorsrust.Generate([]byte(guardedSpec), []byte(cases), vectorsrust.Options{Service: "songe-hello"})

	want := `expectedError Semantic answers 409 on "getGreeting" and expectedStatus is 404`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("generating reported %v, want %q", err, want)
	}
}

func TestANamedErrorNoTaxonomyMemberCarriesIsRefusedWithTheList(t *testing.T) {
	cases := `{"cases": [{"case": "a", "operation": "getGreeting", "input": {"id": "g1"}, "expectedStatus": 404, "expectedError": "Bogus", "expectedErrorSubstring": "g1"}]}`

	_, err := vectorsrust.Generate([]byte(guardedSpec), []byte(cases), vectorsrust.Options{Service: "songe-hello"})

	want := `expectedError "Bogus" names no member a controller answers, the members are Authentication, Authorization, Invalid, NotFound, NotImplemented, RateLimited, Semantic and Unauthenticated`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("generating reported %v, want %q", err, want)
	}
}

func TestNamingUnauthenticatedOnAnOperationWithoutXAuthIsRefused(t *testing.T) {
	cases := `{"cases": [{"case": "a", "operation": "getGreeting", "input": {"id": "g1"}, "expectedStatus": 401, "expectedError": "Unauthenticated", "expectedErrorSubstring": "bearer"}]}`

	_, err := vectorsrust.Generate([]byte(guardedSpec), []byte(cases), vectorsrust.Options{Service: "songe-hello"})

	want := `expectedError Unauthenticated means the ticket verifier refuses the bearer, and "getGreeting" carries no x-auth`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("generating reported %v, want %q", err, want)
	}
}

func TestNamingAuthenticationOnAnXAuthOperationArmsTheControllerAndNotTheVerifier(t *testing.T) {
	cases := `{"cases": [{"case": "a", "operation": "countGreeting", "input": {"id": "g1"}, "bearer": "open", "subject": "expired", "expectedStatus": 401, "expectedError": "Authentication", "expectedErrorSubstring": "expired"}]}`

	files, err := vectorsrust.Generate([]byte(guardedSpec), []byte(cases), vectorsrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	want := `Err(GreetingControllerError::Authentication { subject: "expired".to_string(), reason: "generated by vectors-rust".to_string() })`
	if !strings.Contains(files[0].Content, want) {
		t.Fatalf("the emitted test lacks %q\n%s", want, files[0].Content)
	}

	if strings.Contains(files[0].Content, "greeting_controller.expect_count_greeting().never();") {
		t.Fatal("naming Authentication arms the controller, the verifier answers the subject")
	}
}

func TestANamedErrorOnASuccessCaseIsRefused(t *testing.T) {
	cases := `{"cases": [{"case": "a", "operation": "getGreeting", "input": {"id": "g1"}, "controllerReply": {"id": "g1", "name": "n", "count": 0}, "expectedStatus": 200, "expectedError": "NotFound"}]}`

	_, err := vectorsrust.Generate([]byte(guardedSpec), []byte(cases), vectorsrust.Options{Service: "songe-hello"})

	want := `it carries controllerReply and expectedError "NotFound", a case answers a reply or an error, never both`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("generating reported %v, want %q", err, want)
	}
}

func TestEveryTaxonomyMemberCanBeNamedOnAGrpcCaseAndAssertsItsStatusCode(t *testing.T) {
	tests := []struct {
		member string
		code   string
	}{
		{member: "Authentication", code: "Unauthenticated"},
		{member: "Authorization", code: "PermissionDenied"},
		{member: "NotFound", code: "NotFound"},
		{member: "Invalid", code: "InvalidArgument"},
		{member: "Semantic", code: "FailedPrecondition"},
		{member: "RateLimited", code: "ResourceExhausted"},
		{member: "NotImplemented", code: "Unimplemented"},
	}

	for _, tc := range tests {
		t.Run(tc.member, func(t *testing.T) {
			cases := `{"cases": [{"case": "a", "operation": "grpc_Ping", "input": {"message": "songe", "count": 0}, "expectedError": "` + tc.member + `"}]}`

			files, err := vectorsrust.Generate([]byte(helloSpec), []byte(cases), callOptions())
			if err != nil {
				t.Fatalf("generating: %v", err)
			}

			if want := "tonic::Code::" + tc.code + ","; !strings.Contains(files[0].Content, want) {
				t.Fatalf("the emitted test lacks %q\n%s", want, files[0].Content)
			}

			if want := "HelloControllerError::" + tc.member + " {"; !strings.Contains(files[0].Content, want) {
				t.Fatalf("the emitted test lacks %q\n%s", want, files[0].Content)
			}
		})
	}
}

func TestAGrpcCaseWithoutARefusalCarriesNoStatusHelper(t *testing.T) {
	cases := `{"cases": [{"case": "a", "operation": "grpc_Ping", "input": {"message": "songe", "count": 1}, "controllerReply": {"message": "songe", "count": 2}}]}`

	files, err := vectorsrust.Generate([]byte(helloSpec), []byte(cases), callOptions())
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	if strings.Contains(files[0].Content, "fn grpc_code(") {
		t.Fatalf("a file with no refused call carries the status helper\n%s", files[0].Content)
	}
}
