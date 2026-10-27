package restrust_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/restrust"
)

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
  /notes/{id}:
    get:
      operationId: getNote
      x-controller: account
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
        - name: lang
          in: query
          schema:
            type: string
      responses:
        "200":
          description: The note
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Signed"
components:
  schemas:
    SignInRequest:
      type: object
      required: [provider, idToken]
      properties:
        provider:
          type: string
          enum: [google]
        idToken:
          type: string
          minLength: 1
        nickname:
          type: string
          minLength: 3
          maxLength: 6
        count:
          type: integer
    Signed:
      type: object
      required: [provider]
      properties:
        provider:
          type: string
`

const boundedDeclarations = `        provider:
          type: string
          enum: [google]
        idToken:
          type: string
          minLength: 1
        nickname:
          type: string
          minLength: 3
          maxLength: 6
`

const unboundedDeclarations = `        provider:
          type: string
        idToken:
          type: string
        nickname:
          type: string
`

const enumCheck = `one_of("provider", &body.provider, &["google"], StatusCode::UNPROCESSABLE_ENTITY)?;`

const requiredLengthCheck = `length_within("idToken", &body.id_token, 1, None, StatusCode::UNPROCESSABLE_ENTITY)?;`

const optionalLengthCheck = `if let Some(value) = &body.nickname { length_within("nickname", value, 3, Some(6), StatusCode::UNPROCESSABLE_ENTITY)?; }`

func boundedDriver(t *testing.T, spec string) string {
	t.Helper()

	byPath, err := generatedByPath(t, spec)
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	driver, ok := byPath["driver/zz_generated_http_driver.rs"]
	if !ok {
		t.Fatal("no http driver was generated")
	}

	return driver
}

func checkedBeforeTheController(t *testing.T, driver, check string) {
	t.Helper()

	handler := strings.Index(driver, "async fn sign_in(")
	if handler < 0 {
		t.Fatalf("the driver has no sign_in handler:\n%s", driver)
	}

	body := driver[handler:]
	at := strings.Index(body, check)
	call := strings.Index(body, ".sign_in(")

	if at < 0 || call < 0 || at > call {
		t.Fatalf("the sign_in handler does not run %q before the controller call:\n%s", check, body)
	}
}

func TestARequestBodyPropertyDeclaringAStringEnumIsCheckedInTheHandlerBeforeTheControllerRuns(t *testing.T) {
	driver := boundedDriver(t, boundedSpec)

	checkedBeforeTheController(t, driver, enumCheck)

	for _, want := range []string{
		"fn one_of(name: &str, value: &str, allowed: &[&str], invalid: StatusCode) -> Result<(), Rejection> {",
		`format!("validating {name:?}: {value:?} is not one of {}", allowed.join(", ")),`,
	} {
		if !strings.Contains(driver, want) {
			t.Fatalf("the driver lacks %q:\n%s", want, driver)
		}
	}
}

func TestARequestBodyPropertyDeclaringALengthIsCheckedInTheHandlerBeforeTheControllerRuns(t *testing.T) {
	driver := boundedDriver(t, boundedSpec)

	checkedBeforeTheController(t, driver, requiredLengthCheck)

	for _, want := range []string{
		"fn length_within(",
		"let length = value.chars().count();",
		`format!("validating {name:?}: the value holds {length} characters and the property takes {bounds}"),`,
	} {
		if !strings.Contains(driver, want) {
			t.Fatalf("the driver lacks %q:\n%s", want, driver)
		}
	}
}

func TestAnOptionalBoundedPropertyIsCheckedOnlyWhenTheBodyCarriesIt(t *testing.T) {
	driver := boundedDriver(t, boundedSpec)

	checkedBeforeTheController(t, driver, optionalLengthCheck)
}

func TestEveryStringPropertyOfARequestBodyIsTrimmedBeforeItsBoundsAndBeforeTheControllerRuns(t *testing.T) {
	driver := boundedDriver(t, boundedSpec)

	checkedBeforeTheController(t, driver, "Json(mut body): Json<SignInRequestWire>")

	for _, pair := range [][2]string{
		{"body.id_token = body.id_token.trim().to_string();", requiredLengthCheck},
		{"body.nickname = body.nickname.map(|value| value.trim().to_string());", optionalLengthCheck},
		{"body.provider = body.provider.trim().to_string();", enumCheck},
	} {
		checkedBeforeTheController(t, driver, pair[0])

		trimAt := strings.Index(driver, pair[0])
		checkAt := strings.Index(driver, pair[1])

		if checkAt < 0 || trimAt > checkAt {
			t.Fatalf("the driver does not trim with %q before it checks with %q:\n%s", pair[0], pair[1], driver)
		}
	}

	if strings.Contains(driver, "body.count =") {
		t.Fatalf("the driver trims the integer property count:\n%s", driver)
	}
}

func TestAnOperationWithNoBoundInItsBodyEmitsNoBoundHelper(t *testing.T) {
	unbounded := strings.Replace(boundedSpec, boundedDeclarations, unboundedDeclarations, 1)
	if unbounded == boundedSpec {
		t.Fatal("the bounded declarations were never found, the fixture moved")
	}

	driver := boundedDriver(t, unbounded)

	for _, unwanted := range []string{"fn one_of(", "fn length_within(", "one_of(\"", "length_within(\""} {
		if strings.Contains(driver, unwanted) {
			t.Fatalf("the driver carries %q while no body declares a bound:\n%s", unwanted, driver)
		}
	}
}

func TestABoundDeclarationIsRefusedWhenItBreaksTheContract(t *testing.T) {
	cases := []struct {
		name    string
		find    string
		replace string
		want    string
	}{
		{
			name:    "an enum on a property that is not a string",
			find:    "        count:\n          type: integer\n",
			replace: "        count:\n          type: integer\n          enum: [1, 2]\n",
			want:    `reading property "count" of schema "SignInRequest": enum is declared on a property of type integer, the engine checks an enum on a string property only`,
		},
		{
			name:    "an enum value that is not a string",
			find:    "          enum: [google]\n",
			replace: "          enum: [google, 7]\n",
			want:    `reading property "provider" of schema "SignInRequest": enum value 7 is not a string, every enum value of a string property is a string`,
		},
		{
			name:    "a null enum value",
			find:    "          enum: [google]\n",
			replace: "          enum: [google, null]\n",
			want:    `reading property "provider" of schema "SignInRequest": enum value null is not a string, every enum value of a string property is a string`,
		},
		{
			name:    "an empty enum",
			find:    "          enum: [google]\n",
			replace: "          enum: []\n",
			want:    `reading property "provider" of schema "SignInRequest": enum is an empty list, an enum lists at least one value a request may carry`,
		},
		{
			name:    "a length on a property that is not a string",
			find:    "        count:\n          type: integer\n",
			replace: "        count:\n          type: integer\n          maxLength: 4\n",
			want:    `reading property "count" of schema "SignInRequest": maxLength is declared on a property of type integer, a length bounds a string property only`,
		},
		{
			name:    "a negative length",
			find:    "          minLength: 1\n",
			replace: "          minLength: -1\n",
			want:    `reading property "idToken" of schema "SignInRequest": minLength is -1, a length counts characters and is never below zero`,
		},
		{
			name:    "a minLength above the maxLength",
			find:    "          minLength: 3\n          maxLength: 6\n",
			replace: "          minLength: 7\n          maxLength: 6\n",
			want:    `reading property "nickname" of schema "SignInRequest": minLength 7 is above maxLength 6, no value fits`,
		},
		{
			name:    "an enum on a query parameter",
			find:    "        - name: lang\n          in: query\n          schema:\n            type: string\n",
			replace: "        - name: lang\n          in: query\n          schema:\n            type: string\n            enum: [fr]\n",
			want:    `reading GET /notes/{id}: query parameter "lang" declares enum, the engine checks enum, minLength and maxLength on a request body property only`,
		},
		{
			name:    "a minLength on a query parameter",
			find:    "        - name: lang\n          in: query\n          schema:\n            type: string\n",
			replace: "        - name: lang\n          in: query\n          schema:\n            type: string\n            minLength: 2\n",
			want:    `reading GET /notes/{id}: query parameter "lang" declares minLength, the engine checks enum, minLength and maxLength on a request body property only`,
		},
		{
			name:    "a maxLength on a path parameter",
			find:    "          in: path\n          required: true\n          schema:\n            type: string\n",
			replace: "          in: path\n          required: true\n          schema:\n            type: string\n            maxLength: 36\n",
			want:    `reading GET /notes/{id}: path parameter "id" declares maxLength, the engine checks enum, minLength and maxLength on a request body property only`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			broken := strings.Replace(boundedSpec, tc.find, tc.replace, 1)
			if broken == boundedSpec {
				t.Fatalf("%q was never found, the fixture moved", tc.find)
			}

			_, err := restrust.Generate([]byte(broken), restrust.Options{Service: "songe-bounds"})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want an error carrying %q, got %v", tc.want, err)
			}
		})
	}
}

const boundedCrateManifest = `[workspace]

