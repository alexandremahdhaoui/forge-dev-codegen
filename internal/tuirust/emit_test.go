// Copyright 2024 Alexandre Mahdhaoui
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tuirust_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/tuirust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/cellmanifest"
)

const boardSpec = `controller: board
width: 12
height: 6
tickMs: 100
`

func generate(t *testing.T, doc string, opts tuirust.Options) map[string]tuirust.File {
	t.Helper()

	files, err := tuirust.Generate([]byte(doc), opts)
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	byPath := map[string]tuirust.File{}
	for _, f := range files {
		byPath[f.Path] = f
	}

	return byPath
}

func TestGeneratingTheTuiSpecEmitsTheWholeFileSet(t *testing.T) {
	files, err := tuirust.Generate([]byte(boardSpec), tuirust.Options{Service: "songe-tui"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	wantPaths := []string{
		"adapter/mod.rs",
		"adapter/zz_generated_crossterm_keyboard.rs",
		"adapter/zz_generated_crossterm_screen.rs",
		"controller/mod.rs",
		"controller/zz_generated_board_controller.rs",
		"driver/mod.rs",
		"driver/zz_generated_tui_driver.rs",
		"mod.rs",
		"port/mod.rs",
		"port/zz_generated_keyboard.rs",
		"port/zz_generated_screen.rs",
		"types/mod.rs",
		"types/zz_generated_frame.rs",
		"types/zz_generated_key.rs",
		"zz_generated_cell.yaml",
	}

	gotPaths := []string{}
	for _, f := range files {
		gotPaths = append(gotPaths, f.Path)
	}

	if !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Fatalf("emitted paths\n got %q\nwant %q", gotPaths, wantPaths)
	}
}

func TestTheCellManifestNamesTheDriverTheTwoAdaptersTheControllerAndTheTwoPorts(t *testing.T) {
	files := generate(t, boardSpec, tuirust.Options{Service: "songe-tui"})

	m, err := cellmanifest.Parse([]byte(files[cellmanifest.FileName].Content))
	if err != nil {
		t.Fatalf("parsing the manifest: %v\n%s", err, files[cellmanifest.FileName].Content)
	}

	if m.Cell != "tui" {
		t.Errorf("cell = %q, want tui", m.Cell)
	}

	if len(m.Provides.Drivers) != 1 {
		t.Fatalf("drivers = %+v", m.Provides.Drivers)
	}

	driver := m.Provides.Drivers[0]
	if driver.Name != "tui" || driver.Type != "TuiDriver" || driver.Module != "tui::driver::tui_driver" {
		t.Errorf("driver = %+v", driver)
	}

	if !reflect.DeepEqual(driver.Requires, []string{"BoardController"}) {
		t.Errorf("driver requires = %+v", driver.Requires)
	}

	if driver.Config["tick_ms"].Type != cellmanifest.FieldTypeDuration || fmt.Sprint(driver.Config["tick_ms"].Default) != "100" {
		t.Errorf("driver config = %+v", driver.Config)
	}

	if len(m.Provides.Adapters) != 2 {
		t.Fatalf("adapters = %+v", m.Provides.Adapters)
	}

	screen, keyboard := m.Provides.Adapters[0], m.Provides.Adapters[1]
	if screen.Name != "crossterm_screen" || screen.Implements != "Screen" || screen.Type != "CrosstermScreen" || screen.Module != "tui::adapter::crossterm_screen" {
		t.Errorf("screen adapter = %+v", screen)
	}

	if keyboard.Name != "crossterm_keyboard" || keyboard.Implements != "Keyboard" || keyboard.Type != "CrosstermKeyboard" || keyboard.Module != "tui::adapter::crossterm_keyboard" {
		t.Errorf("keyboard adapter = %+v", keyboard)
	}

	if len(m.Provides.Controllers) != 1 {
		t.Fatalf("controllers = %+v", m.Provides.Controllers)
	}

	controller := m.Provides.Controllers[0]
	if controller.Trait != "BoardController" || controller.Impl != "BoardControllerImpl" || controller.Module != "tui::controller" {
		t.Errorf("controller = %+v", controller)
	}

	if !reflect.DeepEqual(controller.Ports, []string{"Screen", "Keyboard"}) {
		t.Errorf("controller ports = %+v, the driver reaches them through the controller", controller.Ports)
	}

	wantPorts := []cellmanifest.Port{
		{Trait: "Screen", Module: "tui::port::screen"},
		{Trait: "Keyboard", Module: "tui::port::keyboard"},
	}

	if !reflect.DeepEqual(m.Provides.Ports, wantPorts) {
		t.Errorf("ports = %+v", m.Provides.Ports)
	}
}

func TestTheControllerTraitHasOneMethodPerInputAndCarriesItsPortsForTheDriver(t *testing.T) {
	files := generate(t, boardSpec, tuirust.Options{Service: "songe-tui"})

	controller := files["controller/zz_generated_board_controller.rs"].Content

	for _, want := range []string{
		"pub trait TuiPorts: Send + Sync {",
		"fn screen(&self) -> Arc<dyn Screen + Send + Sync>;",
		"fn keyboard(&self) -> Arc<dyn Keyboard + Send + Sync>;",
		"pub trait BoardController: TuiPorts + Send + Sync {",
		"fn on_key(&self, frame: &Frame, key: Key) -> Result<Step, BoardControllerError>;",
		"fn on_line(&self, frame: &Frame, line: &str) -> Result<Step, BoardControllerError>;",
		"fn on_tick(&self, frame: &Frame) -> Result<Frame, BoardControllerError>;",
		"pub struct BoardControllerImpl {",
		"    pub(crate) screen: Arc<dyn Screen + Send + Sync>,",
		"    pub(crate) keyboard: Arc<dyn Keyboard + Send + Sync>,",
		"pub fn new(\n        screen: Arc<dyn Screen + Send + Sync>,\n        keyboard: Arc<dyn Keyboard + Send + Sync>,\n    ) -> Self {",
		"impl TuiPorts for BoardControllerImpl {",
	} {
		if !strings.Contains(controller, want) {
			t.Fatalf("the controller never carried %q:\n%s", want, controller)
		}
	}
}

func TestThePortsAreAutomockedTraitsThatNameNoTerminalCrate(t *testing.T) {
	files := generate(t, boardSpec, tuirust.Options{Service: "songe-tui"})

	screen := files["port/zz_generated_screen.rs"].Content
	keyboard := files["port/zz_generated_keyboard.rs"].Content

	for _, want := range []string{
		"#[cfg_attr(test, mockall::automock)]",
		"pub trait Screen: Send + Sync {",
		"fn enter(&self) -> Result<(), ScreenError>;",
		"fn draw(&self, frame: &Frame, prompt: &Prompt) -> Result<(), ScreenError>;",
		"fn leave(&self) -> Result<(), ScreenError>;",
	} {
		if !strings.Contains(screen, want) {
			t.Fatalf("the screen port never carried %q:\n%s", want, screen)
		}
	}

	for _, want := range []string{
		"#[cfg_attr(test, mockall::automock)]",
		"pub trait Keyboard: Send + Sync {",
		"fn read(&self, timeout: Duration) -> Result<Option<Input>, KeyboardError>;",
	} {
		if !strings.Contains(keyboard, want) {
			t.Fatalf("the keyboard port never carried %q:\n%s", want, keyboard)
		}
	}

	for _, layer := range []string{"port", "controller", "types", "driver"} {
		for path, file := range files {
			if strings.HasPrefix(path, layer+"/") && strings.Contains(file.Content, "crossterm") {
				t.Fatalf("%s names crossterm, only an adapter may", path)
			}
		}
	}
}

func TestTheDriverBindsTheGridSizeTheTickAndTheViKeysFromTheSpec(t *testing.T) {
	files := generate(t, boardSpec, tuirust.Options{Service: "songe-tui"})

	driver := files["driver/zz_generated_tui_driver.rs"].Content

	for _, want := range []string{
		"pub const WIDTH: u16 = 12;",
		"pub const HEIGHT: u16 = 6;",
		"pub const DEFAULT_TICK_MS: i64 = 100;",
		"Input::Char('h') | Input::Left => Some(Key::Left),",
		"Input::Char('j') | Input::Down => Some(Key::Down),",
		"Input::Char('k') | Input::Up => Some(Key::Up),",
		"Input::Char('l') | Input::Right => Some(Key::Right),",
		"Input::Char('a') => Some(Key::Act),",
		"Input::Char(' ') => Some(Key::EndTurn),",
		"Input::Char('q') => Some(Key::Quit),",
		"matches!(input, Input::Enter)",
		"pub async fn bind(&mut self) -> Result<(), TuiDriverError> {",
		"pub fn announce(&self) -> Result<(), TuiDriverError> {",
		"pub async fn serve(self) -> Result<(), TuiDriverError> {",
		"Some(Input::Interrupt) => Next::Quit,",
		"impl Drop for TuiDriver {",
		"tokio::task::spawn_blocking(move || {",
		"pub fn error_chain(error: &dyn std::error::Error) -> String {",
		"if self.config.tick_ms < 1 {",
		"println!(\"TUI {WIDTH}x{HEIGHT}\");",
	} {
		if !strings.Contains(driver, want) {
			t.Fatalf("the driver never carried %q:\n%s", want, driver)
		}
	}
}

func TestTheKeyMapFollowsTheSpecWhenItRebindsAnAction(t *testing.T) {
	const rebound = `controller: board
width: 4
height: 4
tickMs: 50
keys:
  quit: escape
  line: ":"
  act: "'"
`

	files := generate(t, rebound, tuirust.Options{Service: "songe-tui"})

	driver := files["driver/zz_generated_tui_driver.rs"].Content

	for _, want := range []string{
		"Input::Escape => Some(Key::Quit),",
		"matches!(input, Input::Char(':'))",
		"Input::Char('\\'') => Some(Key::Act),",
		"Input::Char('h') | Input::Left => Some(Key::Left),",
	} {
		if !strings.Contains(driver, want) {
			t.Fatalf("the driver never carried %q:\n%s", want, driver)
		}
	}
}

func TestTheAdaptersWriteToStdoutThroughCrosstermAndRestoreTheTerminal(t *testing.T) {
	files := generate(t, boardSpec, tuirust.Options{Service: "songe-tui"})

	screen := files["adapter/zz_generated_crossterm_screen.rs"].Content
	keyboard := files["adapter/zz_generated_crossterm_keyboard.rs"].Content

	for _, want := range []string{
		"terminal::enable_raw_mode().map_err(entering)?;",
		"if let Err(source) = execute!(stdout(), EnterAlternateScreen, cursor::Hide) {\n            terminal::disable_raw_mode().map_err(entering)?;",
		"execute!(stdout(), cursor::Show, LeaveAlternateScreen).map_err(leaving)?;",
		"Print(text),\n                cursor::Show\n            )",
		"Prompt::Closed => queue!(out, cursor::Hide).map_err(&failing)?,",
		"terminal::disable_raw_mode().map_err(leaving)",
		"pub struct CrosstermScreenConfig {}",
		"pub fn new(config: CrosstermScreenConfig) -> Self {",
	} {
		if !strings.Contains(screen, want) {
			t.Fatalf("the screen adapter never carried %q:\n%s", want, screen)
		}
	}

	for _, want := range []string{
		"if !event::poll(timeout).map_err(&failing)? {",
		"Event::Key(key) if key.kind != KeyEventKind::Release => Ok(Some(input_of(key))),",
		"key.modifiers.contains(KeyModifiers::CONTROL) && key.code == KeyCode::Char('c')",
		"pub struct CrosstermKeyboardConfig {}",
	} {
		if !strings.Contains(keyboard, want) {
			t.Fatalf("the keyboard adapter never carried %q:\n%s", want, keyboard)
		}
	}
}

func TestEveryLayerAndTheCellCarryAGeneratedModFile(t *testing.T) {
	files := generate(t, boardSpec, tuirust.Options{Service: "songe-tui"})

	if !strings.Contains(files["mod.rs"].Content, "pub mod adapter;\npub mod controller;\npub mod driver;\npub mod port;\npub mod types;") {
		t.Fatalf("the cell mod file lists the wrong layers:\n%s", files["mod.rs"].Content)
	}

	if !strings.Contains(files["controller/mod.rs"].Content, "mod board_controller;") {
		t.Fatalf("the controller mod file never mounts the user impl file:\n%s", files["controller/mod.rs"].Content)
	}

	if !strings.Contains(files["controller/mod.rs"].Content, "pub use zz_generated_board_controller::{\n    BoardController, BoardControllerError, BoardControllerImpl, TuiPorts,\n};") {
		t.Fatalf("the controller mod file never exports the trait, the error, the impl and the ports:\n%s", files["controller/mod.rs"].Content)
	}

	for _, layer := range []string{"adapter", "driver"} {
		if !strings.Contains(files[layer+"/mod.rs"].Content, "#![allow(clippy::disallowed_methods, clippy::disallowed_types)]") {
			t.Fatalf("the %s mod file never allows the io lint table:\n%s", layer, files[layer+"/mod.rs"].Content)
		}
	}

	for _, layer := range []string{"controller", "port", "types"} {
		if strings.Contains(files[layer+"/mod.rs"].Content, "#![allow(") {
			t.Fatalf("the %s mod file allows the io lint table:\n%s", layer, files[layer+"/mod.rs"].Content)
		}
	}
}

func TestTheCellDefaultsToTuiAndTheMountPointsFollowIt(t *testing.T) {
	files := generate(t, boardSpec, tuirust.Options{Service: "songe-tui"})

	if !strings.Contains(files["driver/zz_generated_tui_driver.rs"].Content, "use crate::tui::controller::{BoardController, BoardControllerError};") {
		t.Fatal("the driver never reached the controller through the default cell")
	}

	named := generate(t, boardSpec, tuirust.Options{Service: "songe-tui", Cell: "screen"})

	if !strings.Contains(named["driver/zz_generated_screen_driver.rs"].Content, "use crate::screen::controller::{BoardController, BoardControllerError};") {
		t.Fatal("the driver ignored the named cell")
	}

	if !strings.Contains(named["controller/zz_generated_board_controller.rs"].Content, "pub trait ScreenPorts: Send + Sync {") {
		t.Fatal("the ports trait ignored the named cell")
	}
}

func TestGeneratingRefusesAnInputItCannotSpell(t *testing.T) {
	cases := []struct {
		name string
		doc  string
		opts tuirust.Options
		want string
	}{
		{
			name: "no service name",
			doc:  boardSpec,
			opts: tuirust.Options{},
			want: "the service name is required",
		},
		{
			name: "a cell rust cannot spell",
			doc:  boardSpec,
			opts: tuirust.Options{Service: "songe-tui", Cell: "Tui Cell"},
			want: "is not a name Rust can spell as a module",
		},
		{
			name: "a cell that is a rust keyword",
			doc:  boardSpec,
			opts: tuirust.Options{Service: "songe-tui", Cell: "type"},
			want: "is not a name Rust can spell as a module",
		},
		{
			name: "no controller",
			doc:  "width: 12\nheight: 6\ntickMs: 100\n",
			opts: tuirust.Options{Service: "songe-tui"},
			want: "controller is empty",
		},
		{
			name: "a width of zero",
			doc:  "controller: board\nwidth: 0\nheight: 6\ntickMs: 100\n",
			opts: tuirust.Options{Service: "songe-tui"},
			want: "width 0 is outside 1 to 200",
		},
		{
			name: "a height past the limit",
			doc:  "controller: board\nwidth: 12\nheight: 201\ntickMs: 100\n",
			opts: tuirust.Options{Service: "songe-tui"},
			want: "height 201 is outside 1 to 200",
		},
		{
			name: "a tick of zero",
			doc:  "controller: board\nwidth: 12\nheight: 6\ntickMs: 0\n",
			opts: tuirust.Options{Service: "songe-tui"},
			want: "tickMs 0 is outside 1 to 60000",
		},
		{
			name: "two actions on one key",
			doc:  "controller: board\nwidth: 12\nheight: 6\ntickMs: 100\nkeys:\n  quit: h\n",
			opts: tuirust.Options{Service: "songe-tui"},
			want: `keys.left and keys.quit both bind "h"`,
		},
		{
			name: "a key that is not one character",
			doc:  "controller: board\nwidth: 12\nheight: 6\ntickMs: 100\nkeys:\n  act: ctrl-a\n",
			opts: tuirust.Options{Service: "songe-tui"},
			want: `keys.act: "ctrl-a" is not one character and not one of space, enter, escape`,
		},
		{
			name: "an action that does not exist",
			doc:  "controller: board\nwidth: 12\nheight: 6\ntickMs: 100\nkeys:\n  jump: x\n",
			opts: tuirust.Options{Service: "songe-tui"},
			want: "keys.jump names no action",
		},
		{
			name: "a key the spec does not know",
			doc:  "controller: board\nwidth: 12\nheight: 6\ntickMs: 100\ncolor: red\n",
			opts: tuirust.Options{Service: "songe-tui"},
			want: "reading the tui spec",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tuirust.Generate([]byte(tc.doc), tc.opts)
			if err == nil {
				t.Fatal("generating was accepted")
			}

			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("generating reported %q, want it to name %q", err, tc.want)
			}
		})
	}
}

func TestTheSpecFillsEveryUnboundActionWithItsViDefault(t *testing.T) {
	spec, err := tuirust.ParseSpec([]byte("controller: board\nwidth: 12\nheight: 6\ntickMs: 100\nkeys:\n  quit: x\n"))
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	want := map[string]string{
		"left": "h", "down": "j", "up": "k", "right": "l", "act": "a", "endTurn": "space", "quit": "x", "line": "enter",
	}

	if !reflect.DeepEqual(spec.Keys, want) {
		t.Fatalf("keys = %v, want %v", spec.Keys, want)
	}
}
