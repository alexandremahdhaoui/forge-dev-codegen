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
	"regexp"
	"strings"

	"sigs.k8s.io/yaml"
)

var ident = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

var parameterTypes = map[string]bool{
	"bool": true, "string": true, "int": true, "uint": true,
	"double": true, "duration": true, "timestamp": true, "ipaddress": true,
}

const parameterTypeList = "bool, string, int, uint, double, duration, timestamp, ipaddress, list<T> or map<T>"

func ParseSpec(doc []byte) (Module, error) {
	var m Module

	if err := checkDocumentShape(doc); err != nil {
		return Module{}, err
	}

	if err := yaml.UnmarshalStrict(doc, &m); err != nil {
		return Module{}, fmt.Errorf("reading %s: %w", specFile, err)
	}

	if err := m.validate(); err != nil {
		return Module{}, fmt.Errorf("reading %s: %w", specFile, err)
	}

	return m, nil
}

type relationScope struct {
	subjectTypes  []string
	subjectsKnown bool
}

type typeScope struct {
	local     bool
	relations map[string]relationScope
}

type scope map[string]*typeScope

func (s scope) add(name string, local bool) *typeScope {
	if s[name] == nil {
		s[name] = &typeScope{relations: map[string]relationScope{}}
	}

	s[name].local = s[name].local || local

	return s[name]
}

func (s scope) hasRelation(typeName, relation string) bool {
	ts := s[typeName]
	if ts == nil {
		return false
	}

	_, known := ts.relations[relation]

	return known
}

func (m Module) validate() error {
	if !ident.MatchString(m.Name) {
		return fmt.Errorf("module %q is not a lower case identifier, use lower case letters, digits and underscores and start with a letter", m.Name)
	}

	if err := m.validateConditions(); err != nil {
		return err
	}

	s, err := m.buildScope()
	if err != nil {
		return err
	}

	for _, t := range m.Types {
		if err := m.validateRelations(s, t); err != nil {
			return err
		}
	}

	for _, e := range m.Extends {
		for _, t := range e.Types {
			if err := m.validateRelations(s, t); err != nil {
				return fmt.Errorf("extending module %q: %w", e.Module, err)
			}
		}
	}

	if err := m.validateCycles(); err != nil {
		return err
	}

	return m.validateVectors(s)
}

func (m Module) conditionNames() map[string]bool {
	names := map[string]bool{}

	for _, c := range m.Conditions {
		names[c.Name] = true
	}

	return names
}

func (m Module) validateConditions() error {
	seen := map[string]bool{}

	for _, c := range m.Conditions {
		if !ident.MatchString(c.Name) {
			return fmt.Errorf("condition %q is not a lower case identifier", c.Name)
		}

		if seen[c.Name] {
			return fmt.Errorf("condition %q is declared twice", c.Name)
		}

		seen[c.Name] = true

		if strings.TrimSpace(c.Expression) == "" {
			return fmt.Errorf("condition %q has no expression, write the CEL expression it evaluates", c.Name)
		}

		if len(c.Parameters) == 0 {
			return fmt.Errorf("condition %q declares no parameter, a condition reads at least one typed parameter", c.Name)
		}

		if err := validateParameters(c); err != nil {
			return err
		}
	}

	return nil
}

func validateParameters(c Condition) error {
	params := map[string]bool{}

	for _, p := range c.Parameters {
		if !ident.MatchString(p.Name) {
			return fmt.Errorf("condition %q parameter %q is not a lower case identifier", c.Name, p.Name)
		}

		if params[p.Name] {
			return fmt.Errorf("condition %q parameter %q is declared twice", c.Name, p.Name)
		}

		params[p.Name] = true

		if !isParameterType(p.Type) {
			return fmt.Errorf(
				"condition %q parameter %q has type %q, use %s",
				c.Name, p.Name, p.Type, parameterTypeList,
			)
		}
	}

	return nil
}

func isParameterType(t string) bool {
	if parameterTypes[t] {
		return true
	}

	for _, wrapper := range []string{"list<", "map<"} {
		if strings.HasPrefix(t, wrapper) && strings.HasSuffix(t, ">") {
			return parameterTypes[strings.TrimSuffix(strings.TrimPrefix(t, wrapper), ">")]
		}
	}

	return false
}

func (m Module) buildScope() (scope, error) {
	s := scope{}

	for _, t := range m.Types {
		if !ident.MatchString(t.Name) {
			return nil, fmt.Errorf("type %q is not a lower case identifier", t.Name)
		}

		if s[t.Name] != nil {
			return nil, fmt.Errorf("type %q is declared twice", t.Name)
		}

		if err := s.add(t.Name, true).addRelations(t); err != nil {
			return nil, err
		}
	}

	for _, r := range m.References {
		if err := m.addReference(s, r); err != nil {
			return nil, err
		}
	}

	for _, e := range m.Extends {
		if err := m.addExtension(s, e); err != nil {
			return nil, err
		}
	}

	return s, nil
}

