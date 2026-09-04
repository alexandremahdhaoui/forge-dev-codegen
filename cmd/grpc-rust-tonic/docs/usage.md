# grpc-rust-tonic

A forge-dev generator that turns one proto3 file into the rust skeleton
of every gRPC service it declares. It fills one cell, a module directory
under `src`. The cell of the core crate holds the plain message types,
the client port trait and the controller. The cell of the app crate
holds the client adapter, the server driver, and the files tonic-build
needs to compile the proto.

The parser is small and on purpose. It reads `package`, `message` with
scalar and message fields, and `service` with unary rpcs. It refuses
imports, options, enums, extend, streaming, nested messages, oneofs,
maps, repeated fields and qualified type references, with a clear error
naming what broke.

This file sits inside the cell, at `src/grpc/forge-dev.yaml`. The build
step that runs it points `src` at the cell.

```yaml
name: songe-hello
kind: grpc
language: rust
generator: forge://github.com/alexandremahdhaoui/forge-dev-codegen/cmd/grpc-rust-tonic
proto:
  specPath: ../../.forge/spec-cache/hello.v1.proto
layout:
  side: core
```

The `generate` tool takes the normalized forge-dev model. `name` is the
service and names the crates `<name>-core` and `<name>-app`. `protoSpec`
is the proto3 document. `layout.side` picks the half of the skeleton
this cell holds, `core` or `app`. `layout.cell` names the module
directory and defaults to `grpc`.

Every emitted path is relative to the cell directory. The engine never
writes above it.

## What the proto decides

For every `service` in the file:

| Emitted, under the cell | Holds |
|---|---|
| `types/zz_generated_<service>_messages.rs` | one plain serde struct per message reachable from the service's rpcs |
| `port/zz_generated_<service>_client.rs` | trait `<Service>Client`, one method per rpc, mockable under test, and its error enum `<Service>ClientError` |
| `controller/zz_generated_<service>_controller.rs` | trait `<Service>Controller` and the impl that calls the hand body |
| `hand/<service>_controller.rs` | the body, written once and never again |
| `adapter/zz_generated_<service>_grpc_client.rs` | `<Service>GrpcClient`, a tonic channel behind the port trait |
| `driver/zz_generated_<service>_grpc_driver.rs` | `<Service>GrpcDriver`, a tonic server forwarding each rpc to `Arc<dyn <Service>Controller>` from core |
| `zz_generated_build.rs` | the build script tonic-build needs |
| `proto/zz_generated_<service>.proto` | the proto file, copied verbatim |

Each layer directory carries a `mod.rs` that mounts its generated file
and aliases it under the logical name, so a reader writes
`<core_crate>::grpc::controller::hello_controller::HelloController` and
never a `#[path]` attribute. The cell's own `mod.rs` lists the layers.

`<service>` is the snake case of the proto service name. `<Service>` is
its Pascal case. The crates are named `<name>-core` and `<name>-app`
from the model's `name`, matching hexagonal-rust, so the two engines
can fill the same pair of crates.

## Where the prost types live

`zz_generated_build.rs` compiles the proto with `protox`, a pure
rust protobuf parser, so no `protoc` binary is required. It calls
`tonic_prost_build::configure().compile_fds(...)`, not
`tonic_prost_build::compile_protos`, because `protox::compile` already
read the file.

The adapter and the driver each declare their own private
`mod pb { tonic::include_proto!("<package>"); }` and their own `From`
impls between the plain core types and `pb::*`. Core never depends on
the generated proto code. That dependency belongs to app, the boundary
where wire types are mapped to internal types, per this workspace's
architecture rule. Core only depends on `serde`, `serde_json` and
`thiserror` and stays free of anything that must be compiled from a
`.proto` file.

## The client port trait is synchronous

`<Service>Client` methods return `Result` directly, not a `Future`.
`mockall::automock` mocks a synchronous trait without extra
dependencies. `<Service>GrpcClient` bridges the synchronous trait to
tonic's async client with `tokio::runtime::Handle::current().block_on`,
so it must be called from inside a multi threaded tokio runtime.
`tokio::task::block_in_place` panics on a current thread runtime, so
the caller needs `#[tokio::main(flavor = "multi_thread")]` or plain
`#[tokio::main]`, which already defaults to multi thread. A
hexagonal-rust server main is `#[tokio::main]` with no flavor argument,
so it already satisfies this and needs no change to host a grpc
adapter alongside its axum driver.

## What the crates need

`core` needs `serde` with `derive`, `serde_json` and `thiserror`, plus
`mockall` under dev. `app` needs the core crate, `tonic`, `tonic-prost`,
`prost` and `tokio` with at least `rt-multi-thread`. `app`'s
build-dependencies need `protox` and `tonic-prost-build`.

The consumer's own `lib.rs` mounts the cell with one plain line, which
hexagonal-rust writes from `layout.cells`:

```rust
pub mod grpc;
```

The app crate's `build.rs` sits at the crate root and includes the
generated one:

```rust
include!("src/grpc/zz_generated_build.rs");
```
