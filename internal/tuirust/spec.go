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
	"fmt"
	"sort"
	"unicode/utf8"

	"sigs.k8s.io/yaml"

	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/rustname"
)

const (
	MaxSide   = 200
	MaxTickMs = 60000
)

var Actions = []string{"left", "down", "up", "right", "act", "endTurn", "quit", "line"}

var DefaultKeys = map[string]string{
	"left":    "h",
	"down":    "j",
	"up":      "k",
	"right":   "l",
	"act":     "a",
	"endTurn": "space",
	"quit":    "q",
	"line":    "enter",
}

var namedKeys = map[string]string{
	"space":  "Input::Char(' ')",
	"enter":  "Input::Enter",
	"escape": "Input::Escape",
}

type Spec struct {
	Controller string            `json:"controller"`
	Width      int               `json:"width"`
	Height     int               `json:"height"`
	TickMs     int               `json:"tickMs"`
	Keys       map[string]string `json:"keys,omitempty"`
}

func ParseSpec(doc []byte) (Spec, error) {
	var s Spec

	if err := yaml.UnmarshalStrict(doc, &s); err != nil {
		return Spec{}, fmt.Errorf("reading the tui spec: %w", err)
	}

	if err := s.validate(); err != nil {
		return Spec{}, fmt.Errorf("reading the tui spec: %w", err)
	}

	if s.Keys == nil {
		s.Keys = map[string]string{}
	}

	for _, action := range Actions {
		if _, bound := s.Keys[action]; !bound {
			s.Keys[action] = DefaultKeys[action]
		}
	}

	if err := s.validateKeys(); err != nil {
		return Spec{}, fmt.Errorf("reading the tui spec: %w", err)
	}

	return s, nil
}

func (s Spec) validate() error {
	if s.Controller == "" {
		return fmt.Errorf("controller is empty, it names the controller the user implements")
	}

	if !rustname.IsModuleName(rustname.Snake(s.Controller)) {
		return fmt.Errorf("controller %q is not a name Rust can spell as a module", s.Controller)
	}

	if s.Width < 1 || s.Width > MaxSide {
		return fmt.Errorf("width %d is outside 1 to %d", s.Width, MaxSide)
	}

	if s.Height < 1 || s.Height > MaxSide {
		return fmt.Errorf("height %d is outside 1 to %d", s.Height, MaxSide)
	}

	if s.TickMs < 1 || s.TickMs > MaxTickMs {
		return fmt.Errorf("tickMs %d is outside 1 to %d", s.TickMs, MaxTickMs)
	}

	return nil
}

func (s Spec) validateKeys() error {
	known := map[string]bool{}
	for _, action := range Actions {
		known[action] = true
	}

	names := make([]string, 0, len(s.Keys))
	for action := range s.Keys {
		names = append(names, action)
	}

	sort.Strings(names)

	boundTo := map[string]string{}

	for _, action := range names {
		if !known[action] {
			return fmt.Errorf("keys.%s names no action, the actions are %v", action, Actions)
		}

		key := s.Keys[action]

		if _, err := InputPattern(key); err != nil {
			return fmt.Errorf("keys.%s: %w", action, err)
		}

		if other, taken := boundTo[key]; taken {
			return fmt.Errorf("keys.%s and keys.%s both bind %q", other, action, key)
		}

		boundTo[key] = action
	}

	return nil
}

func InputPattern(key string) (string, error) {
	if pattern, named := namedKeys[key]; named {
		return pattern, nil
	}

	if utf8.RuneCountInString(key) != 1 {
		return "", fmt.Errorf("%q is not one character and not one of space, enter, escape", key)
	}

	r, _ := utf8.DecodeRuneInString(key)

	if r == '\'' || r == '\\' {
		return fmt.Sprintf("Input::Char('\\%c')", r), nil
	}

	return fmt.Sprintf("Input::Char('%c')", r), nil
}
