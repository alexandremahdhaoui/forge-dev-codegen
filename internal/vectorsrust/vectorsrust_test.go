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
        .times(1)
        .returning(|_body| Ok(serde_json::from_str::<Greeting>("{\"id\":\"6ba7b810-9dad-11d1-80b4-00c04fd430c8\",\"name\":\"Songe\",\"count\":0}").expect("decoding controllerReply")));

    let driver = HttpDriver::new(std::sync::Arc::new(greeting_controller));

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
    assert_eq!(got, want);
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

	want := []string{"app/tests/zz_generated_vectors.rs"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("emitted paths\n got %q\nwant %q", got, want)
	}
}

func TestARelativeAppDirPrefixesTheEmittedPath(t *testing.T) {
	files, err := vectorsrust.Generate([]byte(helloSpec), []byte(helloCases), vectorsrust.Options{
		Service: "songe-hello",
		AppDir:  "../songe-hello-app",
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	if files[0].Path != "../songe-hello-app/tests/zz_generated_vectors.rs" {
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
		"use songe_hello_core::controller::greeting_controller::GreetingControllerError;",
		"mockall::mock! {",
		"    pub GreetingController {}",
		"    impl songe_hello_core::controller::greeting_controller::GreetingController for GreetingController {",
		"        fn create_greeting(&self, body: CreateGreetingRequest) -> Result<Greeting, GreetingControllerError>;",
		"        fn get_greeting(&self, id: &str) -> Result<Greeting, GreetingControllerError>;",
	}

	for _, line := range want {
		if !strings.Contains(content, line) {
			t.Errorf("the file lacks %q\n%s", line, content)
		}
	}

	if strings.Contains(content, "use songe_hello_core::controller::greeting_controller::GreetingController;") {
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

func TestAVectorNamingAnOperationTheSpecDoesNotDeclareIsRefused(t *testing.T) {
	badCases := `{"cases": [{"case": "bogus", "operation": "deleteGreeting", "controllerReply": {"id": "g1"}, "expectedStatus": 200}]}`

	_, err := vectorsrust.Generate([]byte(helloSpec), []byte(badCases), vectorsrust.Options{Service: "songe-hello"})
	if err == nil || !strings.Contains(err.Error(), `names operation "deleteGreeting", which the spec does not declare`) {
		t.Fatalf("want a refusal naming the undeclared operation, got %v", err)
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
