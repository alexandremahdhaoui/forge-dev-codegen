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
`x-stream: events` answers an event stream. The paths become the axum
router.

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

The driver maps a controller error to a status. A port error answers
500 with a generic body. `NotFound` answers 404. `Invalid` answers the
4xx status the operation declares, 422 first, then 400, then 400 by
default. `NotImplemented` answers 501.

## The adapters

`<Store>SqliteStore::new` takes `<Store>SqliteStoreConfig`, whose only
field is `path`. It opens the file, creates the table and the audit
table, and answers the port trait. A single store schema names its
adapter `sqlite` in the manifest. Two or more name theirs
`<store>_sqlite`, so a merge across cells never collides.

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