func (m Module) addReference(s scope, r Reference) error {
	if err := m.checkForeignModule(r.Module, "references"); err != nil {
		return err
	}

	for _, t := range r.Types {
		if !ident.MatchString(t.Name) {
			return fmt.Errorf("references module %q: type %q is not a lower case identifier", r.Module, t.Name)
		}

		if ts := s[t.Name]; ts != nil && ts.local {
			return fmt.Errorf("references module %q: type %q is declared in this module, a referenced type lives in another module", r.Module, t.Name)
		}

		ts := s.add(t.Name, false)

		for _, rel := range t.Relations {
			if !ident.MatchString(rel) {
				return fmt.Errorf("references module %q: type %q relation %q is not a lower case identifier", r.Module, t.Name, rel)
			}

			ts.relations[rel] = relationScope{}
		}
	}

	return nil
}

func (m Module) addExtension(s scope, e Extension) error {
	if err := m.checkForeignModule(e.Module, "extends"); err != nil {
		return err
	}

	for _, t := range e.Types {
		if !ident.MatchString(t.Name) {
			return fmt.Errorf("extends module %q: type %q is not a lower case identifier", e.Module, t.Name)
		}

		if ts := s[t.Name]; ts != nil && ts.local {
			return fmt.Errorf("extends module %q: type %q is declared in this module, an extended type lives in another module", e.Module, t.Name)
		}

		if len(t.Relations) == 0 {
			return fmt.Errorf("extends module %q: type %q adds no relation, an extension adds at least one", e.Module, t.Name)
		}

		if err := s.add(t.Name, false).addRelations(t); err != nil {
			return fmt.Errorf("extends module %q: %w", e.Module, err)
		}
	}

	return nil
}

func (m Module) checkForeignModule(name, section string) error {
	if !ident.MatchString(name) {
		return fmt.Errorf("%s module %q is not a lower case identifier", section, name)
	}

	if name == m.Name {
		return fmt.Errorf("%s module %q is this module, name another module", section, name)
	}

	return nil
}

func (ts *typeScope) addRelations(t Type) error {
	for _, r := range t.Relations {
		if !ident.MatchString(r.Name) {
			return fmt.Errorf("type %q relation %q is not a lower case identifier", t.Name, r.Name)
		}

		if _, dup := ts.relations[r.Name]; dup && ts.local {
			return fmt.Errorf("type %q relation %q is declared twice", t.Name, r.Name)
		}

		if _, dup := ts.relations[r.Name]; dup {
			return fmt.Errorf("type %q relation %q is already a relation of the referenced type, an extension adds new relations only", t.Name, r.Name)
		}

		ts.relations[r.Name] = relationScope{subjectTypes: subjectTypes(r.Subjects), subjectsKnown: true}
	}

	return nil
}

func subjectTypes(subjects []string) []string {
	out := make([]string, 0, len(subjects))

	for _, s := range subjects {
		out = append(out, subjectType(s))
	}

	return out
}

func subjectType(subject string) string {
	name, _, _ := strings.Cut(subject, ":")
	name, _, _ = strings.Cut(name, "#")

	return name
}

func (m Module) validateRelations(s scope, t Type) error {
	conditions := m.conditionNames()

	for _, r := range t.Relations {
		if len(r.Subjects) == 0 && strings.TrimSpace(r.Expression) == "" {
			return fmt.Errorf("type %q relation %q has neither subjects nor an expression, list the subject types it accepts or write the expression it is computed from", t.Name, r.Name)
		}

		if r.Condition != "" && len(r.Subjects) == 0 {
			return fmt.Errorf("type %q relation %q names condition %q but lists no subjects, a condition guards direct subjects", t.Name, r.Name, r.Condition)
		}

		if r.Condition != "" && !conditions[r.Condition] {
			return fmt.Errorf("type %q relation %q names unknown condition %q, declare it under conditions", t.Name, r.Name, r.Condition)
		}

		for _, subject := range r.Subjects {
			if err := validateSubject(s, subject); err != nil {
				return fmt.Errorf("type %q relation %q: %w", t.Name, r.Name, err)
			}
		}

		if r.Expression == "" {
			continue
		}

		if err := validateExpression(s, t.Name, r.Expression); err != nil {
			return fmt.Errorf("type %q relation %q: %w", t.Name, r.Name, err)
		}
	}

	return nil
}

