# goldenpath-lint

Fail a repo when its layout departs from the golden path.

## Why

A repo that drifts from the hexagonal layout hides the boundary between
business logic and the outside world.

## Use

```yaml
test:
  - name: goldenpath
    runner: forge://goldenpath-lint
    spec:
      layout: rust
```

The `go` layout checks a Go repo. The `rust` layout checks one crate. The older
spelling `rust-core-app` still names the `rust` layout.

## One crate

A service is one crate. Under `src` the only directories are the layers
`adapter`, `bin`, `config`, `controller`, `driver`, `port` and `types`, plus the
cells. Any other directory holding rust is flagged as `rust-src-layout`.

The crate carries a `Cargo.toml`, a `forge.yaml` and a `src/lib.rs`. A crate
that holds `zz_generated` files under `src` also carries a `forge-dev.yaml`.

## Cells

A directory under `src` that holds a `forge-dev.yaml` is a cell. A second
generator fills it. `src/rest`, `src/grpc` and `src/udp` are cells like any
other.

Inside a cell the only directories are the five layers `adapter`, `controller`,
`driver`, `port` and `types`. Another directory holding rust is flagged as
`rust-cell-layout`.

A cell that holds `zz_generated` files also holds a `zz_generated_cell.yaml`.
A missing manifest is flagged as `rust-cell-manifest`.

## Pure layers

A file under `controller`, `port` or `types`, at the root and in every cell,
never names an io crate on a use line. The banned names are `axum`, `hyper`,
`reqwest`, `rusqlite`, `tokio`, `tonic`, `tower`, `std::fs`, `std::net`,
`std::process` and `std::time::Instant`. The finding `rust-io-use` names the
file, the line and the crate.

Generated files are checked too. A generator that emits io into a controller is
a bug.

## Mounted files

A layer directory carries a generated `mod.rs`. Every rust file beside it that
the generator did not write is reached by one `mod <name>;` line in that
`mod.rs`. A file nothing reaches is flagged as `rust-file-not-mounted` and the
message names the `mod.rs` that should reach it.

A file counts as generated when its name starts with `zz_generated` or its first
line carries the generator header.

## Path attributes

`rust-no-path-attribute` flags any `#[path` in a rust file under `src`,
generated or hand written. A module directory carries a real `mod.rs` beside
its files, so `lib.rs` reads as plain `pub mod <layer>;` lines.

## Output

Passing tree returns zero findings. A failing tree lists one finding per
violation, then fails the report.

## Next

- [schema.md](schema.md)
