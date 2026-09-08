# authz-gen Configuration

Emits the OpenFGA module of one service from its authz.yaml. It writes the module file in the modular DSL, the fga.yaml test file holding every vector, and a manifest. Its combine kind reads those manifests and writes the fga.mod and the merged fga.yaml the fga CLI runs.

> Full OpenAPI specification: [spec.openapi.yaml](../spec.openapi.yaml)

## Fields

### `note`

- **Type:** `string`
- **Required:** No
- **Description:** Unused. The engine has no configuration. The model it receives carries everything.

