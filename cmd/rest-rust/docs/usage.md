# rest-rust

A forge-dev generator that turns one OpenAPI document into the rust rest
cell of a service. It fills one cell, a module directory under `src`,
holding the five layers a service crate uses: types, port, controller,
adapter and driver.

The document decides everything. A schema becomes a type. A schema
marked `x-store` also becomes a store port and a sqlite adapter. An
operation names its controller with `x-controller` and the ports that
controller consumes with `x-ports`. An operation marked `x-auth: bearer`
is guarded by a ticket verifier. A GET operation marked
`x-stream: events` answers an event stream. A parameter declared
`in: path` or `in: query` becomes an argument of the controller method
and of the client method. The paths become the axum router.

This file sits inside the cell, at `src/rest/forge-dev.yaml`. The build
step that runs it points `src` at the cell.

```yaml
name: songe-hello
kind: rest
language: rust
generator: forge://github.com/alexandremahdhaoui/forge-dev-codegen/cmd/rest-rust
openapi:
  specPath: ../../.forge/spec-cache/hello.v1.yaml
layout:
  cell: rest
  side: server
```

The `generate` tool takes the normalized forge-dev model. `name` is the
service. `openapiSpec` is the document. `layout.cell` names the module
directory and defaults to `rest`. `layout.side` is `server`, `client` or
`both` and defaults to `server`. The server side is the driver, the
controllers and the ports they consume. The client side is one client
port per controller and a reqwest adapter for each.

Every emitted path is relative to the cell directory. The engine never
writes above it.

## What the document decides

| Emitted, under the cell | Side | Holds |
|---|---|---|
| `types/zz_generated_<schema>.rs` | both | one serde struct per component schema |
| `port/zz_generated_<store>_store.rs` | server | trait `<Store>Store` with put and get, plus its error enum |
| `port/zz_generated_<event>_subscribe.rs` | server, with `x-stream` | trait `<Event>Subscribe` with `subscribe`, answering a `std::sync::mpsc::Receiver<Event>` for a key |
| `adapter/zz_generated_<store>_sqlite.rs` | server | `<Store>SqliteStore`, `new` taking `<Store>SqliteStoreConfig`, the table and the audit table |
| `controller/zz_generated_<name>_controller.rs` | server | trait `<Name>Controller`, its error enum, and `<Name>ControllerImpl` holding one boxed port per `x-ports` entry plus the subscribe port of each stream |
| `port/zz_generated_<port>.rs` | server | one port per declaration entry of `x-ports`, its error enum and its trait under mockall |
| `driver/zz_generated_wire.rs` | server | one wire struct per schema, `RejectionWire`, and the mapping both ways |
| `driver/zz_generated_http_driver.rs` | server | `HttpDriver`, `HttpDriverConfig`, the router, and one handler per operation |
| `adapter/zz_generated_wire.rs` | client | the same wire structs, so the adapter never reaches into the driver |
| `port/zz_generated_<name>_client.rs` | client | trait `<Name>Client` with one method per operation of that controller, and `<Name>ClientError` in the wire taxonomy |
| `adapter/zz_generated_<name>_rest_client.rs` | client | `<Name>RestClient`, `new` taking `<Name>RestClientConfig` and, under `x-auth`, a boxed `TokenSource` |
| `zz_generated_cell.yaml` | both | the cell manifest hexagonal-rust reads |

`Subject`, `TicketVerifier` and `TokenSource` belong to the crate root.
hexagonal-rust writes `src/types/zz_generated_subject.rs`,
`src/port/zz_generated_ticket_verifier.rs` and
`src/port/zz_generated_token_source.rs` once per crate whenever a cell
manifest requires them, and every rest cell references
`crate::types::subject::Subject`, `crate::port::ticket_verifier` and
`crate::port::token_source`. Three client cells in one crate share one
token source and one verifier.

## Auth

`x-auth: bearer` on an operation makes its handler read the
`Authorization: Bearer <token>` header, call `TicketVerifier::verify`
with the token, and hand the `Subject` it answers to the controller as
the first argument after `self`. A missing header answers 401 with the
type `authentication` and never calls the verifier. A `Refused` answer
is 401 with the refusal message. A `Verify` failure is 500 with a
generic body. The controller never runs on a refusal. An operation
without `x-auth` changes nothing.

