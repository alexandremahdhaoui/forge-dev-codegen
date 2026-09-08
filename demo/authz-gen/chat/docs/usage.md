# demo-authz-chat

The referencing module of the authz-gen demo. `authz.yaml` beside this
file declares one type, `channel`. It names the `session` and
`character` types of the session module under `references` and reads
membership through `member from session`. `post` holds for a member.

Everything else in this directory is `zz_generated`. Run `forge build`
at the repo root to regenerate it. The golden test in
`internal/authzgen` compares the committed files against a fresh run.
