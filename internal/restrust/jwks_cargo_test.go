package restrust_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/restrust"
)

const jwksSpec = `
openapi: 3.1.0
info:
  title: Identity API
  version: 1.0.0
paths:
  /identity:
    get:
      operationId: getIdentity
      x-controller: identity
      x-ports:
        - kind: verifier
          name: GoogleVerifier
          subject: GoogleIdentity
          adapters: [jwks, secret]
          jwks:
            url: https://www.googleapis.com/oauth2/v3/certs
            issuers: [https://accounts.google.com, accounts.google.com]
      responses:
        "200":
          description: The identity the verifier answered
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/GoogleIdentity"
components:
  schemas:
    GoogleIdentity:
      type: object
      required: [sub, email]
      properties:
        sub:
          type: string
        email:
          type: string
`

const jwksCrateManifest = `[workspace]

[package]
name = "songe-identity"
version = "0.1.0"
edition = "2021"

[dependencies]
axum = "0.8.9"
jsonwebtoken = { version = "11.0.0", features = ["aws_lc_rs"] }
reqwest = { version = "0.13.4", default-features = false, features = ["json", "rustls"] }
serde = { version = "1.0.229", features = ["derive"] }
serde_json = "1.0.151"
thiserror = "2.0.20"
tokio = { version = "1.53.1", features = ["full"] }

[dev-dependencies]
mockall = "0.15.0"
`

const identityControllerImpl = `use crate::rest::controller::{
    IdentityController, IdentityControllerError, IdentityControllerImpl,
};
use crate::rest::types::google_identity::GoogleIdentity;

impl IdentityController for IdentityControllerImpl {
    fn get_identity(&self) -> Result<GoogleIdentity, IdentityControllerError> {
        self.google_verifier
            .verify("offered")
            .map_err(|source| IdentityControllerError::GoogleVerifier {
                id: "offered".to_string(),
                source,
            })
    }
}
`

