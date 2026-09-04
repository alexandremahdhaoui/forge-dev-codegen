# forge-lint

Fail a repo when its forge.yaml carries a stale engine or a missing script, or
when a controller or a driver reaches an adapter.

## Why

A stale `go://` URI or a missing `hack/` script breaks a build days after the
edit that caused it. forge-lint catches it at review time.

A controller that names an adapter loses the port that made it testable. A
driver that names an adapter skips the controller that owns the decision.

## Use

```yaml
test:
  - name: forge-lint
    runner: forge://forge-lint
```

## The adapter stays out of reach

In a Rust repo forge-lint reads every rust file under `src/controller`,
`src/driver` and the same two layers of every cell. A line naming
`crate::adapter`, `super::adapter` or `crate::<cell>::adapter` is flagged as
`forge-lint-adapter-out-of-reach`. The message names the line and the path it
found.

A repo counts as a Rust repo when it carries a `Cargo.toml` or a `crate` key
under `factory`.

## Output

Passing tree returns zero findings. A failing tree lists one finding per
violation, then fails the report.

## Next

- [schema.md](schema.md)
