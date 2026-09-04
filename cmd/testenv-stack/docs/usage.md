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
              addrEnv: HELLO_URL
              readyTimeoutSeconds: 30
```

Each service starts with the accumulated testenv env, then its own
`env` block, then `<NAME>_ADDR=127.0.0.1:0`. The engine reads the
service's stdout until a `LISTENING <port>` line appears or the timeout
passes. Stdout and stderr go to `<TmpDir>/<name>.log`.

The artifact exports `addrEnv` as `http://127.0.0.1:<port>` per
service, lists `stack.<name>.log` and `stack.pids` under files, and
reports each pid in metadata as `testenv-stack.<name>.pid`.

Processes outlive the create call. They run in their own session and
their pids sit in `<TmpDir>/stack.pids`. Delete reads that file, sends
SIGTERM to each process, and sends SIGKILL to any still alive after
five seconds.
