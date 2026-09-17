# chart-gen Configuration

Emits a Helm chart for one crate from the config schema hexagonal-rust writes and the cell manifests. Chart.yaml, one values key per config key, a Deployment env block in the loader's spelling, one Service port per driver with a protocol.

> Full OpenAPI specification: [spec.openapi.yaml](../spec.openapi.yaml)

## Fields

### `note`

- **Type:** `string`
- **Required:** No
- **Description:** Unused. The engine has no configuration. The config schema and the cell manifests carry everything.

