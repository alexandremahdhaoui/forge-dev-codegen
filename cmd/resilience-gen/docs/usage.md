# resilience-gen

One of forge-dev-codegen's composable concern engines: it fills a
`kind: resilience` cell in any of the four languages with the resilience module,
identical to what the repo-level CLI emits - one template, one renderer,
two addresses.

```yaml
name: my-resilience
kind: resilience
language: go
generator: forge://github.com/alexandremahdhaoui/forge-dev-codegen/cmd/resilience-gen
```

A repo composes exactly the concerns it wants: each concern is its own
engine, consumed in place by the module directory that holds the emitted
file. The `generate` tool takes the normalized forge-dev model and
answers files; forge-dev core writes them.
