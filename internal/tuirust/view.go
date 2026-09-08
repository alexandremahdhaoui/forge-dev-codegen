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

package tuirust

import (
	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/rustname"
)

type bindingView struct {
	Variant string
	Pattern string
}

type cellView struct {
	Header           string
	Cell             string
	CratePath        string
	ModulePrefix     string
	Width            int
	Height           int
	TickMs           int
	ControllerSnake  string
	ControllerTrait  string
	ControllerError  string
	ControllerImpl   string
	ControllerModule string
	PortsTrait       string
	DriverStruct     string
	DriverConfig     string
	DriverError      string
	DriverModule     string
	DriverName       string
	ScreenAdapter    string
	ScreenConfig     string
	ScreenModule     string
	ScreenName       string
	KeyboardAdapter  string
	KeyboardConfig   string
	KeyboardModule   string
	KeyboardName     string
	Bindings         []bindingView
	LinePattern      string
}

var keyVariants = map[string]string{
	"left":    "Left",
	"down":    "Down",
	"up":      "Up",
	"right":   "Right",
	"act":     "Act",
	"endTurn": "EndTurn",
	"quit":    "Quit",
}

func buildCellView(spec Spec, opts Options) (cellView, error) {
	cellPascal := rustname.Pascal(opts.Cell)
	controllerPascal := rustname.Pascal(spec.Controller)
	controllerSnake := rustname.Snake(spec.Controller)

	v := cellView{
		Header:           header,
		Cell:             opts.Cell,
		CratePath:        "crate::" + opts.Cell + "::",
		ModulePrefix:     opts.Cell + "::",
		Width:            spec.Width,
		Height:           spec.Height,
		TickMs:           spec.TickMs,
		ControllerSnake:  controllerSnake,
		ControllerTrait:  controllerPascal + "Controller",
		ControllerError:  controllerPascal + "ControllerError",
		ControllerImpl:   controllerPascal + "ControllerImpl",
		ControllerModule: controllerSnake + "_controller",
		PortsTrait:       cellPascal + "Ports",
		DriverStruct:     cellPascal + "Driver",
		DriverConfig:     cellPascal + "DriverConfig",
		DriverError:      cellPascal + "DriverError",
		DriverModule:     opts.Cell + "_driver",
		DriverName:       opts.Cell,
		ScreenAdapter:    "CrosstermScreen",
		ScreenConfig:     "CrosstermScreenConfig",
		ScreenModule:     "crossterm_screen",
		ScreenName:       "crossterm_screen",
		KeyboardAdapter:  "CrosstermKeyboard",
		KeyboardConfig:   "CrosstermKeyboardConfig",
		KeyboardModule:   "crossterm_keyboard",
		KeyboardName:     "crossterm_keyboard",
	}

	for _, action := range Actions {
		pattern, err := InputPattern(spec.Keys[action])
		if err != nil {
			return cellView{}, err
		}

		if action == "line" {
			v.LinePattern = pattern

			continue
		}

		v.Bindings = append(v.Bindings, bindingView{Variant: keyVariants[action], Pattern: pattern})
	}

	return v, nil
}
