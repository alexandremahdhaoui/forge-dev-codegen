package vectorsrust_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/taxonomy"
	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/vectorsrust"
)

const queriedSpec = `
openapi: 3.1.0
info:
  title: Hello API
  version: 1.0.0
paths:
  /greetings:
    get:
      operationId: listGreetings
      x-controller: greeting
      x-ports: [GreetingStore]
      parameters:
        - name: name
          in: query
          required: true
          schema:
            type: string
        - name: after
          in: query
          schema:
            type: string
        - name: limit
          in: query
          schema:
            type: integer
      responses:
        "200":
          description: The page
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/GreetingPage"
        "422":
          description: Refused
components:
  schemas:
    Greeting:
      type: object
      x-store:
        key: id
        lookups:
          - { by: [name], answers: page }
        adapters: [sqlite, memory]
      required: [id, name, count]
      properties:
        id:
          type: string
        name:
          type: string
        count:
          type: integer
    GreetingPage:
      type: object
      required: [greetings]
      properties:
        greetings:
          type: array
          items:
            $ref: "#/components/schemas/Greeting"
`

func generatedVectors(t *testing.T, spec, vectors string) string {
	t.Helper()

	files, err := vectorsrust.Generate([]byte(spec), []byte(vectors), vectorsrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	return files[0].Content
}

func refusedVectors(t *testing.T, spec, vectors, want string) {
	t.Helper()

	_, err := vectorsrust.Generate([]byte(spec), []byte(vectors), vectorsrust.Options{Service: "songe-hello"})
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("want an error carrying %q, got %v", want, err)
	}
}

func TestACaseCarryingEveryQueryParameterSendsThemOnTheUriAndAssertsThemOnTheMock(t *testing.T) {
	vectors := `{"cases": [{"case": "a", "operation": "listGreetings", "input": {"name": "Un Songe", "after": "g1", "limit": 10}, "controllerReply": {"greetings": []}, "expectedStatus": 200, "expectedBody": {"greetings": []}}]}`

	content := generatedVectors(t, queriedSpec, vectors)

	for _, want := range []string{
		`"/greetings?name=Un+Songe&after=g1&limit=10"`,
		`mockall::predicate::eq("Un Songe")`,
		`mockall::predicate::eq(Some("g1".to_string()))`,
		"mockall::predicate::eq(Some(10i64))",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("the emitted test lacks %q:\n%s", want, content)
		}
	}
}

func TestAnAbsentOptionalQueryParameterIsLeftOffTheUriAndAssertedAsNone(t *testing.T) {
	vectors := `{"cases": [{"case": "a", "operation": "listGreetings", "input": {"name": "Songe"}, "controllerReply": {"greetings": []}, "expectedStatus": 200, "expectedBody": {"greetings": []}}]}`

	content := generatedVectors(t, queriedSpec, vectors)

	for _, want := range []string{
		`"/greetings?name=Songe"`,
		"mockall::predicate::eq(None::<String>)",
		"mockall::predicate::eq(None::<i64>)",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("the emitted test lacks %q:\n%s", want, content)
		}
	}
}

func TestAMissingRequiredQueryParameterArmsTheControllerWithNeverBecauseTheDriverRefusesFirst(t *testing.T) {
	vectors := `{"cases": [{"case": "a", "operation": "listGreetings", "input": {"limit": 10}, "expectedStatus": 422, "expectedErrorSubstring": "name"}]}`

	content := generatedVectors(t, queriedSpec, vectors)

	if !strings.Contains(content, "greeting_controller.expect_list_greetings().never();") {
		t.Fatalf("the emitted test still arms the controller:\n%s", content)
	}

	if strings.Contains(content, "?name=") {
		t.Fatalf("the emitted test still sends the missing parameter:\n%s", content)
	}
}

func TestAMissingRequiredQueryParameterOnASuccessCaseIsRefused(t *testing.T) {
	vectors := `{"cases": [{"case": "a", "operation": "listGreetings", "input": {}, "controllerReply": {"greetings": []}, "expectedStatus": 200, "expectedBody": {"greetings": []}}]}`

	refusedVectors(t, queriedSpec, vectors, "the driver refuses before the controller answers")
}

func TestAMissingRequiredQueryParameterWithTheWrongStatusIsRefused(t *testing.T) {
	vectors := `{"cases": [{"case": "a", "operation": "listGreetings", "input": {}, "expectedStatus": 404, "expectedErrorSubstring": "name"}]}`

	refusedVectors(t, queriedSpec, vectors, "so the driver answers 422, not 404")
}

