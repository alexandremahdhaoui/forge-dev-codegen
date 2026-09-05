# windows-allow Configuration

Clears a locally built Windows exe past Smart App Control by restamping a copy so its hash changes until the dev machine allows a build. A dev machine tool. Never a release step.

> Full OpenAPI specification: [spec.openapi.yaml](../spec.openapi.yaml)

## Fields

### `attempts`

- **Type:** `integer`
- **Required:** No
- **Description:** How many builds to try before giving up. Defaults to 8.

### `destination`

- **Type:** `string`
- **Required:** Yes
- **Description:** The directory the allowed build lands in. Env vars are expanded, so $WIN_OUTPUT_PATH works.

### `keep`

- **Type:** `integer`
- **Required:** No
- **Description:** How many builds of this binary to keep in the destination after an allowed one. Older builds of the same name are removed. Builds of other projects are never touched. Defaults to 3.

### `name`

- **Type:** `string`
- **Required:** Yes
- **Description:** The base name of the game binary. The deployed file is name-commit-hash.exe.

### `probeArgs`

- **Type:** `array of string`
- **Required:** No
- **Description:** The arguments the probe runs the exe with.

### `probeExpect`

- **Type:** `string`
- **Required:** Yes
- **Description:** The string an allowed build prints. Its absence with Invalid argument present means the build is blocked.

### `probeTimeoutSeconds`

- **Type:** `integer`
- **Required:** No
- **Description:** How long the probe waits for the exe to print before it stops it. Defaults to 8.

### `rootDir`

- **Type:** `string`
- **Required:** No
- **Description:** The repo directory whose commit names the deployed file. Defaults to the current directory.

### `source`

- **Type:** `string`
- **Required:** Yes
- **Description:** The built Windows exe to clear, a path relative to rootDir.

