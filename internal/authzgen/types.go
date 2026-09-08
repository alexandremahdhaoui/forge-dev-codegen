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

type File struct {
	Path    string
	Content string
}

type Module struct {
	Name       string      `json:"module"`
	References []Reference `json:"references,omitempty"`
	Types      []Type      `json:"types,omitempty"`
	Extends    []Extension `json:"extends,omitempty"`
	Conditions []Condition `json:"conditions,omitempty"`
	Vectors    []Case      `json:"vectors,omitempty"`
}

type Reference struct {
	Module string           `json:"module"`
	Types  []ReferencedType `json:"types"`
}

type ReferencedType struct {
	Name      string   `json:"name"`
	Relations []string `json:"relations,omitempty"`
}

type Type struct {
	Name      string     `json:"name"`
	Relations []Relation `json:"relations,omitempty"`
}

type Relation struct {
	Name       string   `json:"name"`
	Subjects   []string `json:"subjects,omitempty"`
	Expression string   `json:"expression,omitempty"`
	Condition  string   `json:"condition,omitempty"`
}

type Extension struct {
	Module string `json:"module"`
	Types  []Type `json:"types"`
}

type Condition struct {
	Name       string      `json:"name"`
	Parameters []Parameter `json:"parameters"`
	Expression string      `json:"expression"`
}

type Parameter struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type Case struct {
	Name   string  `json:"name"`
	Tuples []Tuple `json:"tuples,omitempty"`
	Checks []Check `json:"checks"`
}

type Tuple struct {
	User      string          `json:"user"`
	Relation  string          `json:"relation"`
	Object    string          `json:"object"`
	Condition *TupleCondition `json:"condition,omitempty"`
}

type TupleCondition struct {
	Name    string         `json:"name"`
	Context map[string]any `json:"context,omitempty"`
}

type Check struct {
	User     string         `json:"user"`
	Relation string         `json:"relation"`
	Object   string         `json:"object"`
	Expected *bool          `json:"expected"`
	Context  map[string]any `json:"context,omitempty"`
}
