# generated-drift

Fail a repo when the code its generators wrote is no longer the code they would write.

## Why

Forge decides whether to rebuild from a digest of what a build entry read and what it wrote. The generator between the two is not in that record. Change a generator and every committed file it wrote is stale while every digest still matches, so the build skips and every gate stays green.

The artifact store is where those digests live. This engine takes the store away, builds, puts the store back, and compares what git shows before against what git shows after. A build that starts with no memory cannot skip, so every generator runs and every file it writes is compared against what is committed.

## Use

```yaml
test:
  - name: generated
    runner: forge://github.com/alexandremahdhaoui/forge-dev-codegen/cmd/generated-drift
    spec: {}
```

## Which forge rebuilds

The engine runs the forge it was built against and refuses any other, because a forge of another version sends fields the current engines reject and the failure then reads as a broken generator.

Build info answers which one that is. A build recording a version of `github.com/alexandremahdhaoui/forge` resolves a forge through the shared tool precedence, asks it for its version, and refuses a mismatch by name with both versions and the resolved path. A build recording no version came from the enclosing workspace, so it runs `go run github.com/alexandremahdhaoui/forge/cmd/forge`, the same rule forge uses for its own engines, and the two are one source tree. Neither path reads an environment variable.

## The artifact store

The engine leaves the store exactly as it found it, absent included.

It renames the store to a sibling `.aside` file rather than holding it in memory, so an interrupt leaves it on disk under a name. It takes the same `.lock` file forge takes around a store write, once to move the store and once to put it back, so a run beside an ordinary build cannot lose that build's records. It never holds the lock across the build, because the build writes the store itself.

## Output

A passing repo returns no findings. A failing repo names every path, what happened to it, and what to do about it.

| Change | It means | Remedy |
|---|---|---|
| rewritten | the build wrote different bytes over a file git shows | read the new bytes and commit them |
| written | the build created a file the repo neither tracks nor ignores | add it to `.gitignore`, or commit it |
| removed | the build deleted a file git shows | drop it from git |

The comparison is by content, so a file already edited when the gate started is reported only when the build writes over it.

The build already wrote those files, so they sit in the tree when the gate fails. Nothing puts them back. An ordinary `forge build` would skip every one of them, which is the hole this gate exists to catch, so a remedy that says to rebuild would not work. Two consequences. Review the tree rather than rerunning. And a second run compares against what the first run left, so revert the first run's files before you read the second run's answer.

## Next

- [schema.md](schema.md)
