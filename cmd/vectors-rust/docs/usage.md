# vectors-rust

A forge-dev generator that turns declared behavior into rust integration
tests. It reads an OpenAPI document and a vectors document and answers one
file, `app/tests/zz_generated_vectors.rs`, holding one async tokio test per
vector case.

forge-dev never writes outside the engine directory. So the app crate holds
its own cell at its root.

```yaml
name: songe-hello
kind: vectors
language: rust
generator: forge://github.com/alexandremahdhaoui/forge-dev-codegen/cmd/vectors-rust
openapi:
  specPath: ./.forge/spec-cache/hello.v1.yaml
layout:
  appDir: .
  vectors: ./.forge/spec-cache/cases.json
```

The `generate` tool takes the normalized forge-dev model. `name` is the
service, it names the `<name>-core` and `<name>-app` crates the generated
tests import. `openapiSpec` is the OpenAPI document, the same one
`hexagonal-rust` reads to emit the driver and the controller trait this
engine tests against. `vectors` is a JSON document holding one `cases` array.
`appDir` is the app crate root relative to the engine directory and defaults
to `app`. It may sit at the top level of the model or under `layout`.

## The vectors document

```json
{
  "cases": [
    {
      "case": "create_valid_name",
      "operation": "createGreeting",
      "input": { "name": "Songe" },
      "controllerReply": { "id": "...", "name": "Songe", "count": 0 },
      "expectedStatus": 201,
      "expectedBody": { "id": "...", "name": "Songe", "count": 0 }
    },
    {
      "case": "create_empty_name_refused",
      "operation": "createGreeting",
      "input": { "name": "" },
      "expectedStatus": 422,
      "expectedErrorSubstring": "name"
    }
  ]
}
```

| Field | Meaning |
|---|---|
| `case` | Names the generated test. Must be a name Rust can spell for a function. |
| `operation` | The `operationId` the vector exercises. Refused if the spec does not declare it. |
| `input` | The JSON sent on the wire. A path parameter comes from the key matching its name. A body operation sends the whole object as the JSON request body. |
| `controllerReply` | Present on a success case. The controller mock returns it, decoded into the operation's core response type. |
| `expectedStatus` | The HTTP status the driver must answer. |
| `expectedBody` | Present on a success case. Compared to the response body as JSON, so field order never matters. |
| `expectedErrorSubstring` | Present on an error case. The controller mock is armed to fail, and the response body's `message` field must contain this text. |

A case needs `controllerReply` or `expectedErrorSubstring`, never neither.

## Datagram cases

A cell that also declares `proto:` reads the datagram service block. A case
whose `operation` is `udp_<rpc>` becomes a test that binds the generated
`<Service>UdpDriver` on `127.0.0.1:0` over a mocked `<Service>Controller`
and round trips one datagram with the generated `<Service>UdpClient`.

```yaml
openapi:
  specPath: ./.forge/spec-cache/hello.v1.yaml
proto:
  specPath: ./.forge/spec-cache/udp/hello.v1.proto
layout:
  appDir: .
  cell: udp
  vectors: ./.forge/spec-cache/cases.json
```

```json
{
  "case": "udp_echo_returns_the_payload",
  "operation": "udp_echo",
  "input": { "sessionId": "0123456789abcdef", "payload": "songe" },
  "controllerReply": { "payload": "songe" },
  "expectedBody": { "sessionId": "0123456789abcdef", "payload": "songe" }
}
```

`input` carries the request message fields plus a `sessionId` of exactly 16
bytes, the one the client stamps on every datagram. `controllerReply` is
what the mocked controller answers. `expectedBody` is what the client reads
back. A key neither message declares is ignored, so `sessionId` may sit in
both.

The engine folds every method name the way udp-rust does and refuses two
rpcs that fold to one byte. A datagram vector reads strings, numbers and
booleans only.

Without a `proto:` a `udp_` case is skipped with a log line, like any other
transport.

An error case is armed by matching `expectedStatus` against the operation's
declared shape: 404 arms `NotFound`, the operation's declared invalid status
(422 or 400) arms `Invalid`, 501 arms `NotImplemented`, and anything else
falls back to the operation's first store port, arming a wrapped store
error that the driver reports as a generic `500` "internal error". Because
the engine has no business knowledge beyond the spec, it embeds
`expectedErrorSubstring` itself into the chosen error's identifier field, so
the constructed message is guaranteed to contain it wherever the driver's
rejection text includes that field. The one case this cannot cover is the
underlying `500` reply, whose body is a fixed "internal error" text set by
the driver, not by the controller error's message.

## How the mock is built

`hexagonal-rust` marks the controller trait `#[cfg_attr(test, mockall::automock)]`.
That mock only exists when the core crate itself is compiled under test,
never when the app crate depends on core as an ordinary dependency, and a
downstream crate cannot switch on an upstream crate's `cfg(test)`. Emitting
a second file into core was the other option, but this engine's input only
ever answers the app crate, so it cannot place a file there.

vectors-rust instead calls `mockall::mock!` directly inside the emitted app
test file, one block per controller referenced by any vector, and
implements the trait through the trait's full path
(`songe_hello_core::controller::greeting_controller::GreetingController`)
rather than importing it by name. This is why: the mock struct itself is
named after the trait, `GreetingController`, so that `mockall::mock!`
answers `MockGreetingController` matching `hexagonal-rust`'s own naming.
Importing the trait under its own name into the same file would collide
with that struct. This choice compiles standalone, with no change to core
required, and was verified by a `cargo test --workspace` run against the
real `hexagonal-rust` output.

A driver taking several controllers needs a mock for each, since
`HttpDriver::new` takes one argument per controller regardless of which one
a given test exercises. Every generated test builds the whole driver,
arming the controller under test and leaving the others as fresh, unarmed
mocks.

## What the crate needs

`app`'s `[dev-dependencies]` need `mockall`, `tower` and `http-body-util`
in addition to what `hexagonal-rust` already put under `[dependencies]`
(`axum`, `serde_json`, `tokio` with the `macros` and `rt` features already
included by its `full` feature).

```toml
[dev-dependencies]
mockall = "0.15"
tower = "0.5"
http-body-util = "0.1"
```

`tower` supplies `ServiceExt::oneshot`, `http-body-util` supplies
`BodyExt::collect` to read the response body out of `axum::body::Body`.
Neither is in the workspace's root `Cargo.toml` dependency table today,
only `mockall` is. The factory needs to add `tower` and `http-body-util`
there before a real workspace can build these tests.

## Files and names

The one file this engine emits is named `zz_generated_vectors.rs` and
starts with the generated header. It is always answered in full, never
`WriteOnce`, since a test file has no hand-written half to protect.
