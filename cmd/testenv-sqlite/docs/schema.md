# testenv-sqlite Configuration

Creates one sqlite file per x-store schema of an OpenAPI document, with the schema the hexagonal-rust engine expects, seeded from the vectors file.

> Full OpenAPI specification: [spec.openapi.yaml](../spec.openapi.yaml)

## Fields

### `keep`

- **Type:** `boolean`
- **Required:** No
- **Description:** Keep the database files after the test. They leave managedResources so the orchestrator never deletes them.

### `pathEnv`

- **Type:** `map[string]string`
- **Required:** No
- **Description:** The environment variable each store's file path is exported as, keyed by store name. A store this never names keeps the SONGE_STORE_<UPPER>_PATH default. The consumer owns the name because the binary that opens the file owns its config keys.

### `seed`

- **Type:** `string`
- **Required:** No
- **Description:** Path to a vectors file. Every case whose operation starts with create and carries a controllerReply is inserted into the store the reply belongs to.

### `specPath`

- **Type:** `string`
- **Required:** Yes
- **Description:** Path to the OpenAPI document whose components.schemas hold the x-store schemas. Relative paths resolve against the project root.

### `stores`

- **Type:** `array of string`
- **Required:** Yes
- **Description:** Names of the x-store schemas to create a database for. Each becomes <TmpDir>/<snake>.db.

