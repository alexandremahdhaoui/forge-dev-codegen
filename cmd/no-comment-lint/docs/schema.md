# no-comment-lint Configuration

Fails a repo when a hand written source file carries a comment outside its allowed exemptions.

> Full OpenAPI specification: [spec.openapi.yaml](../spec.openapi.yaml)

## Fields

### `exclude`

- **Type:** `array of string`
- **Required:** No
- **Description:** Glob patterns excluded from the scan (default is zz_generated*, **/zz_generated*, **/docs/**)

### `languages`

- **Type:** `array of string`
- **Required:** No
- **Description:** Source languages to scan (default is go, rust, python, typescript)

### `rootDir`

- **Type:** `string`
- **Required:** No
- **Description:** Root directory to scan for source files (default is current directory)

