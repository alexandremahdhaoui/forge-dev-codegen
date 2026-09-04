# grpc-rust-tonic

A forge-dev generator that turns one proto3 file into the rust skeleton
of every gRPC service it declares. `core` holds the plain message types
and the client port trait. `app` holds the client adapter, the server
driver, and the files tonic-build needs to compile the proto.

The parser is small and on purpose. It reads `package`, `message` with
scalar and message fields, and `service` with unary rpcs. It refuses
imports, options, enums, extend, streaming, nested messages, oneofs,
maps, repeated fields and qualified type references, with a clear error
naming what broke.

```yaml
name: songe-hello
kind: grpc
language: rust
generator: forge://github.com/alexandremahdhaoui/forge-dev-codegen/cmd/grpc-rust-tonic
proto:
  specPath: ./.forge/spec-cache/hello.v1.proto
surface:
  coreDir: ../songe-hello-core
  appDir: ../songe-hello-app
```

The `generate` tool takes the normalized forge-dev model. `name` is the
service. `protoSpec` is the proto3 document. `coreDir` and `appDir` are
the crate roots relative to the engine directory and default to `core`
and `app`. Both may sit at the top level of the model or under
`surface`.

## What the proto decides

For every `service` in the file:

| Emitted | Holds |
|---|---|
| `core/src/types/zz_generated_<service>_messages.rs` | one plain serde struct per message reachable from the service's rpcs |
| `core/src/port/zz_generated_<service>_client.rs` | trait `<Service>Client`, one method per rpc, mockable under test, and its error enum `<Service>ClientError` |
| `app/src/adapter/zz_generated_<service>_grpc_client.rs` | `<Service>GrpcClient`, a tonic channel behind the port trait |
| `app/src/driver/zz_generated_<service>_grpc_driver.rs` | `<Service>GrpcDriver`, a tonic server forwarding each rpc to `Arc<dyn <Service>Controller>` from core |
| `app/zz_generated_build.rs` | the build script tonic-build needs |
| `app/proto/<name>.proto` | the proto file, copied verbatim |

`<service>` is the snake case of the proto service name. `<Service>` is
its Pascal case. The crates are named `<name>-core` and `<name>-app`
from the model's `name`, matching hexagonal-rust, so the two engines
can fill the same pair of crates.

## The controller is not generated here

The driver expects `core::controller::<service>_controller::{<Service>Controller, <Service>ControllerError}`,
one method per rpc, `fn <rpc>(&self, request: <Request>) -> Result<<Response>, <Service>ControllerError>`.
This engine does not emit that trait. Either hexagonal-rust's
`x-controller` fills it for the same service name, or it is hand
written once in core, the same way a hexagonal-rust hand stub is
written once and never regenerated.

## Where the prost types live

`app/zz_generated_build.rs` compiles the proto with `protox`, a pure
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

Consumer's own `lib.rs` mounts the emitted files with `#[path]`, the
same way hexagonal-rust's generated `mod` files do, since this engine
never writes outside `types/`, `port/`, `adapter/` and `driver/`:

```rust
// core/src/lib.rs
pub mod types {
    #[path = "types/zz_generated_<service>_messages.rs"]
    pub mod zz_generated_hello_messages;
}
pub mod port {
    #[path = "port/zz_generated_<service>_client.rs"]
    pub mod zz_generated_hello_client;
}
```

```rust
// app/src/lib.rs
pub mod adapter {
    #[path = "adapter/zz_generated_<service>_grpc_client.rs"]
    pub mod zz_generated_hello_grpc_client;
}
pub mod driver {
    #[path = "driver/zz_generated_<service>_grpc_driver.rs"]
    pub mod zz_generated_hello_grpc_driver;
}
```

and `app/build.rs` includes the generated one:

```rust
include!("zz_generated_build.rs");
```
