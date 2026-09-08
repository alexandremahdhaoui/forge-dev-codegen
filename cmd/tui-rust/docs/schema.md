# tui-rust Configuration

Emits the rust tui cell of one service from its tui spec. It writes the frame and key types, a Screen port and a Keyboard port, a crossterm adapter for each, a controller trait with the struct that carries its ports, and a driver that owns the terminal loop.

> Full OpenAPI specification: [spec.openapi.yaml](../spec.openapi.yaml)

## Fields

### `note`

- **Type:** `string`
- **Required:** No
- **Description:** Unused. The engine has no configuration. The model it receives carries everything.

