# authz-gen

A forge-dev generator that turns one service's `authz.yaml` into its
OpenFGA module. It answers three files. The module in the modular DSL.
The test file holding every vector. A manifest a spec repo combines
into `fga.mod`. The internal model is backend neutral. A Cedar emitter
lands beside the Cedar adapter.

The cell is one directory in the spec repo with `forge-dev.yaml` and
`authz.yaml` side by side.

```yaml
name: songe-session-authz
kind: authz
version: 0.1.0
description: The authorization module of songe-session.
generator: forge://github.com/alexandremahdhaoui/forge-dev-codegen/cmd/authz-gen
wiring:
  specPath: ./authz.yaml
generate:
  packageName: main
  docsBaseURL: https://raw.githubusercontent.com/alexandremahdhaoui/songe-session-spec/refs/heads/main
```

The `generate` tool takes the normalized forge-dev model. `kind` must be
`authz`. `wiringSpec` is the text of `authz.yaml`. Every emitted path is
relative to the cell directory and named `zz_generated`.

## The authz.yaml format

| Key | Meaning |
|---|---|
| `module` | The module name. A lower case identifier. It names the emitted files and the `module` line. |
| `references` | Types of other modules this module names in subjects, expressions or vectors. Each entry names the module and its types. A type lists the relations this module reads. |
| `types` | The types this module declares. Each has a name and relations. |
| `extends` | Types of another module this module adds relations to. Each entry names the module and the types with their new relations. |
| `conditions` | CEL conditions. Each has a name, typed parameters and an expression. |
| `vectors` | Test cases. Each has a name, the tuples it needs and the checks it asserts. |

A relation has a name and one or both of `subjects` and `expression`.
`subjects` lists the types it accepts directly. A subject is a type, a
wildcard `type:*` or a userset `type#relation`. `condition` names a
condition every direct subject is guarded by. `expression` computes the
relation from others with `or`, `and`, `but not`, parentheses and
`relation from tupleset`. One operator per level. Group the rest with
parentheses. A relation with both renders as `[subjects] or expression`.

A condition parameter type is one of `int`, `uint`, `double`, `bool`,
`bytes`, `string`, `duration`, `timestamp`, `ipaddress`, `any`,
`list<T>` or `map<T>`.

A tuple is `user`, `relation`, `object` and an optional `condition` with
a `name` and a `context`. A check is `user`, `relation`, `object`,
`expected` as `true` or `false` and an optional `context`. A check names
one user, never a wildcard.

The engine refuses, each with a message naming the fix. A module name
that is not a lower case identifier. An unknown subject type. An
expression naming an unknown relation. A `from` relation through a
computed relation. A case naming an unknown type or relation. A check
without `expected`. An unknown condition. An extension repeating a
relation of the referenced type. An unknown key.

## Three modules

A base module. Accounts own characters. A session has a host and members.

```yaml
module: session
types:
  - name: account
    relations:
      - name: self
        subjects: [account]
      - name: create_session
        expression: self
  - name: character
    relations:
      - name: owner
        subjects: [account]
      - name: delete_character
        expression: owner
  - name: session
    relations:
      - name: host
        subjects: [character]
      - name: member
        subjects: [character]
      - name: join
        expression: member
      - name: leave
        expression: member
vectors:
  - name: a member may join
    tuples:
      - user: character:alice
        relation: member
        object: session:s1
    checks:
      - user: character:alice
        relation: join
        object: session:s1
        expected: true
      - user: character:bob
        relation: join
        object: session:s1
        expected: false
```

A referencing module. A channel belongs to a session. Only a session
member may post.

```yaml
module: chat
references:
  - module: session
    types:
      - name: character
      - name: session
        relations: [member, host]
types:
  - name: channel
    relations:
      - name: session
        subjects: [session]
      - name: member
        expression: member from session
      - name: post
        expression: member
vectors:
  - name: a member may post and a non member may not
    tuples:
      - user: session:s1
        relation: session
        object: channel:c1
      - user: character:alice
        relation: member
        object: session:s1
    checks:
      - user: character:alice
        relation: post
        object: channel:c1
        expected: true
      - user: character:bob
        relation: post
        object: channel:c1
        expected: false
```

An extending module. The session gains a turn owner. A monster can be
attacked by the turn owner while it is alive.

```yaml
module: play
references:
  - module: session
    types:
      - name: character
      - name: session
        relations: [member, host]
extends:
  - module: session
    types:
      - name: session
        relations:
          - name: turn_owner
            subjects: [character]
          - name: end_turn
            expression: turn_owner
types:
  - name: monster
    relations:
      - name: session
        subjects: [session]
      - name: alive
        subjects: ["character:*"]
        condition: monster_alive
      - name: attack
        expression: alive and turn_owner from session and member from session
conditions:
  - name: monster_alive
    parameters:
      - name: hp
        type: int
    expression: hp > 0
vectors:
  - name: a dead monster refuses attack
    tuples:
      - user: session:s1
        relation: session
        object: monster:boar
      - user: character:alice
        relation: member
        object: session:s1
      - user: character:alice
        relation: turn_owner
        object: session:s1
      - user: "character:*"
        relation: alive
        object: monster:boar
        condition:
          name: monster_alive
    checks:
      - user: character:alice
        relation: attack
        object: monster:boar
        context:
          hp: 0
        expected: false
```

The three live under `demo/authz-gen` with their generated output.

## The emitted files

`zz_generated_<module>.fga` is the module. The `module` line, the types,
the `extend type` blocks, the conditions. No `model` block. The schema
version lives in `fga.mod`.

```
module play

type monster
  relations
    define session: [session]
    define alive: [character:* with monster_alive]
    define attack: alive and turn_owner from session and member from session

extend type session
  relations
    define turn_owner: [character]
    define end_turn: turn_owner

condition monster_alive(hp: int) {
  hp > 0
}
```

`zz_generated_<module>.fga.yaml` is the test file. `model_file` names
`fga.mod`, so it runs once the modules are combined. One entry under
`tests` per vector with its tuples and one check per assertion.

```yaml
name: "play"
model_file: "fga.mod"
tests:
  - name: "a dead monster refuses attack"
    tuples:
      - user: "character:*"
        relation: "alive"
        object: "monster:boar"
        condition:
          name: "monster_alive"
    check:
      - user: "character:alice"
        object: "monster:boar"
        context: {"hp":0}
        assertions:
          "attack": false
```

`zz_generated_authz.json` is the manifest. The combining step in
`songe-authz-spec` reads one per module to write `fga.mod` and one
merged test file.

```json
{
  "module": "play",
  "schema": "1.2",
  "model": "zz_generated_play.fga",
  "tests": "zz_generated_play.fga.yaml",
  "types": ["monster"],
  "references": [{"module": "session", "types": ["character", "session"]}],
  "extends": [{"module": "session", "types": ["session"]}],
  "cases": ["a dead monster refuses attack"]
}
```

## Running the vectors in a spec repo

The `fga` CLI runs the tests. It is a toolchain binary the factory
provisions. The combining step copies every module file beside one
`fga.mod` and one merged `fga.yaml` under `.forge/authz`.

```yaml
schema: "1.2"
contents:
  - zz_generated_session.fga
  - zz_generated_chat.fga
  - zz_generated_play.fga
```

The test stage runs `fga model test` on the merged file.

```yaml
test:
  - name: authz
    runner: forge://generic-test-runner
    spec:
      command: fga
      args: ["model", "test", "--tests", ".forge/authz/fga.yaml"]
```

The same command by hand.

```sh
fga model test --tests .forge/authz/fga.yaml
```
