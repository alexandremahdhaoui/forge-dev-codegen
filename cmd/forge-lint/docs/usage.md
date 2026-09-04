# forge-lint

Fail a repo when its forge.yaml carries a stale engine or a missing script.

## Why

A stale `go://` URI or a missing `hack/` script breaks a build days after the
edit that caused it. forge-lint catches it at review time.

## Use

```yaml
test:
  - name: forge-lint
    runner: forge://forge-lint
```

## Output

Passing tree returns zero findings. A failing tree lists one finding per
violation, then fails the report.

## Next

- [schema.md](schema.md)
