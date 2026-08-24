# demo-cobra-cli

The proof that cli-cobra is a drop-in for the builtin cli cell. The
handlers file is exactly what the builtin cell would take; only the
`generator:` line differs, and the behavior does not:

```sh
demo-cobra-cli greet world     # hello [world]
demo-cobra-cli fail 4          # exit 4
demo-cobra-cli nope            # exit 2, unknown command "nope"
```

The unit stage pins all of it against the built binary.