func TestAQueryParameterOfTheWrongJsonTypeIsRefused(t *testing.T) {
	vectors := `{"cases": [{"case": "a", "operation": "listGreetings", "input": {"name": "Songe", "limit": "ten"}, "controllerReply": {"greetings": []}, "expectedStatus": 200, "expectedBody": {"greetings": []}}]}`

	refusedVectors(t, queriedSpec, vectors, `query parameter "limit" must be a JSON integer`)
}

const boundedSpec = `
openapi: 3.1.0
info:
  title: Bounds API
  version: 1.0.0
paths:
  /sign-in:
    post:
      operationId: signIn
      x-controller: account
      requestBody:
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/SignInRequest"
      responses:
        "200":
          description: Signed in
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Signed"
        "422":
          description: Refused
components:
  schemas:
    SignInRequest:
      type: object
      required: [provider, nickname]
      properties:
        provider:
          type: string
          enum: [google]
        nickname:
          type: string
          minLength: 3
          maxLength: 6
    Signed:
      type: object
      required: [provider]
      properties:
        provider:
          type: string
`

func TestACaseWhoseBodyBreaksADeclaredBoundArmsTheControllerWithNeverBecauseTheDriverRefusesFirst(t *testing.T) {
	vectors := `{"cases": [
		{"case": "apple", "operation": "signIn", "input": {"provider": "apple", "nickname": "Songe"}, "expectedStatus": 422, "expectedErrorSubstring": "provider"},
		{"case": "seven", "operation": "signIn", "input": {"provider": "google", "nickname": "Songeur"}, "expectedStatus": 422, "expectedErrorSubstring": "nickname"},
		{"case": "six_letters_of_seven_bytes", "operation": "signIn", "input": {"provider": "google", "nickname": "Rêveur"}, "controllerReply": {"provider": "google"}, "expectedStatus": 200}
	]}`

	content := generatedVectors(t, boundedSpec, vectors)

	for _, name := range []string{"apple", "seven"} {
		start := strings.Index(content, "async fn "+name+"()")
		if start < 0 {
			t.Fatalf("no test %s was emitted:\n%s", name, content)
		}

		body := content[start:]
		if end := strings.Index(body[1:], "async fn "); end >= 0 {
			body = body[:end+1]
		}

		if !strings.Contains(body, "account_controller.expect_sign_in().never();") {
			t.Fatalf("the test %s still arms the controller:\n%s", name, body)
		}
	}

	if !strings.Contains(content, ".times(1)") {
		t.Fatalf("the case inside its bounds no longer arms the controller:\n%s", content)
	}
}

func TestAPaddedBodyValueIsExpectedTrimmedAtTheControllerWhileTheRequestCarriesItPadded(t *testing.T) {
	vectors := `{"cases": [{"case": "padded", "operation": "signIn", "input": {"provider": " google ", "nickname": "  Songe  "}, "controllerReply": {"provider": "google"}, "expectedStatus": 200}]}`

	content := generatedVectors(t, boundedSpec, vectors)

	for _, want := range []string{
		`serde_json::from_str::<SignInRequest>("{\"nickname\":\"Songe\",\"provider\":\"google\"}")`,
		`Body::from("{\"provider\":\" google \",\"nickname\":\"  Songe  \"}")`,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("the emitted test lacks %q:\n%s", want, content)
		}
	}
}

func TestABodyValueOfOnlySpacesBreaksAMinLengthBecauseTheDriverTrimsItFirst(t *testing.T) {
	vectors := `{"cases": [{"case": "spaces", "operation": "signIn", "input": {"provider": "google", "nickname": "     "}, "expectedStatus": 422, "expectedErrorSubstring": "nickname"}]}`

	content := generatedVectors(t, boundedSpec, vectors)

	if !strings.Contains(content, "account_controller.expect_sign_in().never();") {
		t.Fatalf("the emitted test still arms the controller:\n%s", content)
	}
}

func TestABodyBoundBreakOnASuccessCaseIsRefused(t *testing.T) {
	vectors := `{"cases": [{"case": "a", "operation": "signIn", "input": {"provider": "apple", "nickname": "Songe"}, "controllerReply": {"provider": "apple"}, "expectedStatus": 200}]}`

	refusedVectors(t, boundedSpec, vectors, `reading vector "a": input breaks the declared bound of the body property "provider", and the driver refuses before the controller answers`)
}

