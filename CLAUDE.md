# CLAUDE.md

This repo holds the first party forge-dev generator engines: one engine
per third party library, one kind and language cell each, all behind the
one `generate` contract.

Read ~/.claude/CLAUDE.md first. Those rules apply here.

## Scope

A collection, not a gate. Anyone's engine at any `forge://` URI speaks
the same contract and plugs in the same way; being in this repo grants
nothing. What this repo adds is the guarantee that the first party
engines are drop-in swaps: same author contract as the builtin cell they
replace, same behavior vectors.

## The two contracts every engine here honors

**The generate contract.** One MCP tool, `generate`: the normalized
forge-dev model in, files out, paths relative to the engine directory.
forge-dev core keeps parsing, freshness, the runnable manifest and the
writing. The spec is `docs/model.md` in forge's `cmd/forge-dev`.

**The behavior contract of the cell.** A generator that fills a builtin
cell must be a drop-in: the author writes the same handlers file
(`NewCLIHandlers` for the cli kind) and the program behaves the same -
exit codes pass through, an unknown command exits 2 naming itself, a nil
handler fails loud. The demo consumer beside each engine pins this
against the built binary. Swapping the library must never change
behavior; if the library disagrees with the contract, wrap the library.

## Adding an engine

One `cmd/<kind>-<language>-<library>` directory - the cell, then the
library: forge-dev.yaml (an mcp-server with
the one `generate` tool), spec.openapi.yaml, handlers.go doing the
emitting, docs/usage.md. Add a demo consumer and its test. Nothing else
changes; the registry is the factory, not a table in code.

## The concern engines

`logging-gen`, `telemetry-gen`, `resilience-gen` and `containerfile-gen` each
fill their own custom kind across all four languages. They render through
one shared package, `internal/concerns`, so two cells naming one concern
cannot drift: one template, one renderer. A concern engine is named
`<concern>-gen` - no language in the name, because one engine serves
every language cell of its concern. The golden repos consume them in
place: the module directory is the engine cell.

## Build and test

```sh
forge build
forge test-all
```

Stages are lint and unit. The demo tests exec built binaries, never
`go run` - `go run` swallows exit codes.
