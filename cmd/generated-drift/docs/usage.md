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

Build info describes the gate, never the repo under test. So the repo under test must sit in the same Go workspace as the gate. A gate built in a workspace takes the workspace path for every repo it is pointed at, and a repo outside that workspace has no version to run forge at, so the run fails loudly instead of answering wrong.

The version question is asked from an empty directory the engine makes, with `GIT_CEILING_DIRECTORIES` set above it and with `GIT_DIR` and `GIT_WORK_TREE` stripped from the environment. An unstamped forge falls back to `git describe` in whatever directory it was started from, so a forge with no version of its own would otherwise answer the tags of the repo under test. Those tags live in the same namespace and the factory bumps them together, so the answer could match by accident and the gate would accept a forge it never checked. The ceiling stops the walk upward, which matters when the temporary directory lands inside a repository. It does nothing against an exported git directory, which names a repository outright and which every git hook exports, so those two variables go. Away from every repository that forge answers `dev` and the gate refuses it by name.

## The artifact store

The gate runs alone in a repo.

It renames the store to a sibling `.aside` file rather than holding it in memory, so an interrupt leaves it on disk under a name. It takes the same `.lock` file forge takes around a store write, once to move the store and once to put it back. It never holds that lock across the build, because the build writes the store itself.

The restore renames the copy back over whatever the build left at the store path. That is why the gate runs alone. An ordinary `forge build` running beside it loses every record it wrote. A second gate on the same repo is worse. It would move the first gate's fresh store aside over the first gate's saved copy, destroy the only copy of the original, and answer green on a comparison the first gate poisoned. So the gate takes a second lock of its own, `.drift-lock` beside the store, and holds it from before the move until after the restore. A second gate waits there rather than interleaving.

A run that finds an `.aside` already in place refuses and names both paths, because an interrupted run left it and the move would overwrite the only copy. Read that file. Move it back over the store path when it is the store you want to keep. Delete it when it is not. Then run the gate again.

The gate leaves the store exactly as it found it, absent included, when it runs alone.

## The build must find nothing to skip

The lock keeps two gates from interleaving their moves. It does not keep the outer forge that started the first gate from writing the whole store back into the hole the second gate has already made, after the lock is released and before the first answer arrives. The second gate then builds against a full store, skips every entry, regenerates nothing, and compares a tree no generator touched.

So the gate reads its own build's output. A build that starts with no artifact store has no record to call anything fresh, so an entry reported as skipped and unchanged proves the store was there. The gate refuses and names every entry forge skipped, the repository and the store path. The refusal holds whoever wrote the store and whenever they wrote it, and it takes no flag.

The shape it matches is one line of forge's own build, `⏭  Skipping <entry> (unchanged)`, whole line, prefix and suffix. Forge writes that line in one place and only after the freshness rule found a record. The other line starting the same way ends `(built by the <stage> stage that needs it)`, which is an entry a test stage owns rather than an entry found fresh, and the suffix separates the two.

An ordinary `forge build` beside the gate is still not protected. The gate's build reads no store, so it reports no skip and the refusal stays silent, while the restore still renames the saved copy over whatever that build wrote. Protecting it needs one lock that every writer of the store respects for the whole of a build, which is forge's to take, not this gate's.

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
