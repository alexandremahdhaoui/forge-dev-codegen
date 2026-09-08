# demo-tui-rust

The reference consumer of tui-rust. One cell, `src/tui`, generated from
`tui.yaml` beside this file. The user's file is
`controller/board_controller.rs`. Everything else in the cell is
`zz_generated`.

The board is 12 by 6. An at sign starts at the top left. `h`, `j`, `k`,
`l` and the arrow keys move it one cell. `a` acts. Space ends the turn.
Enter opens a prompt, a second Enter puts the line on the message line.
`q` quits. Control C quits too.

Run it from a terminal.

```sh
cargo run --manifest-path demo/tui-rust/Cargo.toml
```

Test it.

```sh
cargo test --manifest-path demo/tui-rust/Cargo.toml
```

The controller tests drive `on_key`, `on_line` and `on_tick` with keys
and check the frame. The loop tests drive `TuiDriver` with `MockScreen`
and `MockKeyboard` and check what was drawn and that the screen was left
on every exit, a quit, an error and a panic.
