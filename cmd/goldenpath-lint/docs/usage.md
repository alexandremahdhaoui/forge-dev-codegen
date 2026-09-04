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

## Output

Passing tree returns zero findings. A failing tree lists one finding per
violation, then fails the report.

## Next

- [schema.md](schema.md)
