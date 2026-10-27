# chart-gen

chart-gen emits a Helm chart for one crate. chart-gen reads the config
schema hexagonal-rust writes and the cell manifests, so nothing about the
chart is spelled by hand.

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
description, `layout.cells` and `wiring.specPath`. chart-gen reads the
wiring file for the binary and `src/<cell>/zz_generated_cell.yaml` for every
listed cell. A missing root `forge-dev.yaml`, a missing manifest or a listed cell
with none is refused by name.

chart-gen emits these files, relative to the cell.

| File | Holds |
|---|---|
| `Chart.yaml` | the root name and description, the root version as both `version` and `appVersion` |
| `values.yaml` | `image.repository` set to the binary, then one key per config key with no `x-secret` with its default, plus `secretName` when a key carries `x-secret` |
| `templates/zz_generated_deployment.yaml` | one Deployment, one replica, one env entry per config key named by its `x-env`, one container port per enabled driver with a protocol |

A config key marked `x-secret: true` holds a secret value. The env entry
of a secret key reads it with `valueFrom.secretKeyRef` from the Kubernetes
Secret that `.Values.secretName` names, under a Secret key spelled like the
config key. `secretName` has no default, and the render fails naming
`secretName` while nobody sets it. A secret key gets no `values.yaml` entry
of its own, and the chart never renders a secret value. A person writes the
Secret into the cluster once by hand. chart-gen refuses by name a config key
that carries `x-secret` and a default, because git holds no secret.
| `templates/zz_generated_service.yaml` | one Service with one port per enabled driver with a protocol. Absent when no driver carries one, and renders nothing when every such driver is off |

A container port and a service port read the text after the last colon of
`.Values.<driver>_addr` at template time. hexagonal-rust's config key rule
composes the key from the driver name and the field `addr`, so the chart and
the config schema spell it one way. A driver whose key the schema lacks is
refused by name. A port that is not an integer above zero fails the render
naming the key. A loopback host, `127.0.0.0/8`, `localhost` or `[::1]`,
fails the render naming the key too, since nothing outside the pod reaches
a loopback host.

A port renders only when `.Values.driver_<driver>` is on, in the Service and
in the container alike, and the render never reads the addr of a driver that
is off. The same config key rule spells that flag key. A driver whose flag key
the schema lacks is refused by name.

Every object carries `app.kubernetes.io/name` equal to `.Chart.Name` and
nothing else. No ingress, no configmap, no secret, no helpers, no notes.
