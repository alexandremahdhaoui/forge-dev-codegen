# goldenpath-lint Configuration

Fails a repo when its layout departs from the hexagonal golden path for its language.

> Full OpenAPI specification: [spec.openapi.yaml](../spec.openapi.yaml)

## Fields

### `layout`

- **Type:** `string`
- **Required:** Yes
- **Description:** The golden path layout to check, either go or rust. The older spelling rust-core-app still names the rust layout

### `rootDir`

- **Type:** `string`
- **Required:** No
- **Description:** Root directory of the repo to check (default is current directory)

