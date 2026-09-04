# hexagonal-rust

A forge-dev generator that turns one OpenAPI document into the rust
skeleton of one service. Two crates come out. `core` holds types, ports,
controllers and the hand stubs. `app` holds the sqlite adapters, the axum
driver and main.

forge-dev never writes outside the engine directory. So each crate holds
its own cell at its root and names its side.

```yaml
name: songe-hello
kind: hexagonal
language: rust
generator: forge://github.com/alexandremahdhaoui/forge-dev-codegen/cmd/hexagonal-rust
openapi:
  specPath: ./.forge/spec-cache/hello.v1.yaml
surface:
  side: core
  coreDir: .
  cells: [grpc]
```

The app crate says `side: app` and `appDir: .`.

The `generate` tool takes the normalized forge-dev model. `name` is the
service. `openapiSpec` is the document. `coreDir` and `appDir` are the
crate roots relative to the engine directory and default to `core` and
`app`. `side` is `core`, `app` or absent for both. The three may sit at
the top level of the model or under `surface`.

## Cells

`surface.cells` lists the module directories under `src` that another
generator fills. `lib.rs` gains one plain `pub mod <cell>;` line per
name. No `#[path]` attribute. The cell owns its directory and writes its
own `mod.rs`.

A name Rust cannot spell as a module is refused. So is a name the
skeleton already owns, and a name listed twice.

## Extra hand modules

`surface.hand` lists module names under `src/hand` that the author writes
and the spec does not declare.

```yaml
surface:
  side: core
  coreDir: .
  hand: [echo_controller, datagram]
```

`src/hand/mod.rs` gains one plain `pub mod <name>;` line each, beside the
controller stubs. The modules are siblings, so one reaches another with
`crate::hand::<name>`.

A name Rust cannot spell as a module is refused. So is `mod`, a name
listed twice, and a controller the spec already declares.

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

Every file except a hand stub and a `mod.rs` is named `zz_generated_*`.
Every generated file starts with the generated header. The root cell owns
one real `mod.rs` per layer directory, so `lib.rs` is plain
`pub mod <layer>;` lines and no file carries a `#[path]` attribute. A
layer `mod.rs` names the generated file and aliases it, so
`crate::types::greeting` reads the same as before.

## What the crates need

The factory owns `Cargo.toml`. `core` needs `serde` with `derive`,
`serde_json` and `thiserror`, plus `mockall` under dev. `app` needs the
core crate, `anyhow`, `axum`, `rusqlite` with `bundled`, `serde`,
`serde_json`, `thiserror` and `tokio` with `full`.