[package]
name = "songe-bounds"
version = "0.1.0"
edition = "2021"

[dependencies]
axum = "0.8"
serde = { version = "1", features = ["derive"] }
serde_json = "1"
thiserror = "2"
tokio = { version = "1", features = ["full"] }

[dev-dependencies]
mockall = "0.15"
tower = "0.5"
`

const boundedControllerImpl = `use crate::rest::controller::{AccountController, AccountControllerError, AccountControllerImpl};
use crate::rest::types::sign_in_request::SignInRequest;
use crate::rest::types::signed::Signed;

impl AccountController for AccountControllerImpl {
    fn sign_in(&self, body: SignInRequest) -> Result<Signed, AccountControllerError> {
        Ok(Signed {
            provider: body.provider,
        })
    }

    fn get_note(&self, id: &str, _lang: Option<String>) -> Result<Signed, AccountControllerError> {
        Err(AccountControllerError::NotFound { id: id.to_string() })
    }
}
`

const boundedRouterTest = `use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;

use axum::body::{to_bytes, Body};
use axum::http::Request;
use tower::ServiceExt;

use songe_bounds::rest::controller::{AccountController, AccountControllerError};
use songe_bounds::rest::driver::http_driver::{HttpDriver, HttpDriverConfig};
use songe_bounds::rest::types::sign_in_request::SignInRequest;
use songe_bounds::rest::types::signed::Signed;

