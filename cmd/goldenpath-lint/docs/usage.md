# goldenpath-lint

Fail a repo when its layout departs from the golden path.

## Why

A repo that drifts from the hexagonal layout hides the boundary between
business logic and the outside world.

## Use

```yaml
test:
  - name: goldenpath
    runner: forge://goldenpath-lint
    spec:
      layout: rust-core-app
```

## Cells

A directory under `src` that holds a `forge-dev.yaml` is a cell. A second
generator fills it. The layer rules apply one level down inside it.

A cell of a `-core` crate holds `port`, `controller`, `types` and `hand`. A cell
of an `-app` crate holds `adapter` and `driver`. A directory inside a cell that
holds rust under another name is flagged.

A cell's `mod.rs` and everything under its `hand` directory are hand written
files. Every other rust file inside a cell is generated and named
`zz_generated*`.

## Output

Passing tree returns zero findings. A failing tree lists one finding per
violation, then fails the report.

## Next

- [schema.md](schema.md)
