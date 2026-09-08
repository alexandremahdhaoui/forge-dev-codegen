# generated-drift

Fail a repo when the code its generators wrote is no longer the code they would write.

## Why

Forge decides whether to rebuild from a digest of what a build entry read and what it wrote. The generator between the two is not in that record. Change a generator and every committed file it wrote is stale while every digest still matches, so the build skips and every gate stays green.

The artifact store is where those digests live. This engine takes the store away, builds, puts the store back, and compares what git shows before against what git shows after. A build that starts with no memory cannot skip, so every generator runs and every file it writes is compared against what is committed.

## Use

```yaml
test:
  - name: generated
    runner: forge://github.com/alexandremahdhaoui/forge-dev-codegen/cmd/generated-drift
    spec: {}
```

## Output

A passing repo returns no findings. A failing repo names every path and what happened to it.

| Change | It means |
|---|---|
| rewritten | the build wrote different bytes over a file git shows |
| written | the build created a file the repo neither tracks nor ignores |
| removed | the build deleted a file git shows |

The comparison is by content, so a file already edited when the gate started is reported only when the build writes over it.

## Next

- [schema.md](schema.md)
