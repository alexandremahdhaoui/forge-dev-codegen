# rest-rust

A forge-dev generator that turns one OpenAPI document into the rust rest
cell of a service. It fills one cell, a module directory under `src`,
holding the five layers a service crate uses: types, port, controller,
adapter and driver.

The document decides everything. A schema becomes a type. A schema
marked `x-store` also becomes a store port and a sqlite adapter. An
operation names its controller with `x-controller` and the ports that
controller consumes with `x-ports`. The paths become the axum router.

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
```

The `generate` tool takes the normalized forge-dev model. `name` is the
service. `openapiSpec` is the document. `layout.cell` names the module
directory and defaults to `rest`.

Every emitted path is relative to the cell directory. The engine never
writes above it.

## What the document decides

| Emitted, under the cell | Holds |
|---|---|
| `types/zz_generated_<schema>.rs` | one serde struct per component schema |
| `port/zz_generated_<store>_store.rs` | trait `<Store>Store` with put and get, plus its error enum |
| `adapter/zz_generated_<store>_sqlite.rs` | `<Store>SqliteStore`, `new` taking `<Store>SqliteStoreConfig`, the table and the audit table |
| `controller/zz_generated_<name>_controller.rs` | trait `<Name>Controller`, its error enum, and `<Name>ControllerImpl` holding one boxed port per `x-ports` entry |
| `driver/zz_generated_wire.rs` | one wire struct per schema the paths carry, and the mapping both ways |
| `driver/zz_generated_http_driver.rs` | `HttpDriver`, `HttpDriverConfig`, the router, and one handler per operation |
| `zz_generated_cell.yaml` | the cell manifest hexagonal-rust reads |

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
and `tokio`, plus `mockall` under dev.

The crate's own `lib.rs` mounts the cell with one plain line, which
hexagonal-rust writes from `layout.cells`:

```rust
pub mod rest;
```