func TestABodyBoundBreakExpectingAStatusOtherThanTheInvalidStatusIsRefused(t *testing.T) {
	vectors := `{"cases": [{"case": "a", "operation": "signIn", "input": {"provider": "google", "nickname": "Al"}, "expectedStatus": 409, "expectedErrorSubstring": "nickname"}]}`

	refusedVectors(t, boundedSpec, vectors, `reading vector "a": input breaks the declared bound of the body property "nickname", so the driver answers 422, not 409`)
}

func TestANamedErrorOnACaseWhoseBodyBreaksABoundIsRefusedBecauseTheControllerNeverRuns(t *testing.T) {
	vectors := `{"cases": [{"case": "a", "operation": "signIn", "input": {"provider": "google", "nickname": "Al"}, "expectedStatus": 422, "expectedError": "Invalid", "expectedErrorSubstring": "nickname"}]}`

	refusedVectors(t, boundedSpec, vectors, `reading vector "a": expectedError "Invalid" names an error the controller answers, and input breaks the declared bound of the body property "nickname", so the driver refuses before the controller runs and the case names no expectedError`)
}

func TestANamedErrorOnACaseMissingARequiredQueryParameterIsRefusedBecauseTheControllerNeverRuns(t *testing.T) {
	vectors := `{"cases": [{"case": "a", "operation": "listGreetings", "input": {"limit": 10}, "expectedStatus": 422, "expectedError": "Invalid", "expectedErrorSubstring": "name"}]}`

	refusedVectors(t, queriedSpec, vectors, `reading vector "a": expectedError "Invalid" names an error the controller answers, and input names no value for the required query parameter "name", so the driver refuses before the controller runs and the case names no expectedError`)
}

func TestANamedErrorOutsideTheTaxonomyIsRefusedWithTheTaxonomyListEvenWhenTheDriverRefusesFirst(t *testing.T) {
	vectors := `{"cases": [{"case": "a", "operation": "signIn", "input": {"provider": "google", "nickname": "Al"}, "expectedStatus": 422, "expectedError": "Validation", "expectedErrorSubstring": "nickname"}]}`

	refusedVectors(t, boundedSpec, vectors, `reading vector "a": expectedError "Validation" names no member a controller answers, the members are `+strings.Join(taxonomy.DetailedVariantNames(), ", ")+" and Unauthenticated")
}

func TestACaseWithoutATicketSubjectIsRefusedByTheVerifierBeforeTheDriverReadsItsBody(t *testing.T) {
	guarded := strings.Replace(boundedSpec, "      x-controller: account\n", "      x-controller: account\n      x-auth: bearer\n", 1)
	if guarded == boundedSpec {
		t.Fatal("the x-controller line was never found, the fixture moved")
	}

	vectors := `{"cases": [{"case": "a", "operation": "signIn", "input": {"provider": "google", "nickname": "Al"}, "expectedStatus": 401, "expectedError": "Unauthenticated", "expectedErrorSubstring": "bearer"}]}`

	content := generatedVectors(t, guarded, vectors)

	for _, want := range []string{"account_controller.expect_sign_in().never();", "ticket_verifier.expect_verify().never();"} {
		if !strings.Contains(content, want) {
			t.Fatalf("the emitted test lacks %q:\n%s", want, content)
		}
	}
}

func TestEveryNewTaxonomyStatusArmsItsOwnControllerError(t *testing.T) {
	cases := []struct {
		name   string
		status int
		want   string
	}{
		{
			name:   "authentication",
			status: 401,
			want:   `Err(GreetingControllerError::Authentication { subject: "boom".to_string(), reason: "generated by vectors-rust".to_string() })`,
		},
		{
			name:   "authorization",
			status: 403,
			want:   `Err(GreetingControllerError::Authorization { subject: "boom".to_string(), reason: "generated by vectors-rust".to_string() })`,
		},
		{
			name:   "semantic",
			status: 409,
			want:   `Err(GreetingControllerError::Semantic { resource: "boom".to_string(), reason: "generated by vectors-rust".to_string() })`,
		},
		{
			name:   "rate limiting",
			status: 429,
			want:   `Err(GreetingControllerError::RateLimited { subject: "boom".to_string(), reason: "generated by vectors-rust".to_string() })`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vectors := `{"cases": [{"case": "a", "operation": "listGreetings", "input": {"name": "Songe"}, "expectedStatus": ` +
				strconv.Itoa(tc.status) + `, "expectedErrorSubstring": "boom"}]}`

			content := generatedVectors(t, queriedSpec, vectors)

			if !strings.Contains(content, tc.want) {
				t.Fatalf("the emitted test lacks %q:\n%s", tc.want, content)
			}
		})
	}
}