The `TicketVerifier` port is listed under `requires.ports` and under the
driver's `ports`, so `wiring.yaml` names its adapter the way a store
port is named. The driver's `new` takes it after the controllers.

## Parameters

A parameter is a string or an integer. Anything else is refused by name.

`in: path` keeps the shape it always had. Every `{name}` of the path is
declared, the handler extracts it, and the controller takes it in path
order.

`in: query` follows the path parameters, in declaration order, on the
controller method and on the client method. `required: true` gives a
`&str` or an `i64`. Anything else gives an `Option<String>` or an
`Option<i64>`. A name declared in both places is refused, and a query
parameter declared twice is refused.

The handler reads the query as a map of strings, so the driver owns
every refusal instead of axum. A missing required parameter and an
integer that will not parse both answer the operation's invalid status,
422 first then 400, with the type `validation` and a message naming the
parameter. The controller never runs.

The client builds the query into the url itself, percent encoding every
name and value, and leaves out an optional parameter that is `None`. It
needs no reqwest feature beyond `json`.

```yaml
parameters:
  - name: after
    in: query
    schema:
      type: string
```

## Declared ports

`x-ports` takes a port name or a declaration. A name is the store port
of an `x-store` schema, the subscribe port of the operation's own
stream, or a port some operation declares. A declaration is an object
naming a `kind`. The declared kinds are `clock`.

```yaml
x-ports:
  - GreetingStore
  - kind: clock
    name: GreetingClock
    instant: Instant
    span: Span
    adapters: [memory, system]
```

The name is Pascal case and may not take the name of a store or
subscribe port. A kind says what the port is, so the engine writes both
its trait and every adapter its `adapters` list names. There is no kind
that carries raw Rust signatures and leaves the adapter to the user. A
port the engine cannot write is a port that belongs in its own spec.

A clock adapter is `memory` or `system`. The memory clock starts at the
moment its configuration names and steps one unit per call, so a vector
reads the same stamp on every run. The system clock reads the machine
through the standard library and takes no configuration. A service that
expires anything wires `system`.

A clock names two schemas and each carries one required integer
property. The engine takes the field names from those schemas, so the
declaration owns them and the engine invents nothing. A schema carrying
any other shape is refused by name.

The engine writes `port/zz_generated_<snake>.rs` holding
`<Name>Error` with a `Refused` and a `Call` arm and the trait under
`mockall::automock`, so the user never writes a port trait. The
controller struct gains one boxed field, `new` takes it in port name
order, and the controller error enum gains one arm wrapping
`<Name>Error`. The manifest declares the port and the adapters its kind
emits, so `wiring.yaml` picks one.

Declare a port once. Every other operation names it by its name. A
second declaration that disagrees is refused, and a name no operation
declares is refused.

## Streams

`x-stream: events` on a GET operation makes its 2xx response a
`text/event-stream`. The response schema, declared under
`text/event-stream`, is the event type. The controller method answers
`std::sync::mpsc::Receiver<Event>`, the controller decides who may
subscribe, and the driver forwards every event as one `data:` frame
until the receiver ends or the client goes away. The controller consumes
the `<Event>Subscribe` port, added to its ports whether or not `x-ports`
names it. `x-ports` may name a subscribe port only on the stream that
owns it. A stream operation takes no request body.

The `<Event>Subscribe` port is listed under `requires.ports`, so
`wiring.yaml` names its adapter. The driver streams through an
`http-body-util` channel body, which needs that crate's `channel`
feature. The bridge waits fifteen seconds for an event, then writes an
SSE comment frame as a keep alive, so a client that went away is
noticed and the bridge thread ends.

## The client

Every operation of a controller becomes one method on `<Name>Client`,
taking the path parameters and the body as core types and answering the
core response, a `std::sync::mpsc::Receiver<Event>` for a stream, or
`<Name>ClientError`. The error carries the wire taxonomy. `Runtime`,
`Authentication`, `Authorization`, `Validation`, `Semantic` and
`RateLimiting`, each with the operation and the message the server sent.
A transport failure and an unknown error type are `Runtime`.

