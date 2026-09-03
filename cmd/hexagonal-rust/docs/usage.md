# hexagonal-rust

A forge-dev generator that turns one OpenAPI document into the rust
skeleton of one service. Two crates come out. `core` holds types, ports,
controllers and the hand stubs. `app` holds the sqlite adapters, the axum
driver and main.

```yaml
name: songe-hello
kind: hexagonal
language: rust
generator: forge://github.com/alexandremahdhaoui/forge-dev-codegen/cmd/hexagonal-rust
surface:
  coreDir: ../songe-hello-core
  appDir: ../songe-hello-app
```

The `generate` tool takes the normalized forge-dev model. `name` is the
service. `openapiSpec` is the document. `coreDir` and `appDir` are the
crate roots relative to the engine directory and default to `core` and
`app`. They may sit at the top level of the model or under `surface`.

## What the spec decides

| Spec | Emitted |
|---|---|
| `components.schemas.<Name>` | `core/src/types/zz_generated_<snake>.rs`, a serde struct |
| a schema with `x-store: true` | `core/src/port/zz_generated_<snake>_store.rs`, trait `<Name>Store` with `put` and `get`, mockable under test |
| | `app/src/adapter/zz_generated_<snake>_sqlite.rs`, `<Name>SqliteStore` over rusqlite with an audit table |
| an operation with `x-controller: <name>` | `core/src/controller/zz_generated_<name>_controller.rs`, trait `<Pascal>Controller` and `<Pascal>ControllerImpl` generic over the `x-ports` |
| | `core/src/hand/<name>_controller.rs`, one function per operation, written once and never again |
| `paths` | `app/src/driver/zz_generated_http_driver.rs`, an axum router with wire types mapped at the edge |
| | `app/src/bin/<name>-server.rs`, main |

Every `x-ports` entry names `<Name>Store` of an `x-store` schema. An
`x-store` schema needs a required string property `id`. Request bodies and
2xx responses `$ref` a component schema. Path parameters are strings or
integers.

Main reads `SONGE_STORE_<NAME>_PATH` for each store and defaults to
`:memory:`. It binds `<NAME>_ADDR` with the default `127.0.0.1:0` and
prints `LISTENING <port>` once bound.

## Files and names

Every file except a hand stub is named `zz_generated_*` and starts with the
generated header. `core/src/lib.rs`, `app/src/lib.rs` and the server
binary keep the names cargo needs. Modules are mounted with `#[path]` so
no file needs to be named `mod.rs`.

## What the crates need

The factory owns `Cargo.toml`. `core` needs `serde` with `derive`,
`serde_json` and `thiserror`, plus `mockall` under dev. `app` needs the
core crate, `anyhow`, `axum`, `rusqlite` with `bundled`, `serde`,
`serde_json`, `thiserror` and `tokio` with `full`.
