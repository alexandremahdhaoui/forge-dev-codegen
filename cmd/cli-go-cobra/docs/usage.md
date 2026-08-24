# cli-go-cobra

A forge-dev generator that fills the cli x go cell with a cobra
dispatcher. It is a drop-in swap for the builtin cell:

```yaml
kind: cli
generator: forge://github.com/alexandremahdhaoui/forge-dev-codegen/cmd/cli-go-cobra
surface:
  commands:
    - name: greet
      description: Print the greeting.
```

The author contract does not change: write `NewCLIHandlers() CLIHandlers`
with one `func(args []string) int` per command. Neither does the behavior
contract: exit codes pass through, an unknown command exits 2 naming
itself, a nil handler fails loud. Flag parsing is disabled on the
subcommands - the handler owns its args, exactly as the builtin cell
hands them over.

The `generate` tool takes the normalized forge-dev model and answers one
file, `zz_generated.cli.go`; forge-dev core writes it and keeps the
manifest, freshness and docs. `cmd/demo-cli-go-cobra` pins the contract
against the built binary.
