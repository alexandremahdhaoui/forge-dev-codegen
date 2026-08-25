# delivery-gen

One of forge-dev-codegen's composable concern engines: it fills a
`kind: delivery` cell in any of the four languages with the delivery module,
identical to what the repo-level CLI emits - one template, one renderer,
two addresses.

```yaml
name: my-delivery
kind: delivery
language: go
generator: forge://github.com/alexandremahdhaoui/forge-dev-codegen/cmd/delivery-gen
```

A repo composes exactly the concerns it wants: each concern is its own
engine, consumed in place by the module directory that holds the emitted
file. The `generate` tool takes the normalized forge-dev model and
answers files; forge-dev core writes them.
