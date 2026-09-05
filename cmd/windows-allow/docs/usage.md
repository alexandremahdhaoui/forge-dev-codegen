# windows-allow

Clears a locally built Windows exe past Smart App Control.

## Why it exists

Smart App Control allows an unsigned exe by its file hash. The machine's cloud guess on a hash it has never seen lands allowed about one time in two and sticks to the bytes. So a build that was refused stays refused, and the way through is a hash the machine has not judged yet. This tool writes a copy of the build with its own timestamp field changed, so each copy is a new hash, probes it, and stops at the first the machine allows. It names each file by its hash and never overwrites an allowed build.

This is a dev machine tool. It probes the exe through WSL, which works only on the box that built it. Never put it on a release path.

## The root fix

A signature that chains to a Microsoft trusted root passes the policy with no retry. Azure Artifact Signing under a company, or an OV code signing certificate, is the shippable answer. This tool exists only for the local loop until then.

## Spec

```yaml
test:
  - name: windows-allow
    runner: forge://github.com/alexandremahdhaoui/forge-dev-codegen/cmd/windows-allow
    spec:
      source: build/dist/songe-hello-node_windows_amd64.exe
      destination: $WIN_OUTPUT_PATH
      name: songe-hello-node
      attempts: 8
      keep: 3
      probeExpect: LISTENING
      probeTimeoutSeconds: 8
```

The stage passes when a build is allowed and names the file. It fails when every attempt is refused, and running it again often clears.

## Cleanup

Every deployed file is named `name-commit-hash.exe`, so the name is the marker of what this tool built. After an allowed build the tool keeps that build plus the newest `keep` builds of the same name and removes the older ones by timestamp. It matches on the `name-` prefix, so a build of another binary or another project keeps a different prefix and is never touched. A build the machine refused is removed as soon as the probe reads it, so only the ones that run stay.
