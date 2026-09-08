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

package authzgen

import (
	"fmt"
	"strings"
)

const (
	unvisited = iota
	onStack
	settled
)

type computedType struct {
	name      string
	relations []string
	edges     map[string][]string
}

func (m Module) validateCycles() error {
	for _, t := range m.computedTypes() {
		if err := t.checkCycles(); err != nil {
			return err
		}
	}

	return nil
}

func (m Module) computedTypes() []computedType {
	order := []string{}
	byName := map[string]*computedType{}

	declared := make([]Type, 0, len(m.Types))
	declared = append(declared, m.Types...)

	for _, e := range m.Extends {
		declared = append(declared, e.Types...)
	}

	for _, t := range declared {
		c := byName[t.Name]
		if c == nil {
			c = &computedType{name: t.Name, edges: map[string][]string{}}
			byName[t.Name] = c
			order = append(order, t.Name)
		}

		for _, r := range t.Relations {
			c.relations = append(c.relations, r.Name)
			c.edges[r.Name] = computedRelations(r.Expression)
		}
	}

	out := make([]computedType, 0, len(order))
	for _, name := range order {
		out = append(out, *byName[name])
	}

	return out
}

func computedRelations(expression string) []string {
	if strings.TrimSpace(expression) == "" {
		return nil
	}

	e, err := parseExpression(expression)
	if err != nil {
		return nil
	}

	names := []string{}

	_ = e.walk(func(t term) error {
		if t.tupleset == "" {
			names = append(names, t.relation)
		}

		return nil
	})

	return names
}

func (t computedType) checkCycles() error {
	w := &cycleWalk{edges: t.edges, state: map[string]int{}}

	for _, relation := range t.relations {
		cycle := w.visit(relation)
		if cycle == nil {
			continue
		}

		return fmt.Errorf(
			"type %q relation %q is computed from itself, the cycle is %s, break it with direct subjects or a relation from another type",
			t.name, cycle[0], strings.Join(cycle, " -> "),
		)
	}

	return nil
}

type cycleWalk struct {
	edges map[string][]string
	state map[string]int
	stack []string
}

func (w *cycleWalk) visit(relation string) []string {
	switch w.state[relation] {
	case onStack:
		return append(w.stackFrom(relation), relation)
	case settled:
		return nil
	}

	w.state[relation] = onStack
	w.stack = append(w.stack, relation)

	for _, next := range w.edges[relation] {
		if cycle := w.visit(next); cycle != nil {
			return cycle
		}
	}

	w.stack = w.stack[:len(w.stack)-1]
	w.state[relation] = settled

	return nil
}

func (w *cycleWalk) stackFrom(relation string) []string {
	for i, name := range w.stack {
		if name == relation {
			return append([]string{}, w.stack[i:]...)
		}
	}

	return append([]string{}, w.stack...)
}
