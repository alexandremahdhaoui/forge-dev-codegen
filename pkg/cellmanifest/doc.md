# Cell manifest

The cell manifest is the contract between a transport engine and hexagonal-rust.
One schema. Three writers. One reader. Any language.

A transport engine generates a cell inside a service crate. It knows what that
cell holds. hexagonal-rust generates `lib.rs`, the root layer modules and main.
It knows nothing about a cell until the cell says so. The manifest is how a cell
says so.

## The file

Every transport engine writes `zz_generated_cell.yaml` beside the code it
generates. The first line names the generator and forbids hand edits.

Every manifest carries `version: "1"`. A reader refuses any other value.

The manifest lists what the cell provides and what it needs.

- `provides.drivers` a driver, the controller traits it needs, its config fields
- `provides.adapters` an adapter, the port it implements, whether its `new` is
  fallible, its config fields
- `provides.controllers` a controller trait, its impl struct, its ports
- `provides.ports` a port trait the cell declares
- `requires.ports` a port trait the cell needs somebody else to declare
- `buildScript` the cargo build script the cell emits, as a path relative to
  the cell. Optional. The name starts with `zz_generated`. hexagonal-rust
  includes every declared script from the crate root build script.

A config field carries a type among string, integer, boolean and duration, plus
`required`, `default` and `description`. Names are snake case. Types and traits
are Rust idents. Modules are `::` paths and every driver, adapter, controller
and port names one.

An integer default above 2 to the 53 travels as a string, because JSON numbers
lose precision past that point.

## Who writes and who reads

| Role | Who |
|---|---|
| writer | rest-rust, grpc-rust-tonic, udp-rust |
| reader | hexagonal-rust |

`Read`, `Parse` and `Write` move the file. `Validate` refuses a manifest the
reader cannot use. `Merge` gathers the cells of one service. It refuses two
manifests that name one cell. It refuses two cells that provide one driver name,
one adapter name or one controller trait. It refuses two cells that provide one
port trait from different modules and accepts a repeat that matches. `Schema`
returns the JSON Schema so a writer in another language checks its own output.
