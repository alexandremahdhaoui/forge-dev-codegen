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
	"reflect"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/vectorsrust"
)

const helloSpec = `
openapi: 3.1.0
info:
  title: Hello API
  version: 1.0.0
paths:
  /greetings:
    post:
      operationId: createGreeting
      summary: Create a greeting
      x-controller: greeting
      x-ports: [GreetingStore]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/CreateGreetingRequest"
      responses:
        "201":
          description: The created greeting
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Greeting"
        "422":
          description: Error payload
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Error"
  /greetings/{id}:
    get:
      operationId: getGreeting
      summary: Get a greeting by id
      x-controller: greeting
      x-ports: [GreetingStore]
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        "200":
          description: The greeting
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Greeting"
        "404":
          description: Error payload
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Error"
components:
  schemas:
    CreateGreetingRequest:
      type: object
      required: [name]
      properties:
        name:
          type: string
    Greeting:
      type: object
      x-store: true
      required: [id, name, count]
      properties:
        id:
          type: string
        name:
          type: string
        count:
          type: integer
    Error:
      type: object
      required: [type, message]
      properties:
        type:
          type: string
        message:
          type: string
`

const helloCases = `{
  "cases": [
    {
      "case": "create_valid_name",
      "operation": "createGreeting",
      "input": { "name": "Songe" },
      "controllerReply": {
        "id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
        "name": "Songe",
        "count": 0
      },
      "expectedStatus": 201,
      "expectedBody": {
        "id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
        "name": "Songe",
        "count": 0
      }
    },
    {
      "case": "create_empty_name_refused",
      "operation": "createGreeting",
      "input": { "name": "" },
      "expectedStatus": 422,
      "expectedErrorSubstring": "name"
    },
    {
      "case": "get_existing_id",
      "operation": "getGreeting",
      "input": { "id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8" },
      "controllerReply": {
        "id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
        "name": "Songe",
        "count": 1
      },
      "expectedStatus": 200,
      "expectedBody": {
        "id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
        "name": "Songe",
        "count": 1
      }
    },
    {
      "case": "get_unknown_id",
      "operation": "getGreeting",
      "input": { "id": "00000000-0000-0000-0000-000000000000" },
      "expectedStatus": 404,
      "expectedErrorSubstring": "not found"
    }
  ]
}`

const createValidNameBody = `#[tokio::test]
async fn create_valid_name() {
    let mut greeting_controller = MockGreetingController::new();
    greeting_controller
        .expect_create_greeting()
        .with(mockall::predicate::eq(serde_json::from_str::<CreateGreetingRequest>("{\"name\":\"Songe\"}").expect("decoding input")))
        .times(1)
        .returning(|_body| Ok(serde_json::from_str::<Greeting>("{\"id\":\"6ba7b810-9dad-11d1-80b4-00c04fd430c8\",\"name\":\"Songe\",\"count\":0}").expect("decoding controllerReply")));

    let driver = HttpDriver::new(HttpDriverConfig::default(), std::sync::Arc::new(greeting_controller));

    let request = Request::builder()
        .method("POST")
        .uri("/greetings")
        .header("content-type", "application/json")
        .body(Body::from("{\"name\":\"Songe\"}"))
        .unwrap();

    let response = driver.router().oneshot(request).await.unwrap();

    assert_eq!(response.status().as_u16(), 201);

    let body_bytes = response.into_body().collect().await.unwrap().to_bytes();
    let got: serde_json::Value = serde_json::from_slice(&body_bytes).expect("a JSON body");
    let want: serde_json::Value = serde_json::from_str("{\"id\":\"6ba7b810-9dad-11d1-80b4-00c04fd430c8\",\"name\":\"Songe\",\"count\":0}").expect("expectedBody is valid JSON");
    assert!(body_matches(&want, &got), "body {got} does not match {want}");
}`

