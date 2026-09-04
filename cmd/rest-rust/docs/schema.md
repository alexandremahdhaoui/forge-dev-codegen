# rest-rust Configuration

Emits the rust rest cell of one service from its OpenAPI document. It writes the wire types, the axum driver, a store port and a sqlite adapter per x-store schema, and a controller trait with the struct that carries its ports.

> Full OpenAPI specification: [spec.openapi.yaml](../spec.openapi.yaml)

## Fields

### `note`

- **Type:** `string`
- **Required:** No
- **Description:** Unused. The engine has no configuration. The model it receives carries everything.