const jwksRustTest = `use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::Arc;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use axum::extract::State;
use axum::http::{header, StatusCode};
use axum::response::IntoResponse;
use axum::routing::get;
use axum::{Json, Router};
use jsonwebtoken::{encode, Algorithm, EncodingKey, Header};
use serde_json::{json, Value};

use songe_identity::rest::adapter::google_verifier_jwks::{
    GoogleVerifierJwks, GoogleVerifierJwksConfig,
};
use songe_identity::rest::port::google_verifier::{GoogleVerifier, GoogleVerifierError};
use songe_identity::rest::types::google_identity::GoogleIdentity;

const PRIVATE_KEY: &str = r#"-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDO8lybnLJRFKqT
oiiJtCF8YeD0/bk1mm8f7YINmGkcuhX5bMpgf6MaoWHYGi8CQ+4QcYxZ8f9Dbql/
6N2stYvP3pCcRiioelxxcfFkDNb6NwB0EiolJFyWLvp3WWRISZPFdzdrM3jg0XUB
CQNUThre+hLNQbHL7xCsqGH2/QeEVDmdAA89jPV9ww6MAPPo67Gi1/Fkhh8CxaIq
xFtCxIEqEtR+hhD6lcifuLLtlnTwL3Zxmc4VhYAS1Os9u/vhRa3yluaNPGZhj4zk
1tkDKwQSYXQY3mA3ucQ/zGXfhyCLjx1aiKgoJpVcrtbEfTG1PQVrTerZ2RbDct98
OKvwZYdnAgMBAAECggEAKBmIXlLXrqRdhSvnxP8Vef+RUoeTitJ48+vWxoGg5IMe
XO16rYYt+awJxJEWSM8W4zLNamRc/RkFWRarVZr8E3Rh+xYRLMxA+26DwspGIcfk
m5kld3r7QdKqj9PPfb5cFuhJ4cjghFex31h6v6w+wrQcTo+LPyP8zCf4i55uBXLf
L9Aak/pIuI+hXfNI3Quu5S+yMniQk33PTOK5WwRXbLmX8pwoKaORlR6x8DgJXcEW
M18HwK/T4tUWO4svr+okuScBRhg7LXG3+cnr9X7a2IkI/CBxMuQDs9mwGGwBQuiQ
e30UqV/qMNhMT1OSnivq9N9A/rloAFCRUhkvFQlosQKBgQD1jx+HJR2obTCK4kpe
ackygc8K897U4zjTU6C5mQV+T8eeSE0Mmwr1KJwVtaTIsf+2RoZLl8DwKuHD1ivN
UFrO5S5HWN2agv8DDMrxRAf2bSNtn2zC6XLo3B1gqgL2GB+NOMEKIWLsNtt9wkBb
g70FlpHaNjIvCj63l16POnjH+QKBgQDXvvLO2iViQUoqNJiXTeUxrjeKXsb1X5Zt
lDsSVlqrqdS+V4X9Ljftb7pbj+swobkOEuac7O7x8kq+Z0XFJvwnqMzdRbeSSA8h
Aa1S8bMaZfcisZ5XRDaOdxGek7ey46UJgjWaJY5Eo7WHgMxzdZtP8oaGcwUsrE+E
39J9BrZiXwKBgGzr9WIu9n7VICkawz4p0lMS/VGwZvErcPNcLvjkhWlH3hW5oGOt
CilAwCtpDRTwz6p8kelKAQ0HGGmTwI4rzHvlcKe5JWL6JD+oFwT51kZaltJzWv0C
LDRaHrXX88RRYv1zZT0nHggFNUsNNRrizp+B8GRIke4Hr1YkUI9bsmORAoGBAL/m
G2j8RmZH9qTl1Zg3dBalhMdyF/wnQtFDGOvtnYa1cALCLO8PDjYGvDc3IxZFsyme
jDumz76rKeqER13emEjabyaeHXNi3voX2Mg/LnTbn1/fpxyuEuzTYfuwHqRisehs
RitPHmVVTGEzoidMw5HwAdJTOpYKykxBHaORteZ3AoGAXgH1WFr2Xsz2nqbGovij
TKJP5fyYs5+E/8nTMpZejSnLxTe8qTnK4ht5yGzloD1eaVL4JEGo/1EQCbqDmiHL
SAsePaaPvi+iycn1E1kWcoc40pkOqCwf65qDpGAUnHzuEg77Bv5yClv97MZERyxS
AWFH679PT+XLfbCAy6BuHCE=
-----END PRIVATE KEY-----
"#;

const MODULUS: &str = "zvJcm5yyURSqk6IoibQhfGHg9P25NZpvH-2CDZhpHLoV-WzKYH-jGqFh2BovAkPuEHGMWfH_Q26pf-jdrLWLz96QnEYoqHpccXHxZAzW-jcAdBIqJSRcli76d1lkSEmTxXc3azN44NF1AQkDVE4a3voSzUGxy-8QrKhh9v0HhFQ5nQAPPYz1fcMOjADz6OuxotfxZIYfAsWiKsRbQsSBKhLUfoYQ-pXIn7iy7ZZ08C92cZnOFYWAEtTrPbv74UWt8pbmjTxmYY-M5NbZAysEEmF0GN5gN7nEP8xl34cgi48dWoioKCaVXK7WxH0xtT0Fa03q2dkWw3LffDir8GWHZw";

const KEY_ID: &str = "test-key";

const SHARED_KEY_ID: &str = "shared-key";

const SHARED_SECRET: &[u8] = b"anyone-can-read-this";

const SHARED_SECRET_BASE64: &str = "YW55b25lLWNhbi1yZWFkLXRoaXM";

const AUDIENCE: &str = "songe-client";

const ISSUER: &str = "https://accounts.google.com";

struct KeySet {
    body: Value,
    cache_control: String,
    fetches: AtomicUsize,
}

impl KeySet {
    fn fetches(&self) -> usize {
        self.fetches.load(Ordering::SeqCst)
    }
}

async fn certs(State(set): State<Arc<KeySet>>) -> impl IntoResponse {
    set.fetches.fetch_add(1, Ordering::SeqCst);

    (
        [(header::CACHE_CONTROL, set.cache_control.clone())],
        Json(set.body.clone()),
    )
}

async fn serve(body: Value, max_age: u64) -> (String, Arc<KeySet>) {
    serve_with(body, format!("public, max-age={max_age}")).await
}

async fn serve_with(body: Value, cache_control: String) -> (String, Arc<KeySet>) {
    let set = Arc::new(KeySet {
        body,
        cache_control,
        fetches: AtomicUsize::new(0),
    });

    let url = listen(Router::new().route("/certs", get(certs)).with_state(set.clone())).await;

    (url, set)
}

async fn answer(status: StatusCode, body: String) -> String {
    listen(Router::new().route(
        "/certs",
        get(move || {
            let body = body.clone();
            async move { (status, body) }
        }),
    ))
    .await
}

async fn listen(app: Router) -> String {
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0")
        .await
        .expect("binding the key set server");
    let addr = listener.local_addr().expect("reading the key set address");

    tokio::spawn(async move { axum::serve(listener, app).await });

    format!("http://{addr}/certs")
}

fn without(claim: &str) -> Value {
    let mut stripped = claims();
    stripped
        .as_object_mut()
        .expect("reading the claims")
        .remove(claim);

    stripped
}

fn rsa_key() -> Value {
    json!({ "kty": "RSA", "kid": KEY_ID, "alg": "RS256", "use": "sig", "n": MODULUS, "e": "AQAB" })
}

fn listed_keys() -> Value {
    json!({ "keys": [rsa_key()] })
}

fn now() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("reading the clock")
        .as_secs()
}

fn claims() -> Value {
    json!({
        "iss": ISSUER,
        "aud": AUDIENCE,
        "sub": "google-123",
        "email": "player@songe.test",
        "email_verified": true,
        "exp": now() + 3600,
    })
}

fn sign_with(kid: &str, claims: &Value) -> String {
    let mut header = Header::new(Algorithm::RS256);
    header.kid = Some(kid.to_string());

    let key = EncodingKey::from_rsa_pem(PRIVATE_KEY.as_bytes()).expect("reading the test key");

    encode(&header, claims, &key).expect("signing the token")
}

fn sign(claims: &Value) -> String {
    sign_with(KEY_ID, claims)
}

fn verifier(url: String) -> GoogleVerifierJwks {
    GoogleVerifierJwks::new(GoogleVerifierJwksConfig {
        audience: AUDIENCE.to_string(),
        url,
    })
}

fn refusal(answer: Result<GoogleIdentity, GoogleVerifierError>) -> String {
    match answer {
        Err(GoogleVerifierError::Refused { reason, .. }) => reason,
        other => panic!("want a refusal, got {other:?}"),
    }
}

#[tokio::test(flavor = "multi_thread")]
async fn a_token_signed_by_a_listed_key_for_the_configured_audience_answers_its_sub_and_email() {
    let (url, _set) = serve(listed_keys(), 3600).await;

    let identity = verifier(url).verify(&sign(&claims())).expect("verifying the token");

    assert_eq!(
        identity,
        GoogleIdentity {
            sub: "google-123".to_string(),
            email: "player@songe.test".to_string(),
        }
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn a_token_for_another_audience_is_refused_naming_the_audience() {
    let (url, _set) = serve(listed_keys(), 3600).await;
    let mut other = claims();
    other["aud"] = json!("another-client");

    let reason = refusal(verifier(url).verify(&sign(&other)));

    assert!(reason.contains("audience"), "{reason}");
}

#[tokio::test(flavor = "multi_thread")]
async fn an_expired_token_is_refused_naming_the_expiry() {
    let (url, _set) = serve(listed_keys(), 3600).await;
    let mut expired = claims();
    expired["exp"] = json!(now() - 3600);

    let reason = refusal(verifier(url).verify(&sign(&expired)));

    assert!(reason.contains("expired"), "{reason}");
}

#[tokio::test(flavor = "multi_thread")]
async fn a_token_from_another_issuer_is_refused_naming_the_issuer() {
    let (url, _set) = serve(listed_keys(), 3600).await;
    let mut foreign = claims();
    foreign["iss"] = json!("https://issuer.songe.test");

    let reason = refusal(verifier(url).verify(&sign(&foreign)));

    assert!(reason.contains("issuer"), "{reason}");
}

#[tokio::test(flavor = "multi_thread")]
async fn a_token_carrying_no_email_is_refused_naming_the_claim() {
    let (url, _set) = serve(listed_keys(), 3600).await;
    let mut anonymous = claims();
    anonymous
        .as_object_mut()
        .expect("reading the claims")
        .remove("email");

    let reason = refusal(verifier(url).verify(&sign(&anonymous)));

    assert!(
        reason.contains(&format!("missing field {0}email{0}", '\u{60}')),
        "{reason}"
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn a_token_carrying_no_audience_is_refused_naming_the_claim() {
    let (url, _set) = serve(listed_keys(), 3600).await;
    let mut unaddressed = claims();
    unaddressed
        .as_object_mut()
        .expect("reading the claims")
        .remove("aud");

    let reason = refusal(verifier(url).verify(&sign(&unaddressed)));

    assert!(reason.contains("carries no aud claim"), "{reason}");
}

#[tokio::test(flavor = "multi_thread")]
async fn a_token_carrying_no_expiry_is_refused_naming_the_exp_claim() {
    let (url, _set) = serve(listed_keys(), 3600).await;

    let reason = refusal(verifier(url).verify(&sign(&without("exp"))));

    assert!(reason.contains("carries no exp claim"), "{reason}");
}

#[tokio::test(flavor = "multi_thread")]
async fn a_token_carrying_no_issuer_is_refused_naming_the_iss_claim() {
    let (url, _set) = serve(listed_keys(), 3600).await;

    let reason = refusal(verifier(url).verify(&sign(&without("iss"))));

    assert!(reason.contains("carries no iss claim"), "{reason}");
}

#[tokio::test(flavor = "multi_thread")]
async fn a_token_carrying_no_email_verified_claim_is_refused() {
    let (url, _set) = serve(listed_keys(), 3600).await;

    let reason = refusal(verifier(url).verify(&sign(&without("email_verified"))));

    assert!(reason.contains("email_verified"), "{reason}");
}

#[tokio::test(flavor = "multi_thread")]
async fn a_token_whose_header_names_no_key_id_is_refused() {
    let (url, set) = serve(listed_keys(), 3600).await;
    let key = EncodingKey::from_rsa_pem(PRIVATE_KEY.as_bytes()).expect("reading the test key");
    let token = encode(&Header::new(Algorithm::RS256), &claims(), &key).expect("signing the token");

    let reason = refusal(verifier(url).verify(&token));

    assert!(reason.contains("names no key id"), "{reason}");
    assert_eq!(set.fetches(), 0);
}

#[tokio::test(flavor = "multi_thread")]
async fn a_token_whose_email_verified_is_false_is_refused() {
    let (url, _set) = serve(listed_keys(), 3600).await;
    let mut unverified = claims();
    unverified["email_verified"] = json!(false);

    let reason = refusal(verifier(url).verify(&sign(&unverified)));

    assert!(reason.contains("email_verified"), "{reason}");
}

#[tokio::test(flavor = "multi_thread")]
async fn a_token_signed_by_another_key_is_refused_naming_the_key() {
    let (url, _set) = serve(listed_keys(), 3600).await;
    let token = sign(&claims());
    let mut other = claims();
    other["sub"] = json!("google-456");
    let forged = sign(&other);
    let (signed, _) = token.rsplit_once('.').expect("splitting the token");
    let (_, signature) = forged.rsplit_once('.').expect("splitting the forged token");

    let reason = refusal(verifier(url).verify(&format!("{signed}.{signature}")));

    assert!(reason.contains("signature") && reason.contains(KEY_ID), "{reason}");
}

#[tokio::test(flavor = "multi_thread")]
async fn a_second_token_under_a_known_key_within_max_age_never_fetches_the_key_set_again() {
    let (url, set) = serve(listed_keys(), 3600).await;
    let verifier = verifier(url);

    verifier.verify(&sign(&claims())).expect("verifying the first token");
    verifier.verify(&sign(&claims())).expect("verifying the second token");

    assert_eq!(set.fetches(), 1);
}

#[tokio::test(flavor = "multi_thread")]
async fn a_known_key_past_its_max_age_fetches_the_key_set_again() {
    let (url, set) = serve(listed_keys(), 1).await;
    let verifier = verifier(url);

    verifier.verify(&sign(&claims())).expect("verifying the first token");
    verifier.verify(&sign(&claims())).expect("verifying within the max age");
    assert_eq!(set.fetches(), 1);

    tokio::time::sleep(Duration::from_millis(1100)).await;

    verifier.verify(&sign(&claims())).expect("verifying past the max age");
    assert_eq!(set.fetches(), 2);
}

#[tokio::test(flavor = "multi_thread")]
async fn a_key_set_response_with_no_max_age_is_fetched_again_on_the_next_token() {
    let (url, set) = serve_with(listed_keys(), "public".to_string()).await;
    let verifier = verifier(url);

    verifier.verify(&sign(&claims())).expect("verifying the first token");
    verifier.verify(&sign(&claims())).expect("verifying the next token");

    assert_eq!(set.fetches(), 2);
}

#[tokio::test(flavor = "multi_thread")]
async fn a_token_naming_an_unknown_key_fetches_once_and_is_refused_naming_the_key() {
    let (url, set) = serve(listed_keys(), 3600).await;

    let reason = refusal(verifier(url).verify(&sign_with("rotated-out", &claims())));

    assert!(reason.contains("rotated-out"), "{reason}");
    assert_eq!(set.fetches(), 1);
}

#[tokio::test(flavor = "multi_thread")]
async fn two_tokens_naming_two_unknown_key_ids_inside_one_max_age_window_cause_one_fetch() {
    let (url, set) = serve(listed_keys(), 3600).await;
    let verifier = verifier(url);

    let first = refusal(verifier.verify(&sign_with("rotated-out", &claims())));
    let second = refusal(verifier.verify(&sign_with("never-published", &claims())));

    assert!(first.contains("rotated-out"), "{first}");
    assert!(second.contains("never-published"), "{second}");
    assert_eq!(set.fetches(), 1);
}

#[tokio::test(flavor = "multi_thread")]
async fn two_unknown_key_ids_while_the_key_set_url_answers_500_cause_one_fetch() {
    let fetches = Arc::new(AtomicUsize::new(0));
    let counted = fetches.clone();
    let url = listen(Router::new().route(
        "/certs",
        get(move || {
            counted.fetch_add(1, Ordering::SeqCst);
            async { StatusCode::INTERNAL_SERVER_ERROR }
        }),
    ))
    .await;
    let verifier = verifier(url);

    let first = verifier.verify(&sign_with("rotated-out", &claims()));
    let second = refusal(verifier.verify(&sign_with("never-published", &claims())));

    assert!(matches!(first, Err(GoogleVerifierError::Call { .. })), "{first:?}");
    assert_eq!(second, "no published key carries the key id \"never-published\"");
    assert_eq!(fetches.load(Ordering::SeqCst), 1);
}

#[tokio::test(flavor = "multi_thread")]
async fn a_key_past_its_max_age_is_refused_while_the_key_set_url_answers_500() {
    let failing = Arc::new(std::sync::atomic::AtomicBool::new(false));
    let answers = failing.clone();
    let url = listen(Router::new().route(
        "/certs",
        get(move || {
            let status = if answers.load(Ordering::SeqCst) {
                StatusCode::INTERNAL_SERVER_ERROR
            } else {
                StatusCode::OK
            };
            async move { (status, [(header::CACHE_CONTROL, "public, max-age=1")], Json(listed_keys())) }
        }),
    ))
    .await;
    let verifier = verifier(url);

    verifier.verify(&sign(&claims())).expect("verifying within the max age");
    failing.store(true, Ordering::SeqCst);
    tokio::time::sleep(Duration::from_millis(1100)).await;

    let first = verifier.verify(&sign(&claims()));
    let second = refusal(verifier.verify(&sign(&claims())));

    assert!(matches!(first, Err(GoogleVerifierError::Call { .. })), "{first:?}");
    assert_eq!(second, format!("the key id {KEY_ID:?} names no key the verifier trusts now"));
}

#[tokio::test(flavor = "multi_thread")]
async fn a_token_whose_claims_cannot_be_read_is_refused_naming_the_claims() {
    let (url, _set) = serve(listed_keys(), 3600).await;
    let mut unreadable = claims();
    unreadable["sub"] = json!(123);

    let reason = refusal(verifier(url).verify(&sign(&unreadable)));

    assert!(reason.contains("reading the token claims"), "{reason}");
}

fn listed_keys_and(extra: Value) -> Value {
    json!({ "keys": [rsa_key(), extra] })
}

#[tokio::test(flavor = "multi_thread")]
async fn a_key_naming_no_algorithm_is_refused_naming_the_key() {
    let bare = json!({ "kty": "RSA", "kid": "bare-key", "use": "sig", "n": MODULUS, "e": "AQAB" });
    let (url, _set) = serve(listed_keys_and(bare), 3600).await;

    let reason = refusal(verifier(url).verify(&sign_with("bare-key", &claims())));

    assert!(reason.contains("names no algorithm") && reason.contains("bare-key"), "{reason}");
}

#[tokio::test(flavor = "multi_thread")]
async fn a_key_naming_an_algorithm_that_verifies_no_token_is_refused_naming_the_key() {
    let encryption = json!({ "kty": "RSA", "kid": "encryption-key", "alg": "RSA-OAEP", "use": "enc", "n": MODULUS, "e": "AQAB" });
    let (url, _set) = serve(listed_keys_and(encryption), 3600).await;

    let reason = refusal(verifier(url).verify(&sign_with("encryption-key", &claims())));

    assert!(reason.contains("verifies no token") && reason.contains("encryption-key"), "{reason}");
}

#[tokio::test(flavor = "multi_thread")]
async fn a_key_that_cannot_be_read_is_refused_naming_the_key() {
    let broken = json!({ "kty": "RSA", "kid": "broken-key", "alg": "RS256", "use": "sig", "n": "!!!", "e": "AQAB" });
    let (url, _set) = serve(listed_keys_and(broken), 3600).await;

    let reason = refusal(verifier(url).verify(&sign_with("broken-key", &claims())));

    assert!(reason.contains("reading the key") && reason.contains("broken-key"), "{reason}");
}

#[tokio::test(flavor = "multi_thread")]
async fn a_symmetric_key_in_the_set_is_refused() {
    let shared = json!({ "kty": "oct", "kid": SHARED_KEY_ID, "alg": "HS256", "k": SHARED_SECRET_BASE64 });
    let (url, _set) = serve(json!({ "keys": [rsa_key(), shared] }), 3600).await;
    let mut header = Header::new(Algorithm::HS256);
    header.kid = Some(SHARED_KEY_ID.to_string());
    let token = encode(&header, &claims(), &EncodingKey::from_secret(SHARED_SECRET))
        .expect("signing with the shared secret");

    let reason = refusal(verifier(url).verify(&token));

    assert!(reason.contains("symmetric") && reason.contains(SHARED_KEY_ID), "{reason}");
}

#[tokio::test(flavor = "multi_thread")]
async fn a_key_set_that_cannot_be_fetched_is_a_call_failure_and_never_a_refusal() {
    let url = answer(StatusCode::INTERNAL_SERVER_ERROR, listed_keys().to_string()).await;

    let verdict = verifier(url).verify(&sign(&claims()));

    assert!(
        matches!(verdict, Err(GoogleVerifierError::Call { .. })),
        "{verdict:?}"
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn a_key_set_response_whose_body_is_not_json_is_a_call_failure_and_never_a_refusal() {
    let url = answer(StatusCode::OK, "this body is not a key set".to_string()).await;

    let verdict = verifier(url).verify(&sign(&claims()));

    assert!(
        matches!(verdict, Err(GoogleVerifierError::Call { .. })),
        "{verdict:?}"
    );
}
`

