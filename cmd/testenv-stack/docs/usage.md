# testenv-stack

A testenv subengine that starts service binaries as detached child
processes, each on a free port, and exports one address per service.
A test stage then talks to a running stack through plain environment
variables.

```yaml
test:
  - name: integration
    runner: forge://generic-test-runner
    testenv:
      - engine: forge://github.com/alexandremahdhaoui/forge-dev-codegen/cmd/testenv-sqlite
        spec:
          specPath: .forge/spec-cache/hello.v1.yaml
          stores: [Greeting]
      - engine: forge://github.com/alexandremahdhaoui/forge-dev-codegen/cmd/testenv-stack
        spec:
          services:
            - name: hello
              binary: ./build/bin/hello
              addrEnv: HELLO_ADDR
              readyTimeoutSeconds: 30
```

Each service starts with the accumulated testenv env, then its own
`env` block, then `<addrEnv>=127.0.0.1:0` so the binary binds a free
port on the variable it reads. The engine reads the
service's stdout until a `LISTENING <port>` line appears or the timeout
passes. Stdout and stderr go to `<TmpDir>/<name>.log`.

A service that serves more than one transport announces one line per
transport. `LISTENING <port>` is REST. `LISTENING_GRPC <port>` and
`LISTENING_UDP <port>` are the other two. The engine waits half a second
after the REST line for the others to arrive.

The artifact exports `addrEnv` as `http://127.0.0.1:<port>` per service.
A service that announced gRPC also exports `<addrEnv>_GRPC` as
`http://127.0.0.1:<port>`. One that announced UDP exports `<addrEnv>_UDP`
as `127.0.0.1:<port>`, with no scheme, because UDP has none. It lists
`stack.<name>.log` and `stack.pids` under files, and reports each pid in
metadata as `testenv-stack.<name>.pid`.

Processes outlive the create call. Each runs as the leader of its own
session and process group, and its pid sits in `<TmpDir>/stack.pids`.
Delete reads that file, sends SIGTERM to each group, and sends SIGKILL
to any group still alive after five seconds. Grandchildren die with
their service.
