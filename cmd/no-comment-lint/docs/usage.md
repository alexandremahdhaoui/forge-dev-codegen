# no-comment-lint

Fail a repo when a hand written source file carries a comment.

## Why

Comments rot. The code must explain itself instead.

## Use

```yaml
test:
  - name: no-comments
    runner: forge://no-comment-lint
```

## Output

Passing tree returns zero findings. A failing tree lists every finding with
file, line and text, then fails the report.

## Next

- [schema.md](schema.md)
