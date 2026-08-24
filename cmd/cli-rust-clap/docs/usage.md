# cli-rust-clap

A forge-dev generator that fills the cli x rust cell with a clap
dispatcher. Same behavior contract as every cli cell:

```yaml
kind: cli
language: rust
generator: forge://github.com/alexandremahdhaoui/forge-dev-codegen/cmd/cli-rust-clap
surface:
  commands:
    - name: greet
      description: Print the greeting.
```

The handler owns its args raw, exit codes pass through, an unknown
command exits 2 naming itself, a missing handler fails loud. The
`generate` tool takes the normalized forge-dev model and answers one
dispatcher module; forge-dev core writes it and keeps the manifest,
freshness and docs. The demo consumer in the matching golden repo pins
the contract, and golden-e2e's clidemo-conformance stage holds all the
cli cells to the same vectors.