`<Name>RestClient` calls through reqwest. Its config carries `base_url`.
When one of its operations carries `x-auth` it consumes the `TokenSource`
port and sends the token it answers as a bearer on those operations. A
`TokenSource` failure is an `Authentication` error. Without `x-auth` its
`new` takes the config alone. The methods are synchronous and block
inside the tokio runtime, so a controller with plain methods can call
them. A stream method spawns a task that reads the `data:` frames into
the receiver it answers and logs a transport failure with its chain
before the receiver ends.

The manifest lists `<Name>Client` under `ports` and the adapter under
`adapters`, named `<cell>_<name>_client`, with `ports: [TokenSource]`
and `TokenSource` under `requires.ports` when it needs one, so
hexagonal-rust builds the token source first and hands it to `new`.

Each layer directory carries a `mod.rs` that mounts its generated files.
The controller layer also carries one `mod <name>_controller;` line per
controller, pointing at the file the user writes. The cell's own
`mod.rs` lists the layers.

## What the user writes

One file per controller, `controller/<name>_controller.rs`, holding
`impl <Name>Controller for <Name>ControllerImpl`. The struct, its fields
and `new` are generated. A missing file, a missing method or a wrong
signature is a compile error naming what broke.

## The driver

`new` takes `HttpDriverConfig` and one boxed controller per trait.
`bind` opens the listener on `config.addr`. `local_port` answers the
port it bound. `announce` prints `LISTENING <port>`. `serve` runs axum
until it stops.

The driver maps a controller error to a status. The taxonomy on the wire
is the one CLAUDE.md section 7 names. A player mistake carries its own
message. A runtime failure carries a generic one and the chain goes to
stderr.

| Controller error | Status | Type on the wire |
|---|---|---|
| a port error | 500 | `runtime`, body `internal error` |
| `Authentication` | 401 | `authentication` |
| `Authorization` | 403 | `authorization` |
| `NotFound` | 404 | `semantic` |
| `Invalid` | the declared invalid status, 422 first then 400 | `validation` |
| `Semantic` | 409 | `semantic` |
| `RateLimited` | 429 | `rateLimiting` |
| `NotImplemented` | 501 | `runtime` |

The table is not written in this engine. `internal/taxonomy` owns the
members, the variant fields, the display string, the wire type, the REST
status, the gRPC status and the rule that only a runtime failure hides its
message. rest-rust renders the controller enum, the driver mapping, the
client enum and the client's reverse mapping from it, grpc-rust-tonic
renders its enum and its status mapping from it, and vectors-rust arms its
mocks from it. A ninth member is one edit in one file.

## The adapters

`<Store>SqliteStore::new` takes `<Store>SqliteStoreConfig`, whose only
field is `path`. It opens the file, creates the table and the audit
table, and answers the port trait. A single store schema names its
adapter `sqlite` in the manifest. Two or more name theirs
`<store>_sqlite`, so a merge across cells never collides.

`<Store>MemoryStore::new` takes a `capacity` and refuses a new row past
it with `Full`, naming the capacity and the key. The sqlite store has no
ceiling and never refuses that way.

The two adapters are meant to differ here. A capacity is a property of
one adapter, never of the port. The port permits the refusal and says
nothing about when it comes. Memory answers it because memory fills.
Sqlite never answers it because sqlite never fills. Neither adapter
ignores a promise the port makes, because the port promises no ceiling.

A lookup names a column through the generated `<Store>Column` enum, so
neither adapter can be asked for a column it cannot read and neither
answers a default when asked.

## What the crate needs

`axum`, `rusqlite` with `bundled`, `serde`, `serde_json`, `thiserror`
and `tokio`, plus `mockall` under dev. A cell with a stream needs
`http-body-util` with the `channel` feature under `[dependencies]`. A
client side needs `reqwest` with `json`.

The crate's own `lib.rs` mounts the cell with one plain line, which
hexagonal-rust writes from `layout.cells`:

```rust
pub mod rest;
```
