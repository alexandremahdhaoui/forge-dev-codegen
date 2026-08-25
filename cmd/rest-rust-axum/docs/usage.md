# rest-rust-axum

A forge-dev generator that fills the rest-api x rust cell with a axum
server. The surface is the OpenAPI paths, exactly like the builtin go
cell:

```yaml
kind: rest-api
language: rust
generator: forge://github.com/alexandremahdhaoui/forge-dev-codegen/cmd/rest-rust-axum
```

Each operation routes to a handler the author writes beside the module:
one function per operationId taking the path parameters and answering a
status and a body. The route table derives from the paths verbatim, so
the server cannot drift from the spec; main binds <NAME>_ADDR (default
127.0.0.1:0) and prints LISTENING <port> once bound. The demo consumer
in the matching golden repo pins the behavior, and golden-e2e's
restdemo-conformance stage holds all four cells to one answer.
