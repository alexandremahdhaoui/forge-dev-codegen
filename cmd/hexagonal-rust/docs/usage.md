# hexagonal-rust

A forge-dev generator that stands one rust service crate up around the
cells that fill it. It reads no OpenAPI document and no proto file. It
reads the cell list, the wiring document and the manifest every cell
writes, and it answers the crate root, the root layer modules, the
config module, the config schema and main.

```yaml
name: songe-hello
kind: hexagonal
language: rust
generator: forge://github.com/alexandremahdhaoui/forge-dev-codegen/cmd/hexagonal-rust
configGenerator:
  engine: forge://github.com/alexandremahdhaoui/golden-configgen/cmd/configgen-gen
  outputDir: src/config
layout:
  cells: [rest, grpc, udp]
wiring:
  specPath: ./wiring.yaml
```

`layout.cells` lists the module directories under `src`. Each one owns
its own `forge-dev.yaml` and its own generator. `wiring.specPath` names
the document that says which adapter answers which port and which driver
starts.

## The wiring document

```yaml
binary: songe-hello-node
ports:
  GreetingStore:
    default: sqlite
    adapters:
      sqlite: {}
      memory:
        type: GreetingMemoryStore
        module: adapter::greeting_memory
        config:
          capacity: { type: integer, default: 100 }
drivers:
  rest: { enabled: true }
  grpc: { enabled: true }
  udp:  { enabled: true }
```

`binary` names the binary main lands in, at
`src/bin/zz_generated_<binary>.rs`.

A candidate with no `type` is looked up by name among the adapters the
cell manifests provide. A candidate with a `type` and a `module` is
written by hand. Its module lives under `adapter::`, the crate's root
adapter layer, and the generated `src/adapter/mod.rs` mounts it with one
`mod` line. When such a candidate declares config fields, the engine
also writes its `<Type>Config` struct.

A field carries a type among string, integer, boolean and duration, plus
`default` and `description`.

## What it refuses

Every refusal names the thing that broke.

- a listed cell with no `src/<cell>/forge-dev.yaml`
- a listed cell with no `src/<cell>/zz_generated_cell.yaml`
- a port a controller consumes and the wiring names no candidate for
- a candidate that declares no type and no manifest provides
- a driver the wiring names and no manifest provides
- a driver a manifest provides and the wiring never names
- a driver that requires a controller trait no manifest provides
- a key the wiring schema does not know

## What it emits

| Emitted | Holds |
|---|---|
| `src/lib.rs` | one `pub mod` per root layer, the config module and every cell |
| `src/<layer>/mod.rs` | the root layer, adapter and driver opening with the clippy allow |
| `src/adapter/zz_generated_<module>_config.rs` | the config struct of one hand written candidate |
| `src/config/mod.rs` | mounts the loader the config generator writes |
| `zz_generated_config_spec.yaml` | the Spec schema the config generator reads |
| `src/bin/zz_generated_<binary>.rs` | main |

## The config keys

The schema holds one property per decision.

| Key | Type | From |
|---|---|---|
| `<port>` | string | the port choice, defaulting to the wiring default |
| `<port><Candidate><Field>` | the field type | one candidate config field |
| `driver<Name>` | boolean | whether that driver starts |
| `<driver><Field>` | the field type | one driver config field |

configgen-gen cannot emit an enum, so the port choice travels as a
string. Main matches it and answers an error naming the port and every
candidate it knows.

## Main

Main loads the config, matches each port choice into one boxed adapter,
builds each controller with its ports in declaration order, and starts
each driver its flag enables. A driver is built with its config and the
controllers it requires, bound, announced and spawned. Main refuses to
run with every driver disabled and then joins the ones it started.

The crate needs `anyhow`, `tokio` and `songe-common`, whose
`error::chain` main uses to render a driver failure.