struct PanickingController;

impl AccountController for PanickingController {
    fn sign_in(&self, _body: SignInRequest) -> Result<Signed, AccountControllerError> {
        panic!("the controller ran on a body the driver should have refused")
    }

    fn get_note(&self, _id: &str, _lang: Option<String>) -> Result<Signed, AccountControllerError> {
        panic!("the controller ran on a body the driver should have refused")
    }
}

struct RecordingController {
    reached: Arc<AtomicBool>,
}

impl AccountController for RecordingController {
    fn sign_in(&self, body: SignInRequest) -> Result<Signed, AccountControllerError> {
        self.reached.store(true, Ordering::SeqCst);

        Ok(Signed {
            provider: body.provider,
        })
    }

    fn get_note(&self, id: &str, _lang: Option<String>) -> Result<Signed, AccountControllerError> {
        Err(AccountControllerError::NotFound { id: id.to_string() })
    }
}

async fn sign_in(driver: &HttpDriver, body: &str) -> (u16, serde_json::Value) {
    let request = Request::builder()
        .method("POST")
        .uri("/sign-in")
        .header("content-type", "application/json")
        .body(Body::from(body.to_string()))
        .unwrap();

    let response = driver.router().oneshot(request).await.unwrap();
    let status = response.status().as_u16();
    let bytes = to_bytes(response.into_body(), usize::MAX).await.unwrap();

    (status, serde_json::from_slice(&bytes).unwrap())
}

#[tokio::test]
async fn a_value_outside_its_bounds_is_refused_as_validation_and_never_reaches_the_controller() {
    let driver = HttpDriver::new(HttpDriverConfig::default(), Arc::new(PanickingController));

    for (body, property) in [
        (r#"{"provider":"apple","idToken":"token"}"#, "provider"),
        (r#"{"provider":"google","idToken":""}"#, "idToken"),
        (r#"{"provider":"google","idToken":"   "}"#, "idToken"),
        (r#"{"provider":"google","idToken":"token","nickname":"Al"}"#, "nickname"),
        (r#"{"provider":"google","idToken":"token","nickname":"Songeur"}"#, "nickname"),
    ] {
        let (status, rejection) = sign_in(&driver, body).await;

        assert_eq!(status, 422, "{body}");
        assert_eq!(rejection["type"], "validation", "{body}");
        assert!(
            rejection["message"].as_str().unwrap().contains(property),
            "{body} answered {rejection}"
        );
    }
}

#[tokio::test]
async fn a_padded_value_inside_its_bounds_reaches_the_controller_trimmed() {
    let reached = Arc::new(AtomicBool::new(false));
    let driver = HttpDriver::new(
        HttpDriverConfig::default(),
        Arc::new(RecordingController {
            reached: reached.clone(),
        }),
    );

    let (status, signed) = sign_in(
        &driver,
        r#"{"provider":"  google  ","idToken":" t ","nickname":" Rêveur "}"#,
    )
    .await;

    assert_eq!(status, 200, "{signed}");
    assert_eq!(signed["provider"], "google");
    assert!(reached.load(Ordering::SeqCst));
}
`

func TestTheGeneratedRouterRefusesAValueOutsideItsBoundsAsValidationAndNeverCallsTheController(t *testing.T) {
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Fatalf("this test drives the generated router through cargo and cargo is not on PATH: %v", err)
	}

	files, err := restrust.Generate([]byte(boundedSpec), restrust.Options{Service: "songe-bounds"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	root := t.TempDir()

	write := func(rel, content string) {
		t.Helper()

		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("making %s: %v", filepath.Dir(p), err)
		}

		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", p, err)
		}
	}

	write("Cargo.toml", boundedCrateManifest)
	write("src/lib.rs", "pub mod rest;\n")

	for _, f := range files {
		if strings.HasSuffix(f.Path, ".rs") {
			write(filepath.Join("src", "rest", f.Path), f.Content)
		}
	}

	write("src/rest/controller/account_controller.rs", boundedControllerImpl)
	write("tests/bounds.rs", boundedRouterTest)

	cmd := exec.Command(cargo, "test", "--test", "bounds")
	cmd.Dir = root

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cargo test --test bounds: %v\n%s", err, out)
	}

	for _, ran := range []string{
		"test a_value_outside_its_bounds_is_refused_as_validation_and_never_reaches_the_controller ... ok",
		"test a_padded_value_inside_its_bounds_reaches_the_controller_trimmed ... ok",
	} {
		if !strings.Contains(string(out), ran) {
			t.Fatalf("cargo never reported %q:\n%s", ran, out)
		}
	}
}
