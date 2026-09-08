# tui-rust

A forge-dev generator that turns one small tui spec into the rust tui
cell of a service. The cell is a module directory under `src`. It holds
the frame and key types, a `Screen` port and a `Keyboard` port, one
crossterm adapter for each, a controller trait the user implements, and
a driver that owns the terminal loop.

The driver prints its announcement, then puts the terminal in raw mode
on `bind`, and in `serve` runs the loop. Read a key. Call the
controller. Draw what the controller returns. Stop when the controller
says quit. The terminal is restored on every exit path, a clean quit, an
error, a panic, and a driver dropped after `bind` without `serve`.

## The cell file

This file sits inside the cell, at `src/tui/forge-dev.yaml`. The build
step that runs it points `src` at the cell. forge-dev carries one
generator owned text file, the one `wiring.specPath` names. The tui cell
names its spec there.

```yaml
name: songe-tui
kind: tui
language: rust
generator: forge://github.com/alexandremahdhaoui/forge-dev-codegen/cmd/tui-rust
wiring:
  specPath: ./tui.yaml
layout:
  cell: tui
```

The `generate` tool takes the normalized forge-dev model. `name` is the
service. `wiringSpec` is the text of the tui spec. `layout.cell` names
the module directory and defaults to `tui`.

Every emitted path is relative to the cell directory. The engine never
writes above it.

## The spec

```yaml
controller: board
width: 12
height: 6
tickMs: 100
keys:
  left: h
  down: j
  up: k
  right: l
  act: a
  endTurn: space
  quit: q
  line: enter
```

| Key | Holds |
|---|---|
| `controller` | the name of the controller the user implements. `board` gives `BoardController` and `BoardControllerImpl` |
| `width` and `height` | the grid, 1 to 200 cells each way |
| `tickMs` | how long the driver waits for a key before it ticks the controller, 1 to 60000 |
| `keys` | the key map. Every action left out keeps its vi default shown above |

A key is one character or one of `space`, `enter`, `escape`. Two actions
on one key are refused by name. An action the engine does not know is
refused by name. The arrow keys always move, whatever the letters say.

## What the spec decides

| Emitted, under the cell | Holds |
|---|---|
| `types/zz_generated_frame.rs` | `Grid`, a text grid of cells. `Frame`, the grid plus a status line and a message line. `Step`, `Render(Frame)` or `Quit`. `Prompt`, `Closed` or `Open(String)` |
| `types/zz_generated_key.rs` | `Input`, what the keyboard yields. `Key`, the typed action the controller receives |
| `port/zz_generated_screen.rs` | trait `Screen` with `enter`, `draw` and `leave`, and `ScreenError` |
| `port/zz_generated_keyboard.rs` | trait `Keyboard` with `read`, and `KeyboardError` |
| `controller/zz_generated_<controller>_controller.rs` | trait `<Controller>Controller`, its error, the struct with its ports and `new`, and the `<Cell>Ports` trait the driver reaches the ports through |
| `driver/zz_generated_<cell>_driver.rs` | `<Cell>Driver`, the loop, the key map, and the guard that restores the terminal |
| `adapter/zz_generated_crossterm_screen.rs` | `CrosstermScreen`, raw mode, the alternate screen and the grid written to stdout |
| `adapter/zz_generated_crossterm_keyboard.rs` | `CrosstermKeyboard`, one key event read with a tick timeout |
| `zz_generated_cell.yaml` | the cell manifest hexagonal-rust reads |

Each layer directory carries a `mod.rs` that mounts its generated file
and aliases it under the logical name. The cell's own `mod.rs` lists the
layers. Both ports carry `#[cfg_attr(test, mockall::automock)]`, so a
crate test gets `MockScreen` and `MockKeyboard` for free.

## The controller trait

The user writes one file, `src/tui/controller/<controller>_controller.rs`,
holding `impl <Controller>Controller for <Controller>ControllerImpl`. A
missing file, method or impl is a compile error with a name.

