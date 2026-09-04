# forge-dev-codegen

The first party forge-dev generator engines: one engine per third party
library, each filling one kind and language cell of the forge-dev model
behind the same `generate` contract.

A cell names an engine and gets its generated skeleton from the library
that engine wraps:

```yaml
name: my-tool
kind: cli
generator: forge://github.com/alexandremahdhaoui/forge-dev-codegen/cmd/cli-go-cobra
```

Swap the URI and the skeleton changes library; the author contract and
the behavior contract do not. Every engine here is gated by the same
vectors, so a swap cannot change what a program does.

This repo is a collection, not a gate. The contract is one MCP tool -
`generate`, the normalized forge-dev model in, files out - and any engine
at any `forge://` URI implements it. A custom engine in your own repo
plugs in exactly the way these do; see `docs/model.md` in forge's
`cmd/forge-dev` for the contract.

## Engines

A library-backed engine is named `<kind>-<language>-<library>`: the cell
it fills, then the library that fills it.

| Engine | Cell | Library |
|---|---|---|
| cli-go-cobra | cli x go | github.com/spf13/cobra |
| cli-rust-clap | cli x rust | clap |
| cli-python-typer | cli x python | typer |
| cli-typescript-commander | cli x typescript | commander |
| rest-rust-axum | rest-api x rust | axum |
| rest-python-fastapi | rest-api x python | fastapi |
| rest-typescript-fastify | rest-api x typescript | fastify |
| hexagonal-rust | hexagonal x rust | thiserror, mockall |
| rest-rust | rest x rust | axum, rusqlite, thiserror, mockall |
| grpc-rust-tonic | grpc x rust | tonic, prost, protox |
| udp-rust | udp x rust | prost, tokio |
| vectors-rust | vectors x rust | mockall, tower, http-body-util, tokio |

`cmd/demo-cli-go-cobra` consumes cli-go-cobra and pins its behavior
against the built binary; the other cells' demos live in the matching
golden repos, and golden-e2e's clidemo-conformance stage holds all four
libraries to one behavior.

A concern engine is named `<concern>-gen`: it fills its own custom kind
in any of the four languages, from one shared renderer
(`internal/concerns`), so two cells naming one concern cannot drift.

| Engine | Kind it fills | Answers |
|---|---|---|
| logging-gen | `kind: logging` | the logging module |
| telemetry-gen | `kind: telemetry` | the metrics and tracing module |
| resilience-gen | `kind: resilience` | the resilience module |
| delivery-gen | `kind: delivery` | one Containerfile per layout binary |
| testenv-sqlite | testenv subengine | one sqlite file per x-store schema, seeded from vectors, exports SONGE_STORE_<UPPER>_PATH |
| testenv-stack | testenv subengine | starts service binaries on free ports, waits for LISTENING, exports one address env per service |
| no-comment-lint | test runner | fails on any comment in go, rust, python or typescript. license headers and generated files exempt |
| goldenpath-lint | test runner | fails when a repo drifts from the go or rust layout, a layer is not flat, a layer holds a stray file, a file no generated mod line reaches, or a use line of a pure layer names an io crate or an io path |
| forge-lint | test runner | fails on go:// URIs, an unknown engine scheme, a missing artifactStorePath, missing hack scripts, a unit stage with no runner, go-build on a Rust repo, or an adapter reached from a controller or a driver |

A module directory in a consuming repo is the engine cell: it holds a
forge-dev.yaml naming the concern engine and receives the emitted files
in place. The golden repos are the reference consumers.

## Build and test

```sh
forge build
forge test-all
```
