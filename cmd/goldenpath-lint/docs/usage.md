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

## Flat layers

A layer directory holds files only. A subdirectory inside `adapter`, `config`,
`controller`, `driver`, `port` or `types`, at the root and in every cell, is
flagged as `rust-layer-not-flat` and the message names the subdirectory.

`src/bin` is exempt from the flat rule. Cargo discovers a binary at
`src/bin/<name>.rs` and at `src/bin/<name>/main.rs`, so a binary directory
there is legal.

A layer holds rust files and the known manifests `forge-dev.yaml`,
`zz_generated_cell.yaml` and `zz_generated.runnable.yaml`. `mod.rs` passes
because it ends in `.rs`. Any other file is flagged as `rust-layer-stray-file`
and the message names the file. The rule reads only the top level of `src/bin`,
so a file inside a binary directory is never flagged.

## Pure layers

A file under `controller`, `port` or `types`, at the root and in every cell,
never names an io crate on a use statement or on an attribute. The banned names
are `axum`, `hyper`, `reqwest`, `rusqlite`, `tokio`, `tonic`, `tower`,
`std::fs`, `std::net`, `std::process` and `std::time::Instant`. The finding
`rust-io-use` names the file, the line and the crate.

The scan reads a use statement whole. rustfmt spreads a grouped use over many
lines, so the lines are joined up to the closing semicolon before matching. The
finding reports the line the statement starts on. An attribute line such as a
tokio test attribute is scanned the same way.

A use statement no semicolon closes is skipped. The join stops at a blank line,
at the end of the file, or at a line that leaves no brace open and carries no
semicolon. The statement is reported nowhere and the lines after it are scanned
as usual.

A leading `::` is read. `use ::std::fs;` names `std::fs` like `use std::fs;`
does. A `pub`, `pub(crate)`, `pub(super)` or `pub(in path)` in front of the use
does not hide it.

Generated files are checked too. A generator that emits io into a controller is
a bug.

## Mounted files

A layer directory carries a generated `mod.rs`. A layer that holds a hand
written rust file and carries no generated `mod.rs` is flagged once as
`rust-layer-mod-rs`. No file inside that layer is flagged.

Every rust file beside a generated `mod.rs` that the generator did not write is
reached by one `mod <name>;` line in that `mod.rs`. A file nothing reaches is
flagged as `rust-file-not-mounted` and the message names the `mod.rs` that
should reach it.

`src/bin` carries no `mod.rs`. Cargo discovers a binary by name, so no file
there is ever flagged as unmounted.

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