const jwksRustTestCount = "26 passed"

const clientRejectionTest = `use std::io::{Read, Write};

use songe_identity::rest::adapter::identity_rest_client::{
    IdentityRestClient, IdentityRestClientConfig,
};
use songe_identity::rest::port::identity_client::{IdentityClient, IdentityClientError};

#[tokio::test(flavor = "multi_thread")]
async fn a_rejection_whose_body_cannot_be_read_fails_by_name_and_never_answers_an_empty_string() {
    let listener = std::net::TcpListener::bind("127.0.0.1:0").expect("binding the rejecting server");
    let addr = listener.local_addr().expect("reading the rejecting address");

    std::thread::spawn(move || {
        let (mut stream, _) = listener.accept().expect("accepting the client");
        let mut request = [0u8; 4096];
        let _ = stream.read(&mut request);
        let _ = stream.write_all(
            b"HTTP/1.1 500 Internal Server Error\r\ncontent-type: application/json\r\ncontent-length: 100\r\n\r\n{\"type\"",
        );
    });

    let client = IdentityRestClient::new(IdentityRestClientConfig {
        base_url: format!("http://{addr}"),
    });

    match client.get_identity() {
        Err(IdentityClientError::Runtime { message, source, .. }) => {
            assert!(message.contains("reading the rejection body"), "{message}");
            assert!(source.is_some(), "{message}");
        }
        other => panic!("want a runtime failure naming the rejection body, got {other:?}"),
    }
}
`

