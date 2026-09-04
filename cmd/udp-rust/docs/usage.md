# udp-rust

A forge-dev generator that turns one proto3 file into the rust skeleton
of every UDP datagram service it declares. It fills one cell, a module
directory under `src`. The cell of the core crate holds the prost
message types, the codec, the client port trait and the controller. The
cell of the app crate holds the listening driver and the client adapter.

The proto service block is the handler mapping, the same shape gRPC
uses. One rpc is one datagram kind. An rpc whose reply type is named
`Nothing` gets no reply on the wire.

The parser is the one grpc-rust-tonic uses. It reads `package`,
`message` with scalar and message fields, and `service` with unary
rpcs. It refuses imports, options, enums, extend, streaming, nested
messages, oneofs, maps, repeated fields and qualified type references,
with a clear error naming what broke.

This file sits inside the cell, at `src/udp/forge-dev.yaml`. The build
step that runs it points `src` at the cell.

```yaml
name: songe-hello
kind: udp
language: rust
generator: forge://github.com/alexandremahdhaoui/forge-dev-codegen/cmd/udp-rust
proto:
  specPath: ../../.forge/spec-cache/udp/hello.v1.proto
layout:
  side: core
  cell: udp
```

The path is cell relative, so it climbs to the repo root and reads
`.forge/spec-cache/udp/hello.v1.proto`. The resolver copies a spec under
its own basename, and a repo resolves a REST spec, a gRPC proto and a
UDP proto at once. The udp directory keeps the UDP proto from landing on
the gRPC one when both are named `hello.v1.proto`.

The `generate` tool takes the normalized forge-dev model. `name` is the
service and names the crates `<name>-core` and `<name>-app`. `protoSpec`
is the proto3 document. `layout.side` picks the half of the skeleton
this cell holds, `core` or `app`. `layout.cell` names the module
directory and defaults to `udp`.

Every emitted path is relative to the cell directory. The engine never
writes above it.

## What the proto decides

For every `service` in the file:

| Emitted, under the cell | Holds |
|---|---|
| `types/zz_generated_<service>_messages.rs` | one prost message per message reachable from the service's rpcs |
| `types/zz_generated_context.rs` | `Context`, the session id and the peer address a handler receives |
| `controller/zz_generated_<service>_codec.rs` | the frame, the schema version, the function hash of every rpc and the prost encode and decode |
| `controller/zz_generated_<service>_controller.rs` | trait `<Service>Controller` and the impl that calls the hand body |
| `port/zz_generated_<service>_client.rs` | trait `<Service>Client`, one async method per rpc, and its error enum |
| `hand/<service>_controller.rs` | the body, written once and never again |
| `driver/zz_generated_<service>_udp_driver.rs` | `<Service>UdpDriver<C>`, a socket loop forwarding each datagram to the `<Service>Controller` it was built with |
| `adapter/zz_generated_<service>_udp_client.rs` | `<Service>UdpClient`, one datagram out and one reply in, behind the port trait |

Each layer directory carries a `mod.rs` that mounts its generated file
and aliases it under the logical name. The cell's own `mod.rs` lists
the layers.

## The wire layout

A datagram is the udplb frame.

| Bytes | Field |
|---|---|
| 0-3 | magic `0x55554944` big endian |
| 4-19 | session id, 16 bytes |
| 20 | schema version |
| 21 | function hash |
| 22-N | payload, one prost message |

A datagram carries at most 508 bytes, the magic counted, so a payload
holds at most 486. A reply repeats the session id, the version and the
function hash of the request it answers.

The schema version is the number in the version segment of the proto
package. `songe.hello.v1` gives 1.

The function hash is FNV-1a 32 over the full method name in the form
`package.Service/Method`, folded to 8 bits by the xor of its four
bytes. The engine computes every hash at generate time and emits
`<RPC>_METHOD` and `<RPC>_HASH` in the codec. Two methods that fold to
the same byte end the generation with an error naming both.

The codec refuses a datagram with no magic, one shorter than a header,
one over 508 bytes, one that speaks another schema version, and one
whose function hash names no rpc. A client reading a reply also refuses
a datagram framed with a session id it never opened.

## The driver

`serve` binds nothing. `new` takes a bound `tokio::net::UdpSocket` and
one `<Service>Controller` by value, stored behind a type parameter.
`announce` prints `LISTENING_UDP <port>` when a caller asks for it.

A datagram whose session id is 16 zero bytes is the udplb health probe.
The driver answers it verbatim before it decodes anything.

A datagram that speaks another schema version is dropped and logged
once per peer. A datagram whose function hash names no rpc is dropped
and logged.

A recv error pauses 50 milliseconds. A hundred in a row ends `serve`
with the address and the count.

## What the crates need

`core` needs `prost` and `thiserror`, plus `mockall` under dev. It needs
no build script and no `protoc`, because the message types carry their
own prost derives. `app` needs the core crate, `prost`, `thiserror` and
`tokio` with `net` and `time`.

The consumer's own `lib.rs` mounts the cell with one plain line, which
hexagonal-rust writes from `layout.cells`:

```rust
pub mod udp;
```