func validateSubject(s scope, subject string) error {
	typeName, rest, hasRest := strings.Cut(subject, ":")
	typeName, relation, hasRelation := strings.Cut(typeName, "#")

	if hasRest && (rest != "*" || hasRelation) {
		return fmt.Errorf("subject %q is not a type, a wildcard type:* or a userset type#relation", subject)
	}

	if s[typeName] == nil || !ident.MatchString(typeName) {
		return fmt.Errorf("subject %q names unknown type %q, declare it under types or name its module under references", subject, typeName)
	}

	if !hasRelation {
		return nil
	}

	if !s.hasRelation(typeName, relation) {
		return fmt.Errorf("subject %q names unknown relation %q on type %q", subject, relation, typeName)
	}

	return nil
}

func validateExpression(s scope, typeName, text string) error {
	e, err := parseExpression(text)
	if err != nil {
		return fmt.Errorf("expression %q: %w", text, err)
	}

	return e.walk(func(t term) error {
		if t.tupleset == "" {
			if !s.hasRelation(typeName, t.relation) {
				return fmt.Errorf("expression %q names unknown relation %q on type %q", text, t.relation, typeName)
			}

			return nil
		}

		return validateTupleset(s, typeName, text, t)
	})
}

func validateTupleset(s scope, typeName, text string, t term) error {
	tupleset, known := s[typeName].relations[t.tupleset]
	if !known {
		return fmt.Errorf("expression %q names unknown relation %q on type %q", text, t.tupleset, typeName)
	}

	if !tupleset.subjectsKnown {
		return nil
	}

	if len(tupleset.subjectTypes) == 0 {
		return fmt.Errorf("expression %q reads %q from %q but %q is computed, a from relation lists direct subjects", text, t.relation, t.tupleset, t.tupleset)
	}

	for _, subject := range tupleset.subjectTypes {
		if s.hasRelation(subject, t.relation) {
			return nil
		}
	}

	return fmt.Errorf("expression %q reads %q from %q but no subject type of %q has relation %q", text, t.relation, t.tupleset, t.tupleset, t.relation)
}

func (m Module) validateVectors(s scope) error {
	conditions := m.conditionNames()
	seen := map[string]bool{}

	for i, c := range m.Vectors {
		if c.Name == "" {
			return fmt.Errorf("vector %d has no name, it names the generated test", i)
		}

		if seen[c.Name] {
			return fmt.Errorf("vector %q is declared twice", c.Name)
		}

		seen[c.Name] = true

		if len(c.Checks) == 0 {
			return fmt.Errorf("vector %q has no checks, a case asserts at least one check", c.Name)
		}

		for _, t := range c.Tuples {
			if err := validateTupleShape(s, t.User, t.Relation, t.Object, true); err != nil {
				return fmt.Errorf("vector %q tuple: %w", c.Name, err)
			}

			if t.Condition != nil && !conditions[t.Condition.Name] {
				return fmt.Errorf("vector %q tuple names unknown condition %q, declare it under conditions", c.Name, t.Condition.Name)
			}
		}

		for _, ch := range c.Checks {
			if err := validateTupleShape(s, ch.User, ch.Relation, ch.Object, false); err != nil {
				return fmt.Errorf("vector %q check: %w", c.Name, err)
			}

			if ch.Expected == nil {
				return fmt.Errorf("vector %q check %s %s %s has no expected, write true or false", c.Name, ch.User, ch.Relation, ch.Object)
			}
		}
	}

	return nil
}

func validateTupleShape(s scope, user, relation, object string, wildcardAllowed bool) error {
	if err := validateUser(s, user, wildcardAllowed); err != nil {
		return err
	}

	objectType, objectId, ok := strings.Cut(object, ":")
	if !ok || objectId == "" || strings.Contains(objectId, "#") || objectId == "*" {
		return fmt.Errorf("object %q is not type:id", object)
	}

	if s[objectType] == nil {
		return fmt.Errorf("object %q names unknown type %q", object, objectType)
	}

	if !s.hasRelation(objectType, relation) {
		return fmt.Errorf("relation %q is not a relation of type %q", relation, objectType)
	}

	return nil
}

func validateUser(s scope, user string, wildcardAllowed bool) error {
	userType, userId, ok := strings.Cut(user, ":")
	if !ok || userId == "" {
		return fmt.Errorf("user %q is not type:id, type:id#relation or type:*", user)
	}

	if userId == "*" && !wildcardAllowed {
		return fmt.Errorf("user %q is a wildcard, a check names one user", user)
	}

	if s[userType] == nil {
		return fmt.Errorf("user %q names unknown type %q", user, userType)
	}

	_, userRelation, hasRelation := strings.Cut(userId, "#")
	if hasRelation && !s.hasRelation(userType, userRelation) {
		return fmt.Errorf("user %q names unknown relation %q on type %q", user, userRelation, userType)
	}

	return nil
}
