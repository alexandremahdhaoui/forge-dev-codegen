# udp-rust

A forge-dev generator that turns one proto3 file into the rust skeleton
of every UDP datagram service it declares. It fills one cell, a module
directory under `src`. The cell holds the prost message types, the codec,
the client port trait, the controller trait with its struct and `new`,
the listening driver and the client adapter. With a hello rpc named it
also holds a session gate port, a broadcast port, a push enum, the peer
table adapter and a tick driver.

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
  cell: udp
  hello: Hello
  push: [Counter]
```

The path is cell relative, so it climbs to the repo root and reads
`.forge/spec-cache/udp/hello.v1.proto`. The resolver copies a spec under
its own basename, and a repo resolves a REST spec, a gRPC proto and a
UDP proto at once. The udp directory keeps the UDP proto from landing on
the gRPC one when both are named `hello.v1.proto`.

The `generate` tool takes the normalized forge-dev model. `name` is the
service and names the crate. `protoSpec` is the proto3 document.
`layout.cell` names the module directory and defaults to `udp`.
`layout.hello` names the rpc a session opens with. `layout.push` lists
the rpcs the server sends to a client. A name in either list that is
not an rpc of the proto ends the generation with an error naming it. A
push with no hello is refused, because a push reaches the peers a hello
admitted.

Every emitted path is relative to the cell directory. The engine never
writes above it.

## What the proto decides

For every `service` in the file:

| Emitted, under the cell | Holds |
|---|---|
| `types/zz_generated_<service>_messages.rs` | one prost message per message reachable from the service's rpcs |
| `types/zz_generated_context.rs` | `Context`, the session id and the peer address a handler receives |
| `controller/zz_generated_<service>_codec.rs` | the frame, the schema version, the function hash of every rpc and the prost encode and decode |
| `controller/zz_generated_<service>_controller.rs` | trait `<Service>Controller`, the struct `<Service>ControllerImpl` with its ports and `new`, and the `mod` line of the user's impl file |
| `port/zz_generated_<service>_client.rs` | trait `<Service>Client`, one async method per inbound rpc, and its error enum |
| `driver/zz_generated_<service>_udp_driver.rs` | `<Service>UdpDriver`, a socket loop forwarding each datagram to the controller it was built with |
| `adapter/zz_generated_<service>_udp_client.rs` | `<Service>UdpClient`, one datagram out and one reply in, behind the port trait |

The user writes `src/<cell>/controller/<service>_controller.rs` holding
`impl <Service>Controller for <Service>ControllerImpl`. A missing file
is a compile error naming it.

Each layer directory carries a `mod.rs` that mounts its generated file
and aliases it under the logical name. The cell's own `mod.rs` lists
the layers.

## What a hello adds

Naming `layout.hello` turns the session on. The cell then emits these
too.

| Emitted, under the cell | Holds |
|---|---|
| `types/zz_generated_admission.rs` | `Admission`, `Admitted` or `Refused { reason }` |
| `types/zz_generated_<service>_push.rs` | `<Service>Push`, one variant per push rpc carrying its request message, and `kind()` |
| `port/zz_generated_<service>_session_gate.rs` | trait `<Service>SessionGate` with `admit(session_id, &hello, peer)` answering an `Admission`, under mockall automock |
| `port/zz_generated_<service>_broadcast.rs` | trait `<Service>Broadcast` with `send_to(session_id, push)` and `send_all(push)`, plus `attach`, `admit_peer` and `follow_peer` the driver uses |
| `adapter/zz_generated_<service>_udp_broadcast.rs` | `<Service>UdpBroadcast`, the peer table and the socket slot, implementing the broadcast port |
| `driver/zz_generated_<service>_tick_driver.rs` | `<Service>TickDriver`, calls `on_tick` on the controller every `interval_ms` |

The controller trait gains `on_tick(&self, tick: u64)`. The struct gains
one field, the broadcast port, and `new` takes it. The error enum gains
`Broadcast { kind, source }`. A push rpc gets no inbound method on the
controller and no method on the client port. The codec still names its
method and hash and emits `encode_push` and `decode_push`.

The manifest lists the gate and the broadcast under `ports` on the udp
driver, the broadcast under `ports` on the controller, the peer table
under `provides.adapters` as `udp_broadcast` with `max_sessions`, and
the tick driver under `provides.drivers` as `tick` with `interval_ms`.
hexagonal-rust builds the peer table first, hands it to the controller
as the boxed broadcast port and to the udp driver after the controller.
The wiring names an adapter for the gate. It stays silent on the
broadcast, because the cell provides its only adapter.

## The session flow

The driver decodes a datagram, then looks at its rpc.

On the hello rpc the gate receives the session id, the decoded hello
and the peer. `Admitted` puts the session in the peer table with that
peer and calls the controller. `Refused` drops the datagram and logs
the reason once per peer. A full table refuses a session it does not
hold and logs once per peer. `max_sessions` sizes it, 64 by default.

On any other inbound rpc a session the table does not hold is dropped
and logged once per peer. A session it holds moves to the sender's
address before the controller runs. The server always uses the latest
address for a session.

The same hello again from a new address goes through the gate again
and, when admitted, replaces the peer. That is a reconnect.

A push reaches a peer through the broadcast port. The controller sends
`<Service>Push` to one session or to every session. The peer table
encodes it with the codec, the method hash of the push rpc and the same
frame as a reply, and sends it through the socket the driver attached
at bind. Before bind every send fails naming the unbound driver. A send
to every session skips a peer the socket refuses and answers how many
it reached.

The tick driver takes `interval_ms`, refuses a value below 1 at bind,
prints `TICKING <ms>` on announce, and calls `on_tick` with a count
starting at 1. The controller pushes state from there.

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
function hash of the request it answers. A push carries the session id
of the peer it reaches and the function hash of its push rpc.

The schema version is the number in the version segment of the proto
package. `songe.hello.udp.v1` gives 1.

The function hash is FNV-1a 32 over the full method name in the form
`package.Service/Method`, folded to 8 bits by the xor of its four
bytes. The engine computes every hash at generate time and emits
`<RPC>_METHOD` and `<RPC>_HASH` in the codec. Two methods that fold to
the same byte end the generation with an error naming both.

The codec refuses a datagram with no magic, one shorter than a header,
one over 508 bytes, one that speaks another schema version, one whose
function hash names no rpc, and one whose function hash names a push
rpc arriving inbound. A client reading a reply also refuses a datagram
framed with a session id it never opened.

## The driver

`new` takes the config, one `<Service>Controller` behind `Arc<dyn>`,
and with a hello the gate port and the broadcast port after it. `bind`
opens the socket and attaches it to the broadcast. `announce` prints
`LISTENING_UDP <port>` when a caller asks for it.

A datagram whose session id is 16 zero bytes is the udplb health probe.
The driver answers it verbatim before it decodes anything.

A datagram that speaks another schema version is dropped and logged
once per peer. A datagram whose function hash names no rpc is dropped
and logged.

A recv error pauses 50 milliseconds. A hundred in a row ends `serve`
with the address and the count.

## What the crate needs

The crate needs `prost`, `thiserror` and `tokio` with `net` and `time`,
plus `mockall` under dev. It needs no build script and no `protoc`,
because the message types carry their own prost derives.

The consumer's own `lib.rs` mounts the cell with one plain line, which
hexagonal-rust writes from `layout.cells`:

```rust
pub mod udp;
```
