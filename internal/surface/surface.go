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

// Package surface reads the kind vocabulary out of the opaque surface map
// every generator receives. Shared by every cli engine here, so the cells
// cannot drift on what a command is.
package surface

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// Command is one cli subcommand.
type Command struct {
	Name        string
	GoName      string
	SnakeName   string
	Description string
}

// Commands reads the commands list, sorted by name so generation is
// deterministic.
func Commands(surface map[string]interface{}) ([]Command, error) {
	raw, ok := surface["commands"].([]interface{})
	if !ok || len(raw) == 0 {
		return nil, fmt.Errorf("at least one command is required")
	}

	commands := make([]Command, 0, len(raw))

	for i, entry := range raw {
		m, ok := entry.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("entry %d is not an object", i)
		}

		name, _ := m["name"].(string)
		if name == "" {
			return nil, fmt.Errorf("entry %d has no name", i)
		}

		description, _ := m["description"].(string)

		commands = append(commands, Command{
			Name:        name,
			GoName:      Pascal(name),
			SnakeName:   Snake(name),
			Description: description,
		})
	}

	sort.Slice(commands, func(i, j int) bool { return commands[i].Name < commands[j].Name })

	return commands, nil
}

// Pascal turns run-all into RunAll.
func Pascal(s string) string {
	var b strings.Builder

	up := true

	for _, r := range s {
		switch {
		case r == '-' || r == '_' || r == '.' || r == ' ':
			up = true
		case up:
			b.WriteRune(unicode.ToUpper(r))
			up = false
		default:
			b.WriteRune(r)
		}
	}

	return b.String()
}

// Camel turns run-all into runAll.
func Camel(s string) string {
	p := Pascal(s)
	if p == "" {
		return p
	}

	return strings.ToLower(p[:1]) + p[1:]
}

// Snake turns run-all into run_all.
func Snake(s string) string {
	return strings.NewReplacer("-", "_", ".", "_", " ", "_").Replace(s)
}
