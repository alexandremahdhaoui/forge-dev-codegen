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
| hexagonal-rust | hexagonal x rust | axum, rusqlite, thiserror, mockall |

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
| delivery-gen | `kind: delivery` | one Containerfile per surface binary |

A module directory in a consuming repo is the engine cell: it holds a
forge-dev.yaml naming the concern engine and receives the emitted files
in place. The golden repos are the reference consumers.

## Build and test

```sh
forge build
forge test-all
```
