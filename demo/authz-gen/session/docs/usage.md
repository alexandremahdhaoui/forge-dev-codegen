# demo-authz-session

The base module of the authz-gen demo. `authz.yaml` beside this file
declares three types. An account owns a character. A session has a host
and members. `join` and `leave` hold for a member. `delete_character`
holds for the owner.

Everything else in this directory is `zz_generated`. Run `forge build`
at the repo root to regenerate it. The golden test in
`internal/authzgen` compares the committed files against a fresh run.
