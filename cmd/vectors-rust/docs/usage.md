# vectors-rust

A forge-dev generator that turns declared behavior into rust integration
tests. It reads the transports a service declares and a vectors document,
and answers one file, `app/tests/zz_generated_vectors.rs`, holding one
async tokio test per vector case.

## A service declares the transports it carries

Three surfaces, each optional and each read only when the cell declares it.

| Surface | Declared by | Case prefix |
|---|---|---|
| REST | `openapi.specPath` | the `operationId` |
| UDP | `proto.specPath` | `udp_` |
| gRPC | `layout.grpcProto` | `grpc_` |

An absent OpenAPI document means the service carries no REST, and the
emitted file holds no axum import. The same holds for an absent datagram
proto and an absent grpc proto. A cell declaring none of the three is
refused by name, because a vectors cell with nothing to drive is a
mistake.

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
| `expectedError` | Names the taxonomy member the controller answers, or `Unauthenticated` for the driver's own bearer refusal. Required where `expectedStatus` alone is ambiguous, optional elsewhere, and checked against `expectedStatus` when both are present. |
| `expectedErrorSubstring` | Present on an error case. The controller mock is armed to fail, and the response body's `message` field must contain this text. |

A case needs `controllerReply` or `expectedErrorSubstring`, never neither. A
case carrying `controllerReply` and `expectedError` is refused.

## Guarded cases

An operation marked `x-auth: bearer` takes two more fields.

| Field | Meaning |
|---|---|
| `bearer` | The token sent as `Authorization: Bearer <bearer>`. Absent means no header, and the driver answers 401 before the verifier runs. |
| `subject` | The subject id the mocked `TicketVerifier` answers for `bearer`. Absent with a `bearer` means the verifier refuses it. |

A success case on a guarded operation needs both. The mocked controller
then expects `Subject { id: subject }` as its first argument. Both fields
are refused on an operation without `x-auth`.

A 401 on a guarded operation has two meanings, so the case says which in
`expectedError`.

| `expectedError` | What the test drives |
|---|---|
| `Unauthenticated` | The mocked verifier refuses the bearer. Its refusal embeds `expectedErrorSubstring`. The controller is armed `never`. |
| `Authentication` | The verifier answers `subject` and the mocked controller answers `Authentication`. |

A 401 on a guarded operation without `expectedError` is refused and names
both choices. `Unauthenticated` on an operation without `x-auth` is refused
too.

```json
{
  "case": "count_with_a_valid_ticket_adds_one",
  "operation": "countGreeting",
  "input": { "id": "g1" },
  "bearer": "open",
  "subject": "friend",
  "controllerReply": { "id": "g1", "name": "Songe", "count": 1 },
  "expectedStatus": 200,
  "expectedBody": { "id": "g1", "name": "Songe", "count": 1 }
}
```

## Stream cases

An operation marked `x-stream: events` answers `text/event-stream`.
`controllerReply` is one event. The mocked controller answers a receiver
holding that one event and closes it. The test asserts the status, the
content type, and matches `expectedBody` against the first `data:` frame
of the body as JSON. The test runs on a multi thread runtime because the
driver bridges the receiver on a blocking thread.

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
back.

A case spells a field the way the proto spells it. `character_id`, never
`characterId`. A key no message declares is refused by name, and the
refusal lists the fields the message does declare. `sessionId` is the one
exception, because the frame carries it in both directions and no message
holds it.

A field holding a message reads a nested object. It becomes `Some(...)`.
An absent one becomes `None`, which is what prost emits for an unset
message field. A field holding `bytes` is refused by name. The proto
parser refuses a message that reaches itself before any vector reads it.

The engine folds every method name the way udp-rust does and refuses two
rpcs that fold to one byte.

An operation the engine cannot map is refused by name, and the refusal
lists the surfaces the cell declares. There is no skip.

An error case names its taxonomy member in `expectedError`, or lets
`expectedStatus` name it where the status is unambiguous.