```rust
pub trait BoardController: TuiPorts + Send + Sync {
    fn on_key(&self, frame: &Frame, key: Key) -> Result<Step, BoardControllerError>;
    fn on_line(&self, frame: &Frame, line: &str) -> Result<Step, BoardControllerError>;
    fn on_tick(&self, frame: &Frame) -> Result<Frame, BoardControllerError>;
}
```

One method per input. `on_key` receives the frame on screen and a typed
key and answers the next frame or `Quit`. `on_line` receives a line the
player typed at the prompt. `on_tick` runs when no key arrived within the
tick, so a server push renders with nothing pressed. It also draws the
first frame, from a blank one. The frame is the state the loop holds. A
controller that needs more keeps it behind a port.

`Key` is `Left`, `Down`, `Up`, `Right`, `Act`, `EndTurn` or `Quit`. A
character bound to nothing is ignored. Control C stops the loop without
asking the controller.

The `line` key opens a prompt. Typed characters go to the prompt.
Backspace erases one. Escape closes it. Enter hands the text to
`on_line` and closes it. The prompt draws below the message line.

## The ports and the driver

hexagonal-rust builds the adapters a controller consumes and hands them
to the controller's `new`. It hands a driver its controllers and nothing
else. So the manifest lists `Screen` and `Keyboard` as the controller's
ports, the generated struct carries them, and the generated `<Cell>Ports`
trait, a supertrait of the controller, hands them to the driver. The
user never touches them. The day a manifest driver may name ports of its
own, the supertrait goes away and nothing the user wrote changes.

`bind` refuses a `tick_ms` below 1 naming the key, prints
`TUI <width>x<height>` while the shell still owns the screen, then calls
`Screen::enter`. `announce` only refuses a driver that is not bound.
`serve` moves the driver into a blocking tokio task, draws the first
frame and loops there, so the async runtime keeps serving the other
drivers. A controller error ends `serve` with the input named. A
keyboard error ends it with the tick named. In every case
`Screen::leave` runs before the error comes back. The driver's `Drop`
runs `Screen::leave` when the screen is still open, so a panic inside
the loop and a driver dropped after `bind` both restore the terminal.
The crossterm screen turns raw mode off again when the alternate screen
cannot be entered, and shows the cursor at the end of the prompt line
while the prompt is open. `error_chain` in the driver module is public,
a main joins an error chain with it.

## The manifest

```yaml
cell: tui
generator: tui-rust (forge-dev-codegen)
provides:
  adapters:
  - {name: crossterm_screen, type: CrosstermScreen, module: tui::adapter::crossterm_screen, implements: Screen}
  - {name: crossterm_keyboard, type: CrosstermKeyboard, module: tui::adapter::crossterm_keyboard, implements: Keyboard}
  controllers:
  - {trait: BoardController, impl: BoardControllerImpl, module: tui::controller, ports: [Screen, Keyboard]}
  drivers:
  - {name: tui, type: TuiDriver, module: tui::driver::tui_driver, requires: [BoardController], config: {tick_ms: {type: duration, default: 100}}}
  ports:
  - {trait: Screen, module: tui::port::screen}
  - {trait: Keyboard, module: tui::port::keyboard}
```

The wiring names one adapter per port and enables the driver.

```yaml
ports:
  Screen: { default: crossterm_screen, adapters: { crossterm_screen: {} } }
  Keyboard: { default: crossterm_keyboard, adapters: { crossterm_keyboard: {} } }
drivers:
  tui: { enabled: true }
```

## What the crate needs

`crossterm`, `thiserror` and `tokio` with `rt`, plus `mockall` under
dev. The driver's `bind` and `serve` are async so main awaits them like
every other driver. `serve` awaits one blocking task that holds the
loop.

The consumer's own `lib.rs` mounts the cell with one plain line, which
hexagonal-rust writes from `layout.cells`:

```rust
pub mod tui;
```

`demo/tui-rust` in this repo is the reference consumer. Its controller
moves an at sign with the vi keys and quits on q. Its tests drive the
controller with keys and check the frame, and drive the loop with
`MockScreen` and `MockKeyboard`.
