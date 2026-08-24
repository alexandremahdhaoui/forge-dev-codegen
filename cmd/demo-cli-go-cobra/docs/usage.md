# demo-cli-go-cobra

The proof that cli-go-cobra is a drop-in for the builtin cli cell. The
handlers file is exactly what the builtin cell would take; only the
`generator:` line differs, and the behavior does not:

```sh
demo-cli-go-cobra greet world     # hello [world]
demo-cli-go-cobra fail 4          # exit 4
demo-cli-go-cobra nope            # exit 2, unknown command "nope"
```

The unit stage pins all of it against the built binary.
