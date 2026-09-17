# chart-gen

Emits a Helm chart for one crate. It reads the config schema hexagonal-rust
writes and the cell manifests, so nothing about the chart is spelled by hand.

The chart is a cell at `<crate>/chart` with its own `forge-dev.yaml`.

```yaml
name: songe-hello
kind: chart
version: 0.1.0
generator: forge://github.com/alexandremahdhaoui/forge-dev-codegen/cmd/chart-gen
openapi:
  specPath: ../zz_generated_config_spec.yaml
```

From the crate root it reads `forge-dev.yaml` for the name, the version, the
description, `layout.cells` and `wiring.specPath`. It reads the wiring file
for the binary. It reads `src/<cell>/zz_generated_cell.yaml` for every listed
cell. A missing root `forge-dev.yaml`, a missing manifest or a listed cell
with none is refused by name.

It emits, relative to the cell.

| File | Holds |
|---|---|
| `Chart.yaml` | the root name and description, the root version as both `version` and `appVersion` |
| `values.yaml` | `image.repository` set to the binary, then one key per config key with its default |
| `templates/zz_generated_deployment.yaml` | one Deployment, one replica, one env entry per config key named by its `x-env`, one container port per driver with a protocol |
| `templates/zz_generated_service.yaml` | one Service with one port per driver with a protocol. Absent when no driver carries one |

A container port and a service port read the text after the last colon of
`.Values.<driver>_addr` at template time. The driver name and `_addr` compose
the key, spelled as the config schema spells it. A driver whose key the
schema lacks is refused by name.

Every object carries `app.kubernetes.io/name` equal to `.Chart.Name` and
nothing else. No ingress, no configmap, no secret, no helpers, no notes.
