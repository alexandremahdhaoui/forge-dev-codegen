# testenv-stack Configuration

Starts a list of service binaries on free ports, waits for their LISTENING line, and exports one address env var per service.

> Full OpenAPI specification: [spec.openapi.yaml](../spec.openapi.yaml)

## Fields

### `services`

- **Type:** `array of `
- **Required:** Yes
- **Description:** Services started in order. Each one must print LISTENING <port> on stdout once bound.

