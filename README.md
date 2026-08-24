# forge-dev-codegen

The first party forge-dev generator engines: one engine per third party
library, each filling one kind and language cell of the forge-dev model
behind the same `generate` contract.

A cell names an engine and gets its generated skeleton from the library
that engine wraps:

```yaml
name: my-tool
kind: cli
generator: forge://github.com/alexandremahdhaoui/forge-dev-codegen/cmd/cli-cobra
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

| Engine | Cell | Library |
|---|---|---|
| cli-cobra | cli x go | github.com/spf13/cobra |

`cmd/demo-cobra-cli` consumes cli-cobra and pins its behavior against the
built binary.

## Build and test

```sh
forge build
forge test-all
```
