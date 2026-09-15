# testenv-sqlite

A testenv subengine that creates one sqlite file per `x-store` schema of
an OpenAPI document. The file carries the schema the `hexagonal-rust`
sqlite adapter opens, so a service under test starts against a real
store with no migration step.

```yaml
test:
  - name: integration
    runner: forge://generic-test-runner
    testenv:
      - engine: forge://github.com/alexandremahdhaoui/forge-dev-codegen/cmd/testenv-sqlite
        spec:
          specPath: .forge/spec-cache/hello.v1.yaml
          stores: [Greeting]
          seed: .forge/spec-cache/testdata/cases.json
          keep: false
```

For each store name `Name` the engine finds `components.schemas.Name`
with an `x-store` object and writes `<TmpDir>/<snake>.db` holding two
tables: `"<snake>"("<key>" TEXT PRIMARY KEY, body TEXT NOT NULL)` and
`audit(at, table_name, key, op, before, after)`. `key` is the property
`x-store.key` names. Every identifier is quoted, so a key named after a
SQL keyword still writes valid DDL. Every lookup that answers `one`
becomes `"<snake>_unique_by_<joint>"`, a unique index over the
properties it reads.

`seed` names a vectors file. Every case whose `operation` starts with
`create` and carries a `controllerReply` becomes one row keyed by the
reply's key property. The reply goes to the store whose required
properties it covers.

A seed inserts. It never replaces. Two rows sharing a key, or sharing
the properties a `one` lookup reads, make sqlite refuse the script and
the stage fails naming the index.

The artifact exports `SONGE_STORE_<UPPER>_PATH` per store, lists
`sqlite.<snake>` under files, and reports row counts in metadata. With
`keep: false` the files are managed resources and are removed with the
environment.

The engine writes the file through the `sqlite3` binary when it is on
`PATH`, and through `python3` with its `sqlite3` module otherwise. One
of the two must exist on the machine that runs the stage.