func TestAClientRejectionWhoseBodyCannotBeReadFailsByNameAndNeverAnswersAnEmptyString(t *testing.T) {
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Fatalf("finding cargo, which this contract runs through and never skips: %v", err)
	}

	root := writeJwksCrate(t)

	if err := os.WriteFile(filepath.Join(root, "tests", "client.rs"), []byte(clientRejectionTest), 0o644); err != nil {
		t.Fatalf("writing the client test: %v", err)
	}

	mustCargo(t, cargo, root, "clippy", "--all-targets", "--", "-D", "warnings")

	out := mustCargo(t, cargo, root, "test", "--test", "client")
	if !strings.Contains(out, "1 passed") {
		t.Fatalf("the client rejection contract never reported one passed test:\n%s", out)
	}
}

func TestTheJwksVerifierHoldsItsContractAgainstALocalKeySet(t *testing.T) {
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Fatalf("finding cargo, which this contract runs through and never skips: %v", err)
	}

	root := writeJwksCrate(t)

	mustCargo(t, cargo, root, "clippy", "--all-targets", "--", "-D", "warnings")

	out := mustCargo(t, cargo, root, "test", "--test", "jwks")
	if !strings.Contains(out, jwksRustTestCount) {
		t.Fatalf("the jwks contract never reported %q, so some of its tests did not run:\n%s", jwksRustTestCount, out)
	}
}

func writeJwksCrate(t *testing.T) string {
	t.Helper()

	files, err := restrust.Generate([]byte(jwksSpec), restrust.Options{Service: "songe-identity", Side: restrust.SideBoth})
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

	write("Cargo.toml", jwksCrateManifest)
	write("src/lib.rs", "pub mod rest;\n")

	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".rs") {
			continue
		}

		write(filepath.Join("src", "rest", f.Path), f.Content)
	}

	write("src/rest/controller/identity_controller.rs", identityControllerImpl)
	write("tests/jwks.rs", jwksRustTest)

	return root
}

func mustCargo(t *testing.T, cargo, root string, args ...string) string {
	t.Helper()

	cmd := exec.Command(cargo, args...)
	cmd.Dir = root

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cargo %s: %v\n%s", strings.Join(args, " "), err, out)
	}

	return string(out)
}
