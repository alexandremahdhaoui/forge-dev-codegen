# demo-authz-combined

The combine cell of the authz-gen demo. `combine.yaml` beside this file
names the manifest of every module. The engine copies each module file
here, writes `zz_generated_fga.mod` and merges every vector into
`zz_generated_fga.yaml`.

The `demo-authz-fga` test stage runs the merged file.

```sh
fga model test --tests demo/authz-gen/combined/zz_generated_fga.yaml
```

Everything else in this directory is `zz_generated`. Run `forge build`
at the repo root to regenerate it. A module change reaches the
combination on `forge build --force`, because forge tracks
`combine.yaml` and not the manifests it names.
