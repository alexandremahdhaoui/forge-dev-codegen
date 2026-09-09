# testenv-stack

A testenv subengine that starts service binaries as detached child
processes and exports their addresses. A test stage then talks to a
running stack through plain environment variables.

```yaml
test:
  - name: integration
    runner: forge://generic-test-runner
    testenv:
      - engine: forge://github.com/alexandremahdhaoui/forge-dev-codegen/cmd/testenv-stack
        spec:
          services:
            - name: hello
              binary: ./build/bin/hello
              addrEnv: HELLO_ADDR
              readyTimeoutSeconds: 30
```

Each service starts with the accumulated testenv env, then its own
`env` block. Stdout and stderr go to `<TmpDir>/<name>.log`. Processes
outlive the create call.

## Ports

A service either declares the ports it wants or announces the ports it
picked.

A service that declares `ports` gets each one bound free before it
starts. The engine substitutes the number into `args` and into `env`
through `@port.<name>@` and exports it as `<addrEnv>_<NAME>` in the
form `127.0.0.1:<port>`. `@tmpDir@` substitutes the test environment
directory, so a datastore file belongs to one stack and two stacks never
share it. A placeholder naming anything else is refused, and so is
`@port.<name>@` naming a port the service did not declare.

The delimiter is `@` and not `{{ }}` because forge expands Go templates
over this spec before the engine ever reads it, so a double brace is
already taken. Forge's own `allocateOpenPort` is stable per identifier,
which means two stacks of the same declaration get the same port. The
engine binds its own instead, so two stacks never collide.

A service that declares no `ports` is started with `addrEnv` set to
`127.0.0.1:0`, picks its own ports and announces each one on stdout.
`LISTENING <port>` exports `<addrEnv>`. `LISTENING_<SUFFIX> <port>`
exports `<addrEnv>_<SUFFIX>`. Every announced address carries the
`http://` scheme except `_UDP`, which has none.

## Readiness

`ready.kind` is one of three, and an absent block reads as `stdout`.

| kind | waits for | needs |
|---|---|---|
| stdout | a `LISTENING` line in the log | nothing |
| tcp | a connection to a declared port | `port` |
| http | a get answering `status` | `port`, `path`, `status` |

Any other kind is refused by name. A field a kind does not read is
refused rather than ignored. `readyTimeoutSeconds` caps the wait and
defaults to 30. A service that exits before it is ready fails create.

## After

`after` runs commands once the service is ready, in order. Each one
captures one value out of its stdout and exports it.

```yaml
after:
  - command: seed
    args: [store, create, --api-url, "http://127.0.0.1:@port.http@"]
    export: STORE_ID
    jsonPath: store.id
```

A command with a path separator resolves against the project root. A
bare name is looked up on PATH. Every command runs from the project root
with the env the stack has exported so far, so a later command reads an
earlier export. Exactly one of `regex` and `jsonPath` names the value.
`regex` takes one capturing group. `jsonPath` takes dotted segments, and
a numeric segment indexes an array. A pattern that matches nothing, a
path that names nothing, and a regex with the wrong number of groups are
each refused by name.

Every export lands in the artifact env and in the metadata as
`testenv-stack.<name>.export.<KEY>`.

## Teardown

Each service runs as the leader of its own session and process group,
and its pid sits in `<TmpDir>/stack.pids`. Delete reads that file, sends
SIGTERM to each group, and sends SIGKILL to any group still alive after
five seconds. Grandchildren die with their service. The artifact lists
`stack.<name>.log` and `stack.pids` under files and reports each pid in
metadata as `testenv-stack.<name>.pid`.
