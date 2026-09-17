# grpc-rust-tonic

A forge-dev generator that turns one proto3 file into the rust skeleton
of every gRPC service it declares. It fills one cell, a module directory
under `src`. The cell of the core crate holds the plain message types,
the client port trait and the controller. The cell of the app crate
holds the client adapter, the server driver, and the files tonic-build
needs to compile the proto.

The parser is small and on purpose. It reads `package`, `message` with
scalar and message fields, `repeated` or not, and `service` with unary
rpcs. It refuses imports, options, enums, extend, streaming, nested
messages, oneofs, maps and qualified type references, with a clear error
naming what broke.

It also refuses a message that reaches itself through message fields,
`message cycle A -> B -> A, a message cannot reach itself through
message fields`, and it refuses it whether or not the field is repeated.
A repeated self reference would be a `Vec<Node>`, which is sized and
would compile, so this refusal is a choice rather than a limit. No spec
in the workspace declares that shape, and widening the generator with no
consumer builds for a caller who does not exist. The day a spec declares
one, lift the refusal for the repeated case alone.

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
  cell: grpc
  side: both
```

The `generate` tool takes the normalized forge-dev model. `name` is the
service and names the crates `<name>-core` and `<name>-app`. `protoSpec`
is the proto3 document. `layout.cell` names the module directory and
defaults to `grpc`. `layout.side` is `server`, `client` or `both` and
defaults to `both`. The server side is the tonic driver and the
controller trait. The client side is the client port and the tonic
adapter. Both sides carry the message types and the build script. A
side outside the three words is refused by name.

Every emitted path is relative to the cell directory. The engine never
writes above it.

## Calling another service

A service calls another service through the generated client of that
service's proto. The cell is named after the foreign service and holds
the client side only.

```yaml
name: songe-social
kind: grpc
language: rust
generator: forge://github.com/alexandremahdhaoui/forge-dev-codegen/cmd/grpc-rust-tonic
proto:
  specPath: ../../.forge/spec-cache/identity.v1.proto
layout:
  cell: identity_client
  side: client
```

The manifest provides the port `<Service>Client` and the adapter
`<cell>_client`, and no driver and no controller. The adapter's
`endpoint` key carries the cell name, so two client cells over two
protos get two endpoint keys. A controller in another cell names the
port in its own declaration, `x-ports` for a rest cell or
`layout.ports` for a grpc server cell, and hexagonal-rust wires the
adapter. `layout.ports` on a client cell is refused by name, because a
client cell holds no controller to consume a port.

## Declared ports

`layout.ports` names the ports this cell's controller consumes. A proto
carries no place to hang a declaration, so it lives here, and the
entries read the same way `x-ports` reads in an OpenAPI document.

```yaml
layout:
  cell: grpc
  ports:
    - TokenStore
    - GreetingClock
```

Every entry is the name of a port **another cell provides**.
grpc-rust-tonic writes no port of its own, so an entry declaring a
`kind` or `adapters` is refused by name.

The controller struct gains one boxed field per port, `new` takes them
in declaration order, and the file imports each trait from
`crate::port::<snake>`, the one path the crate root re-exports every
port to. The manifest names the ports on the controller and lists them
under `requires.ports`, so the skeleton refuses when no cell provides
one and `wiring.yaml` picks the adapter.

A cell naming no port still gets a unit struct that derives `Default`.
A struct holding ports carries `#[allow(dead_code)]`, so a body that has
not reached one of its ports yet still builds. The compiler still
refuses a missing method, a wrong signature or a type that does not line
up.

## What the proto decides

For every `service` in the file:

| Emitted, under the cell | Side | Holds |
|---|---|---|
| `types/zz_generated_<service>_messages.rs` | both | one plain serde struct per message reachable from the service's rpcs |
| `port/zz_generated_<service>_client.rs` | client | trait `<Service>Client`, one method per rpc, mockable under test, and its error enum `<Service>ClientError` |
| `adapter/zz_generated_<service>_grpc_client.rs` | client | `<Service>GrpcClient`, a tonic channel behind the port trait |
| `controller/zz_generated_<service>_controller.rs` | server | trait `<Service>Controller`, its error enum and `<Service>ControllerImpl` holding one boxed port per `layout.ports` entry |
| `controller/<service>_controller.rs` | server | the impl block, written once and never again |
| `driver/zz_generated_<service>_grpc_driver.rs` | server | `<Service>GrpcDriver`, a tonic server forwarding each rpc to `Arc<dyn <Service>Controller>` |
| `zz_generated_build.rs` | both | the build script tonic-build needs, compiling the client half, the server half or both |
| `proto/zz_generated_<service>.proto` | both | the proto file, copied verbatim |
| `zz_generated_cell.yaml` | both | the cell manifest hexagonal-rust reads |

Each layer directory carries a `mod.rs` that mounts its generated file
and aliases it under the logical name, so a reader writes
`<core_crate>::grpc::controller::hello_controller::HelloController` and
never a `#[path]` attribute. The cell's own `mod.rs` lists the layers.

`<service>` is the snake case of the proto service name. `<Service>` is
its Pascal case. The crates are named `<name>-core` and `<name>-app`
from the model's `name`, matching hexagonal-rust, so the two engines
can fill the same pair of crates.

## The error taxonomy on the wire

`<Service>ControllerError` carries the taxonomy CLAUDE.md section 7
names, the same one the rest cell uses, so a player mistake and a
runtime failure never look alike over gRPC. The driver maps each arm to
its gRPC status code.

| Controller error | gRPC status | Why |
|---|---|---|
| `Runtime` | `internal` | the caller did nothing wrong and learns nothing, the chain goes to stderr |
| `Authentication` | `unauthenticated` | the request carries no valid credential |
| `Authorization` | `permission_denied` | the caller is known and may not do this |
| `NotFound` | `not_found` | the named entity does not exist |
| `Invalid` | `invalid_argument` | the request itself is wrong, whatever the state |
| `Semantic` | `failed_precondition` | the request is well formed and the state refuses it |
| `RateLimited` | `resource_exhausted` | a quota ran out |
| `NotImplemented` | `unimplemented` | the rpc is not served |

Only `Runtime` hides its message. Every other arm sends
`error.to_string()` as the status message, so the caller reads what it
did wrong.

The table is not written in this engine. `internal/taxonomy` owns the
members, the variant fields, the display string, the wire type, the REST
status, the gRPC status and the generic rule. rest-rust, grpc-rust-tonic
and vectors-rust all render from it, so this cell and the rest cell cannot
drift. A vectors case names a member in `expectedError` and the generated
test asserts the `tonic::Code` this table maps it to, so remapping an arm
fails a vector.

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

The cell build script is one function named after the cell,
`pub fn build_<cell>() -> Result<(), Box<dyn std::error::Error>>`, and
holds no `fn main`. The cell manifest declares `buildScript`.
hexagonal-rust reads every cell manifest, includes each cell build
script and writes the one `fn main` at the crate root
`zz_generated_build.rs`, calling every cell build function in cell name
order and failing on the first error with the cell named. Any number of
grpc cells fit in one crate. Nobody writes a `build.rs` by hand.

```rust
include!("src/authz_client/zz_generated_build.rs");
include!("src/grpc/zz_generated_build.rs");

fn main() -> Result<(), Box<dyn std::error::Error>> {
    build_authz_client().map_err(|source| format!("building cell authz_client: {source}"))?;
    build_grpc().map_err(|source| format!("building cell grpc: {source}"))?;

    Ok(())
}
```