| `expectedStatus` | Taxonomy member | Field carrying the substring |
|---|---|---|
| 401 on an `x-auth` operation | ambiguous, name `Unauthenticated` or `Authentication` | the refusal, or `subject` |
| 401 on any other operation | `Authentication` | `subject` |
| 403 | `Authorization` | `subject` |
| 404 | `NotFound` | `id` |
| the declared invalid status, 422 or 400 | `Invalid` | `field` |
| 409 | `Semantic` | `resource` |
| 429 | `RateLimited` | `subject` |
| 501 | `NotImplemented` | `operation` |

The table is not written here. It is read from `internal/taxonomy`, the one
package that owns the members, the wire type string, the REST status, the
gRPC status and the rule that a runtime failure answers a generic message.
rest-rust, grpc-rust-tonic and vectors-rust all read it, so a ninth member
is one edit.

Anything else is refused by name. Because the engine has no business
knowledge beyond the spec, it embeds `expectedErrorSubstring` itself into
the chosen member's first field, so the constructed message is guaranteed
to contain it wherever the driver's rejection text includes that field. It
cannot cover the `500` reply, whose body is a fixed "internal error" text
set by the driver, not by the controller error's message.

## Query parameters

A query parameter of the operation reads its value from `input` by name.
The test sends it on the uri, percent encoded, and the mocked controller
asserts it in declaration order after the path parameters. A required
parameter is asserted as `&str` or `i64`. An optional one is asserted as
`Some(...)` when `input` names it and `None` when it does not.

Leaving a required query parameter out of `input` is how a case pins the
driver's own refusal. The controller is armed `never`, the uri carries no
value for it, and `expectedStatus` must be the operation's invalid status.
A success case that leaves one out is refused, and so is an error case
naming any other status.

## Session cases

When the udp cell names `layout.hello`, the vectors cell names the same
`hello` and `push` under its own `layout`. Every datagram test then
stands the generated driver up over a mocked controller, a mocked
`<Service>SessionGate` and the real `<Service>UdpPeerTable`. A push case
also builds the real `<Service>UdpBroadcast` over that table for the
mocked controller's `on_tick`.

```yaml
layout:
  cell: udp
  hello: Hello
  push: [Counter]
```

A datagram case gains these fields. Every one is optional.

| Field | Meaning |
|---|---|
| `gate` | `admit` or `refuse`. What the mocked gate answers. Defaults to `admit`. |
| `session` | `registered` or `unknown`. Whether the harness sends a hello for the case's `sessionId` before the operation. Defaults to `registered`. A hello case never needs one. |
| `hello` | The hello request the harness registers with. Missing fields default. |
| `reconnect` | On the hello rpc only. The hello is sent twice from two sockets. The reply of the second must match `expectedBody` and the peer table must point at the second socket. |
| `expectDropped` | The client must get no reply within a short timeout. Drop `controllerReply` and `expectedBody`. Pair it with `gate: refuse` for a refused hello or `session: unknown` for a datagram the peer table never admitted. |
| `expectPush` | The case sends no request. The tick driver runs and the server pushes. It names the push `rpc`, the registered `sessionIds` the push must reach and a `payload` the received push must equal. |

```json
{
  "case": "udp_counter_reaches_two_sessions_on_a_tick",
  "operation": "udp_counter",
  "input": null,
  "hello": { "secret": "open" },
  "expectPush": {
    "rpc": "Counter",
    "sessionIds": ["0123456789abcdef", "fedcba9876543210"],
    "payload": { "tick": 3 }
  }
}
```

A push case registers every session id with a hello first, binds a
`<Service>TickDriver` at a 10 millisecond interval over the mocked
controller, and asserts each peer receives the decoded push. The mocked
controller's `on_tick` sends the push through the real peer table. An
operation naming a push rpc carries `expectPush` and no `input`. Every
other datagram case carries `input` and a 16 byte `sessionId`.

## Request driven push cases

A push may answer a request instead of a tick. Name the inbound rpc as
`operation`, carry its `input`, and name the server to client kind under
`expectPush.rpc`.