func TestTheEmittedFileHasOneEntryUnderAppTests(t *testing.T) {
	files, err := vectorsrust.Generate([]byte(helloSpec), []byte(helloCases), vectorsrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	got := []string{}
	for _, f := range files {
		got = append(got, f.Path)
	}

	want := []string{"tests/zz_generated_vectors.rs"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("emitted paths\n got %q\nwant %q", got, want)
	}
}

func TestARelativeCrateDirPrefixesTheEmittedPath(t *testing.T) {
	files, err := vectorsrust.Generate([]byte(helloSpec), []byte(helloCases), vectorsrust.Options{
		Service:  "songe-hello",
		CrateDir: "../songe-hello",
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	if files[0].Path != "../songe-hello/tests/zz_generated_vectors.rs" {
		t.Fatalf("got path %q", files[0].Path)
	}
}

func TestTheEmittedFileCarriesTheGeneratedHeader(t *testing.T) {
	files, err := vectorsrust.Generate([]byte(helloSpec), []byte(helloCases), vectorsrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	const wantHeader = "// Code generated by vectors-rust (forge-dev-codegen). DO NOT EDIT."

	if !strings.HasPrefix(files[0].Content, wantHeader) {
		t.Fatalf("the file lacks the generated header:\n%s", files[0].Content)
	}
}

func TestTheGeneratedTestBodyForCreateValidNameMatchesExactly(t *testing.T) {
	files, err := vectorsrust.Generate([]byte(helloSpec), []byte(helloCases), vectorsrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	if !strings.Contains(files[0].Content, createValidNameBody) {
		t.Fatalf("the file lacks the exact create_valid_name test body\ngot:\n%s", files[0].Content)
	}
}

func TestTheGeneratedFileMocksTheControllerTraitByFullPathToAvoidNameCollision(t *testing.T) {
	files, err := vectorsrust.Generate([]byte(helloSpec), []byte(helloCases), vectorsrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	content := files[0].Content

	want := []string{
		"use songe_hello::controller::GreetingControllerError;",
		"mockall::mock! {",
		"    pub GreetingController {}",
		"    impl songe_hello::controller::GreetingController for GreetingController {",
		"        fn create_greeting(&self, body: CreateGreetingRequest) -> Result<Greeting, GreetingControllerError>;",
		"        fn get_greeting(&self, id: &str) -> Result<Greeting, GreetingControllerError>;",
	}

	for _, line := range want {
		if !strings.Contains(content, line) {
			t.Errorf("the file lacks %q\n%s", line, content)
		}
	}

	if strings.Contains(content, "use songe_hello::controller::GreetingController;") {
		t.Error("importing the trait under its own name collides with the mock struct sharing that name")
	}
}

func TestAnErrorCaseChoosesTheControllerErrorVariantThatMatchesTheDeclaredStatus(t *testing.T) {
	files, err := vectorsrust.Generate([]byte(helloSpec), []byte(helloCases), vectorsrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	content := files[0].Content

	tests := []struct {
		name string
		want string
	}{
		{
			name: "a 422 with an error substring arms an Invalid reply",
			want: `Err(GreetingControllerError::Invalid { field: "name".to_string(), reason: "generated by vectors-rust".to_string() })`,
		},
		{
			name: "a 404 arms a NotFound reply",
			want: `Err(GreetingControllerError::NotFound { id: "not found".to_string() })`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(content, tt.want) {
				t.Errorf("the file lacks %q\n%s", tt.want, content)
			}
		})
	}
}

func TestAVectorWhoseOperationBelongsToAnotherTransportIsSkippedAndTheRestStillEmit(t *testing.T) {
	mixedCases := `{"cases": [
		{"case": "get_existing_id", "operation": "getGreeting", "input": {"id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8"}, "controllerReply": {"id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8", "name": "Songe", "count": 0}, "expectedStatus": 200},
		{"case": "udp_echo_returns_the_payload", "operation": "udp_echo", "input": {"payload": "songe"}, "expectedStatus": 200, "expectedBody": {"payload": "songe"}}
	]}`

	files, err := vectorsrust.Generate([]byte(helloSpec), []byte(mixedCases), vectorsrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	content := files[0].Content

	if !strings.Contains(content, "async fn get_existing_id()") {
		t.Fatalf("the OpenAPI vector lost its test\n%s", content)
	}

	if strings.Contains(content, "udp_echo_returns_the_payload") {
		t.Fatalf("the vector of another transport reached the emitted file\n%s", content)
	}
}

func TestTheServiceNameIsRequired(t *testing.T) {
	_, err := vectorsrust.Generate([]byte(helloSpec), []byte(helloCases), vectorsrust.Options{})
	if err == nil || !strings.Contains(err.Error(), "the service name is required") {
		t.Fatalf("want a refusal naming the missing service, got %v", err)
	}
}

func TestAServiceNameThatCannotBeACrateNameIsRefused(t *testing.T) {
	_, err := vectorsrust.Generate([]byte(helloSpec), []byte(helloCases), vectorsrust.Options{Service: `svc"; drop table greeting; --`})
	if err == nil || !strings.Contains(err.Error(), "is not one Rust or Cargo can spell") {
		t.Fatalf("want a refusal naming the unspellable service, got %v", err)
	}
}

func TestAControllerReplyWithANon2xxExpectedStatusIsRefused(t *testing.T) {
	badCases := `{"cases": [{"case": "bogus", "operation": "createGreeting", "input": {"name": "Songe"}, "controllerReply": {"id": "g1", "name": "Songe", "count": 0}, "expectedStatus": 422}]}`

	_, err := vectorsrust.Generate([]byte(helloSpec), []byte(badCases), vectorsrust.Options{Service: "songe-hello"})
	if err == nil || !strings.Contains(err.Error(), `controllerReply is present but expectedStatus is 422, a success case needs a 2xx status`) {
		t.Fatalf("want a refusal naming the case and the mismatched status, got %v", err)
	}
}

func TestA2xxExpectedStatusWithoutAControllerReplyIsRefused(t *testing.T) {
	badCases := `{"cases": [{"case": "bogus", "operation": "createGreeting", "input": {"name": ""}, "expectedStatus": 201, "expectedErrorSubstring": "name"}]}`

	_, err := vectorsrust.Generate([]byte(helloSpec), []byte(badCases), vectorsrust.Options{Service: "songe-hello"})
	if err == nil || !strings.Contains(err.Error(), `expectedStatus is 201 but no controllerReply is present, a 2xx status needs a success case`) {
		t.Fatalf("want a refusal naming the case and the mismatched status, got %v", err)
	}
}

func TestAnErrorStatusThatMatchesNoKnownControllerErrorIsRefused(t *testing.T) {
	badCases := `{"cases": [{"case": "bogus", "operation": "createGreeting", "input": {"name": "x"}, "expectedStatus": 500, "expectedErrorSubstring": "boom"}]}`

	_, err := vectorsrust.Generate([]byte(helloSpec), []byte(badCases), vectorsrust.Options{Service: "songe-hello"})
	if err == nil || !strings.Contains(err.Error(), `expectedStatus 500 matches none of NotFound (404), Invalid (422) or NotImplemented (501)`) {
		t.Fatalf("want a refusal naming the case and the unmatched status, got %v", err)
	}
}

func TestTheMockAssertsTheDecodedBodyAndPathParameterAgainstTheVectorInput(t *testing.T) {
	files, err := vectorsrust.Generate([]byte(helloSpec), []byte(helloCases), vectorsrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	content := files[0].Content

	want := []string{
		`.with(mockall::predicate::eq(serde_json::from_str::<CreateGreetingRequest>("{\"name\":\"Songe\"}").expect("decoding input")))`,
		`.with(mockall::predicate::eq("6ba7b810-9dad-11d1-80b4-00c04fd430c8"))`,
	}

	for _, line := range want {
		if !strings.Contains(content, line) {
			t.Errorf("the file lacks %q\n%s", line, content)
		}
	}
}

const errorOnlyCases = `{
  "cases": [
    {
      "case": "create_empty_name_refused",
      "operation": "createGreeting",
      "input": { "name": "" },
      "expectedStatus": 422,
      "expectedErrorSubstring": "name"
    }
  ]
}`

func TestTheGeneratedBodyAssertionIsASubsetMatchSoAResponseKeyTheVectorOmitsIsIgnored(t *testing.T) {
	files, err := vectorsrust.Generate([]byte(helloSpec), []byte(helloCases), vectorsrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	content := files[0].Content

	if !strings.Contains(content, `assert!(body_matches(&want, &got), "body {got} does not match {want}");`) {
		t.Fatalf("the generated body assertion is not a subset match:\n%s", content)
	}

	if strings.Contains(content, "assert_eq!(got, want);") {
		t.Fatalf("the generated file still asserts a whole body equality:\n%s", content)
	}
}

func TestTheGeneratedHelperReadsAUuidPlaceholderOnATopLevelIdAsAnyUuidV4(t *testing.T) {
	files, err := vectorsrust.Generate([]byte(helloSpec), []byte(helloCases), vectorsrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	content := files[0].Content

	want := []string{
		`const UUID_PLACEHOLDER: &str = "<uuid>";`,
		`const ID_KEY: &str = "id";`,
		"fn body_matches(expected: &serde_json::Value, actual: &serde_json::Value) -> bool {",
		"if key == ID_KEY && expected.as_str() == Some(UUID_PLACEHOLDER) {",
		"return actual.as_str().is_some_and(is_uuid_v4);",
		"fn is_uuid_v4(value: &str) -> bool {",
	}

	for _, line := range want {
		if !strings.Contains(content, line) {
			t.Fatalf("the generated file lacks %q:\n%s", line, content)
		}
	}
}

func TestVectorsThatExpectNoBodyEmitNoBodyMatcher(t *testing.T) {
	files, err := vectorsrust.Generate([]byte(helloSpec), []byte(errorOnlyCases), vectorsrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	if strings.Contains(files[0].Content, "fn body_matches(") {
		t.Fatalf("the generated file carries a body matcher no test calls:\n%s", files[0].Content)
	}
}

func TestParsingVectorsRejectsAMalformedDocument(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want string
	}{
		{
			name: "no cases at all",
			doc:  `{"cases": []}`,
			want: "it declares no cases",
		},
		{
			name: "a case without a name",
			doc:  `{"cases": [{"operation": "createGreeting", "expectedStatus": 200, "expectedErrorSubstring": "x"}]}`,
			want: "case is required",
		},
		{
			name: "a case name that is not a Rust test function identifier",
			doc:  `{"cases": [{"case": "1-bad name", "operation": "createGreeting", "expectedStatus": 200, "expectedErrorSubstring": "x"}]}`,
			want: "is not one Rust can spell for a test function",
		},
		{
			name: "two cases sharing a name",
			doc:  `{"cases": [{"case": "dup", "operation": "createGreeting", "expectedStatus": 200, "expectedErrorSubstring": "x"}, {"case": "dup", "operation": "getGreeting", "expectedStatus": 200, "expectedErrorSubstring": "x"}]}`,
			want: "two cases share this name",
		},
		{
			name: "a case without an operation",
			doc:  `{"cases": [{"case": "ok", "expectedStatus": 200, "expectedErrorSubstring": "x"}]}`,
			want: "operation is required",
		},
		{
			name: "a case without an expectedStatus",
			doc:  `{"cases": [{"case": "ok", "operation": "createGreeting", "expectedErrorSubstring": "x"}]}`,
			want: "expectedStatus is required",
		},
		{
			name: "a case with neither controllerReply nor expectedErrorSubstring",
			doc:  `{"cases": [{"case": "ok", "operation": "createGreeting", "expectedStatus": 200}]}`,
			want: "an error case needs expectedErrorSubstring, and a success case needs controllerReply",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := vectorsrust.Generate([]byte(helloSpec), []byte(tt.doc), vectorsrust.Options{Service: "songe-hello"})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want an error containing %q, got %v", tt.want, err)
			}
		})
	}
}

const datagramProto = `syntax = "proto3";

package songe.hello.udp.v1;

service HelloDatagram {
  rpc Echo(Echo) returns (Echo);
}

message Echo {
  string payload = 1;
  uint64 count = 2;
}
`

const datagramCases = `{
  "cases": [
    {
      "case": "a_datagram_echo_comes_back_with_the_count_raised_by_one",
      "operation": "udp_echo",
      "input": { "sessionId": "0123456789abcdef", "payload": "songe", "count": 7 },
      "controllerReply": { "payload": "songe", "count": 8 },
      "expectedBody": { "sessionId": "0123456789abcdef", "payload": "songe", "count": 8 }
    }
  ]
}`

func generateWithDatagrams(t *testing.T, cases string) string {
	t.Helper()

	files, err := vectorsrust.Generate([]byte(helloSpec), []byte(cases), vectorsrust.Options{
		Service: "songe-hello",
		Proto:   []byte(datagramProto),
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("the engine emitted %d files, want 1", len(files))
	}

	return files[0].Content
}

func TestADatagramVectorDrivesTheGeneratedUdpDriverOverAMockedController(t *testing.T) {
	content := generateWithDatagrams(t, datagramCases)

	for _, want := range []string{
		"async fn a_datagram_echo_comes_back_with_the_count_raised_by_one() {",
		"let expected_request = Echo { payload: \"songe\".to_string(), count: 7 };",
		"let controller_reply = Echo { payload: \"songe\".to_string(), count: 8 };",
		"let mut hello_datagram_controller = MockHelloDatagramController::new();",
		"let mut driver = HelloDatagramUdpDriver::new(",
		"        std::sync::Arc::new(hello_datagram_controller),",
		"driver.bind().await.expect(\"a bound udp socket\");",
		"eprintln!(\"serving HelloDatagramUdpDriver: {}\", error_chain(&error));",
		"let client = HelloDatagramUdpClient::new(HelloDatagramUdpClientConfig {",
		"        session_id: \"0123456789abcdef\".to_string(),",
		"assert_eq!(reply, Echo { payload: \"songe\".to_string(), count: 8 });",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("the emitted file never carried %q:\n%s", want, content)
		}
	}
}

func TestADatagramVectorMocksTheDatagramControllerTraitByFullPath(t *testing.T) {
	content := generateWithDatagrams(t, datagramCases)

	for _, want := range []string{
		"impl songe_hello::udp::controller::HelloDatagramController for HelloDatagramController {",
		"fn echo(&self, request: Echo, context: &Context) -> Result<Echo, HelloDatagramControllerError>;",
		"use songe_hello::udp::types::context::Context;",
		"use songe_hello::udp::types::hello_datagram_messages::{Echo};",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("the emitted file never carried %q:\n%s", want, content)
		}
	}
}

func TestADatagramVectorWithNoSessionIdIsRefused(t *testing.T) {
	const missing = `{
  "cases": [
    {
      "case": "no_session",
      "operation": "udp_echo",
      "input": { "payload": "songe", "count": 7 },
      "controllerReply": { "payload": "songe", "count": 8 },
      "expectedBody": { "payload": "songe", "count": 8 }
    }
  ]
}`

	_, err := vectorsrust.Generate([]byte(helloSpec), []byte(missing), vectorsrust.Options{
		Service: "songe-hello",
		Proto:   []byte(datagramProto),
	})
	if err == nil || !strings.Contains(err.Error(), "input needs a sessionId") {
		t.Fatalf("want a refusal naming sessionId, got %v", err)
	}
}

func TestADatagramVectorWithASessionIdThatIsNotSixteenBytesIsRefused(t *testing.T) {
	const short = `{
  "cases": [
    {
      "case": "short_session",
      "operation": "udp_echo",
      "input": { "sessionId": "short", "payload": "songe", "count": 7 },
      "controllerReply": { "payload": "songe", "count": 8 },
      "expectedBody": { "payload": "songe", "count": 8 }
    }
  ]
}`

	_, err := vectorsrust.Generate([]byte(helloSpec), []byte(short), vectorsrust.Options{
		Service: "songe-hello",
		Proto:   []byte(datagramProto),
	})
	if err == nil || !strings.Contains(err.Error(), "sessionId must be 16 bytes, got 5") {
		t.Fatalf("want a refusal naming the session id length, got %v", err)
	}
}

func TestADatagramVectorWithoutAControllerReplyIsRefused(t *testing.T) {
	const noReply = `{
  "cases": [
    {
      "case": "no_reply",
      "operation": "udp_echo",
      "input": { "sessionId": "0123456789abcdef", "payload": "songe", "count": 7 },
      "expectedBody": { "payload": "songe", "count": 8 }
    }
  ]
}`

	_, err := vectorsrust.Generate([]byte(helloSpec), []byte(noReply), vectorsrust.Options{
		Service: "songe-hello",
		Proto:   []byte(datagramProto),
	})
	if err == nil || !strings.Contains(err.Error(), "a datagram case needs controllerReply") {
		t.Fatalf("want a refusal naming controllerReply, got %v", err)
	}
}

func TestWithoutAProtoADatagramVectorIsSkippedLikeAnyOtherTransport(t *testing.T) {
	files, err := vectorsrust.Generate([]byte(helloSpec), []byte(datagramCases+""), vectorsrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	if strings.Contains(files[0].Content, "UdpDriver") {
		t.Fatalf("a datagram test was emitted with no proto:\n%s", files[0].Content)
	}
}
