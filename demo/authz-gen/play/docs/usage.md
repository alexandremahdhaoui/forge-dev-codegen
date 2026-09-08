# demo-authz-play

The extending module of the authz-gen demo. `authz.yaml` beside this
file extends the `session` type of the session module with
`turn_owner`, `move` and `end_turn`. It declares a `monster` type whose
`alive` relation holds under the `monster_alive` condition and whose
`attack` permission holds for the turn owner while the monster is alive.

Everything else in this directory is `zz_generated`. Run `forge build`
at the repo root to regenerate it. The golden test in
`internal/authzgen` compares the committed files against a fresh run.