```json
{
  "case": "udp_an_echo_answers_the_caller_and_pushes_the_counter",
  "operation": "udp_echo",
  "input": { "sessionId": "0123456789abcdef", "payload": "songe" },
  "hello": { "secret": "open" },
  "controllerReply": { "payload": "songe" },
  "expectPush": {
    "rpc": "Counter",
    "sessionIds": ["0123456789abcdef", "fedcba9876543210"],
    "payload": { "tick": 9 }
  }
}
```

`controllerReply` is what the mocked controller answers the caller.
`expectPush` is what the same mock sends through the real
`<Service>Broadcast` while it answers. The test registers every session in
`sessionIds` with a hello, sends the inbound datagram from the asking
session's own socket, then reads the push on every socket and the reply on
the asking one. The asking `sessionId` may sit in `sessionIds` or not. An
rpc answering `Nothing` carries no `controllerReply` and the test asserts
the driver sent no reply. The hello rpc is refused here because it opens
the session.

## Seeded cases

A push case may carry `seed`, the number a mocked rng port answers, so a
roll is deterministic. The vectors cell names that port under `layout.rng`.

```yaml
layout:
  cell: udp
  hello: Hello
  push: [Counter]
  rng:
    trait: TickCounter
    module: udp::port::tick_counter
    method: next
    returns: u64
```

```json
{
  "case": "udp_a_tick_pushes_the_count_the_seeded_rng_port_answers",
  "operation": "udp_counter",
  "input": null,
  "hello": { "secret": "open" },
  "seed": 7,
  "expectPush": {
    "rpc": "Counter",
    "sessionIds": ["0123456789abcdef"],
    "payload": { "tick": "<seed>" }
  }
}
```

The engine emits a `mockall::mock!` for the port and a
`seeded_<trait>(seed)` builder armed to answer the seed. The mocked
controller draws from it while it answers and puts the draw where the
payload reads `<seed>`. The assertion pins the received push to the seed
value, so the draw has to reach the wire.

A seed with no `layout.rng`, a seed on a case that never pushes, a seed no
payload field reads, and a `<seed>` with no seed are each refused by name.

## gRPC cases

A cell that names a `grpcProto` reads the grpc service block. A case whose
`operation` is `grpc_<Rpc>` becomes a test that binds the generated
`<Service>GrpcDriver` on a free port over a mocked `<Service>Controller`
and calls it through the generated `<Service>GrpcClient`.

```yaml
layout:
  crateDir: .
  grpcCell: grpc
  grpcProto: ../.forge/spec-cache/hello.v1.proto
```

```json
{
  "case": "grpc_ping_answers_the_count_raised_by_one",
  "operation": "grpc_Ping",
  "input": { "message": "songe", "count": 7 },
  "controllerReply": { "message": "songe", "count": 8 }
}
```

`operation` spells the rpc as the proto does, after `grpc_`.
`controllerReply` is what the mocked controller answers and what the client
must read back.

A refusal case carries `expectedError` instead, naming the taxonomy member
the controller answers. The mock answers that member and the test asserts
the `tonic::Code` the taxonomy maps it to, walking the client error chain
for the `tonic::Status`. Remap a member to a different code and the test
fails.

```json
{
  "case": "grpc_ping_with_an_empty_message_is_refused",
  "operation": "grpc_Ping",
  "input": { "message": "", "count": 0 },
  "expectedError": "Invalid",
  "expectedErrorSubstring": "message"
}
```

| `expectedError` | Asserted code |
|---|---|
| `Authentication` | `Unauthenticated` |
| `Authorization` | `PermissionDenied` |
| `NotFound` | `NotFound` |
| `Invalid` | `InvalidArgument` |
| `Semantic` | `FailedPrecondition` |
| `RateLimited` | `ResourceExhausted` |
| `NotImplemented` | `Unimplemented` |

`expectedErrorSubstring` stays optional beside it and pins the text the
status carries. `Runtime` is refused, because the driver hides a runtime
failure behind a generic message and no substring can pin it. A member the
taxonomy does not declare is refused with the list. A grpc case carries no
`expectedStatus` and no `expectedBody`, both are refused. The test runs on a
multi thread runtime because the generated client blocks inside tokio.

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
