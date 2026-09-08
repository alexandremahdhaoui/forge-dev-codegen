# rest-rust Configuration

Emits the rust rest cell of one service from its OpenAPI document. The server side writes the wire types, the axum driver, a store port and a sqlite adapter per x-store schema, a TicketVerifier port for x-auth, a Subscribe port for x-stream, and a controller trait with the struct that carries its ports. The client side writes one client port per controller, a reqwest adapter per port and the TokenSource port it reads the bearer token from.

> Full OpenAPI specification: [spec.openapi.yaml](../spec.openapi.yaml)

## Fields

### `note`

- **Type:** `string`
- **Required:** No
- **Description:** Unused. The engine has no configuration. The model it receives carries everything.

