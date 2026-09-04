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

package hexrust

import (
	"fmt"
	"sort"
	"strings"

	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/cellmanifest"
	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/rustname"
)

const handAdapterLayer = "adapter::"

type plan struct {
	Header      string
	Crate       string
	Binary      string
	BinaryIdent string
	ConfigType  string
	Cells       []string
	Ports       []portPlan
	Controllers []controllerPlan
	Drivers     []driverPlan
	HandModules []string
	HandConfigs []handConfigPlan
	Imports     []string
	Keys        []specKey
}

type portPlan struct {
	Trait      string
	Var        string
	ConfigVar  string
	Candidates []candidatePlan
	Names      string
}

type candidatePlan struct {
	Name       string
	Type       string
	ConfigType string
	Fallible   bool
	Fields     []fieldPlan
}

type fieldPlan struct {
	Name string
	Expr string
}

type controllerPlan struct {
	Trait    string
	Impl     string
	Var      string
	Ports    []string
	PortVars []string
}

type driverPlan struct {
	Name        string
	Var         string
	Type        string
	ConfigType  string
	EnabledVar  string
	Fields      []fieldPlan
	Controllers []string
}

type handConfigPlan struct {
	Module  string
	Adapter string
	Type    string
	Fields  []specField
}

type specField struct {
	Ident    string
	RustType string
}

type specKey struct {
	Key         string
	Type        string
	Default     any
	Description string
}

func camel(snake string) string {
	pascal := rustname.Pascal(snake)
	if pascal == "" {
		return pascal
	}

	return strings.ToLower(pascal[:1]) + pascal[1:]
}

func rustType(fieldType string) string {
	switch fieldType {
	case cellmanifest.FieldTypeInteger, cellmanifest.FieldTypeDuration:
		return "i64"
	case cellmanifest.FieldTypeBoolean:
		return "bool"
	default:
		return "String"
	}
}

func specType(fieldType string) string {
	switch fieldType {
	case cellmanifest.FieldTypeInteger, cellmanifest.FieldTypeDuration:
		return "integer"
	case cellmanifest.FieldTypeBoolean:
		return "boolean"
	default:
		return "string"
	}
}

func readExpr(configField, fieldType string) string {
	if rustType(fieldType) == "String" {
		return "config." + configField + ".clone()"
	}

	return "config." + configField
}

func sortedFieldNames(config map[string]cellmanifest.ConfigField) []string {
	names := make([]string, 0, len(config))
	for name := range config {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

func list(values []string) string {
	return strings.Join(values, ", ")
}

func buildPlan(merged cellmanifest.Merged, wiring Wiring, opts Options) (plan, error) {
	p := plan{
		Header:      header,
		Crate:       rustname.Snake(opts.Service),
		Binary:      wiring.Binary,
		BinaryIdent: rustname.Snake(wiring.Binary),
		ConfigType:  rustname.Pascal(wiring.Binary) + "Config",
	}

	p.Cells = append(p.Cells, opts.Cells...)
	sort.Strings(p.Cells)

	if err := checkRequiredPorts(merged); err != nil {
		return plan{}, err
	}

	imports := map[string]bool{}

	if err := planDrivers(&p, merged, wiring, imports); err != nil {
		return plan{}, err
	}

	if err := planControllers(&p, merged, wiring, imports); err != nil {
		return plan{}, err
	}

	if err := planPorts(&p, merged, wiring, imports); err != nil {
		return plan{}, err
	}

	p.Imports = sortedKeys(imports)

	sort.Slice(p.Keys, func(i, j int) bool { return p.Keys[i].Key < p.Keys[j].Key })

	return p, nil
}

func checkRequiredPorts(merged cellmanifest.Merged) error {
	for _, required := range merged.RequiredPorts {
		if _, provided := merged.Ports[required.Trait]; !provided {
			return fmt.Errorf(
				"wiring the ports: cell %q requires port %q and no cell manifest declares that port trait",
				required.Cell, required.Trait,
			)
		}
	}

	return nil
}

func planDrivers(p *plan, merged cellmanifest.Merged, wiring Wiring, imports map[string]bool) error {
	provided := sortedKeys(keysOfDrivers(merged.Drivers))

	for _, name := range sortedDriverNames(wiring.Drivers) {
		entry, taken := merged.Drivers[name]
		if !taken {
			return fmt.Errorf(
				"wiring driver %q: no cell manifest provides a driver with that name, the cells provide %s",
				name, list(provided),
			)
		}

		for _, trait := range entry.Driver.Requires {
			if _, provides := merged.Controllers[trait]; !provides {
				return fmt.Errorf(
					"wiring driver %q: it requires controller %q and no cell manifest provides it",
					name, trait,
				)
			}
		}

		dp := driverPlan{
			Name:       name,
			Var:        rustname.Snake(name) + "_driver",
			Type:       entry.Driver.Type,
			ConfigType: entry.Driver.Type + "Config",
			EnabledVar: "driver_" + rustname.Snake(name),
		}

		for _, field := range sortedFieldNames(entry.Driver.Config) {
			declared := entry.Driver.Config[field]
			key := camel(name) + rustname.Pascal(field)

			dp.Fields = append(dp.Fields, fieldPlan{
				Name: field,
				Expr: readExpr(rustname.Snake(key), declared.Type),
			})

			p.Keys = append(p.Keys, specKey{
				Key:         key,
				Type:        specType(declared.Type),
				Default:     declared.Default,
				Description: declared.Description,
			})
		}

		for _, trait := range entry.Driver.Requires {
			dp.Controllers = append(dp.Controllers, rustname.Snake(trait))
		}

		p.Keys = append(p.Keys, specKey{
			Key:         "driver" + rustname.Pascal(name),
			Type:        "boolean",
			Default:     wiring.Drivers[name].Enabled,
			Description: "Whether the " + name + " driver starts",
		})

		imports[p.Crate+"::"+entry.Driver.Module+"::{"+dp.Type+", "+dp.ConfigType+"}"] = true

		p.Drivers = append(p.Drivers, dp)
	}

	for _, name := range provided {
		if _, wired := wiring.Drivers[name]; !wired {
			return fmt.Errorf(
				"wiring the drivers: cell %q provides driver %q and the wiring never names it",
				merged.Drivers[name].Cell, name,
			)
		}
	}

	return nil
}

func planControllers(p *plan, merged cellmanifest.Merged, wiring Wiring, imports map[string]bool) error {
	needed := map[string]bool{}

	for _, d := range p.Drivers {
		for _, entry := range merged.Drivers[d.Name].Driver.Requires {
			needed[entry] = true
		}
	}

	for _, trait := range sortedKeys(needed) {
		entry := merged.Controllers[trait]

		cp := controllerPlan{
			Trait: trait,
			Impl:  entry.Controller.Impl,
			Var:   rustname.Snake(trait),
			Ports: entry.Controller.Ports,
		}

		for _, port := range entry.Controller.Ports {
			cp.PortVars = append(cp.PortVars, rustname.Snake(port))
		}

		imports[p.Crate+"::"+entry.Controller.Module+"::{"+trait+", "+entry.Controller.Impl+"}"] = true

		p.Controllers = append(p.Controllers, cp)
	}

	return nil
}

func planPorts(p *plan, merged cellmanifest.Merged, wiring Wiring, imports map[string]bool) error {
	consumed := map[string][]string{}

	for _, c := range p.Controllers {
		for _, trait := range c.Ports {
			consumed[trait] = append(consumed[trait], c.Trait)
		}
	}

	byName := map[string]cellmanifest.AdapterEntry{}
	for _, entry := range merged.Adapters {
		byName[entry.Adapter.Name] = entry
	}

	for _, trait := range sortedKeys(consumed) {
		block, wired := wiring.Ports[trait]
		if !wired {
			return fmt.Errorf(
				"wiring the ports: controller %q consumes port %q and the wiring names no candidate for it",
				consumed[trait][0], trait,
			)
		}

		portEntry, provided := merged.Ports[trait]
		if !provided {
			return fmt.Errorf(
				"wiring port %q: no cell manifest declares that port trait",
				trait,
			)
		}

		pp := portPlan{
			Trait:     trait,
			Var:       rustname.Snake(trait),
			ConfigVar: rustname.Snake(camel(rustname.Snake(trait))),
			Names:     list(candidateNames(block)),
		}

		imports[p.Crate+"::"+portEntry.Port.Module+"::"+trait] = true

		for _, name := range candidateNames(block) {
			cp, err := planCandidate(p, trait, name, block.Adapters[name], byName, imports)
			if err != nil {
				return err
			}

			pp.Candidates = append(pp.Candidates, cp)
		}

		p.Keys = append(p.Keys, specKey{
			Key:         camel(rustname.Snake(trait)),
			Type:        "string",
			Default:     block.Default,
			Description: "Which " + trait + " adapter to build, one of " + pp.Names,
		})

		p.Ports = append(p.Ports, pp)
	}

	return nil
}

func planCandidate(
	p *plan,
	trait, name string,
	candidate WiringCandidate,
	byName map[string]cellmanifest.AdapterEntry,
	imports map[string]bool,
) (candidatePlan, error) {
	portKey := camel(rustname.Snake(trait))

	if candidate.Type == "" {
		entry, provided := byName[name]
		if !provided {
			return candidatePlan{}, fmt.Errorf(
				"wiring port %q: candidate %q declares no type and no cell manifest provides an adapter named %q",
				trait, name, name,
			)
		}

		if entry.Adapter.Implements != trait {
			return candidatePlan{}, fmt.Errorf(
				"wiring port %q: candidate %q implements %q and the wiring names it under %q",
				trait, name, entry.Adapter.Implements, trait,
			)
		}

		cp := candidatePlan{
			Name:       name,
			Type:       entry.Adapter.Type,
			ConfigType: entry.Adapter.Type + "Config",
			Fallible:   entry.Adapter.Fallible,
		}

		imports[p.Crate+"::"+entry.Adapter.Module+"::{"+cp.Type+", "+cp.ConfigType+"}"] = true

		for _, field := range sortedFieldNames(entry.Adapter.Config) {
			declared := entry.Adapter.Config[field]
			key := portKey + rustname.Pascal(name) + rustname.Pascal(field)

			cp.Fields = append(cp.Fields, fieldPlan{
				Name: field,
				Expr: readExpr(rustname.Snake(key), declared.Type),
			})

			p.Keys = append(p.Keys, specKey{
				Key:         key,
				Type:        specType(declared.Type),
				Default:     declared.Default,
				Description: declared.Description,
			})
		}

		return cp, nil
	}

	if !strings.HasPrefix(candidate.Module, handAdapterLayer) {
		return candidatePlan{}, fmt.Errorf(
			"wiring port %q: candidate %q has module %q, a hand written adapter lives under the adapter layer",
			trait, name, candidate.Module,
		)
	}

	module := strings.TrimPrefix(candidate.Module, handAdapterLayer)
	if !rustname.IsModuleName(module) {
		return candidatePlan{}, fmt.Errorf(
			"wiring port %q: candidate %q has module %q, which is not a name Rust can spell as a module",
			trait, name, candidate.Module,
		)
	}

	cp := candidatePlan{
		Name:       name,
		Type:       candidate.Type,
		ConfigType: candidate.Type + "Config",
		Fallible:   candidate.Fallible,
	}

	hand := handConfigPlan{Module: module, Adapter: cp.Type, Type: cp.ConfigType}

	for _, field := range sortedFieldNames(candidate.Config) {
		declared := candidate.Config[field]

		if err := checkFieldType(trait, name, field, declared.Type); err != nil {
			return candidatePlan{}, err
		}

		key := portKey + rustname.Pascal(name) + rustname.Pascal(field)

		cp.Fields = append(cp.Fields, fieldPlan{
			Name: field,
			Expr: readExpr(rustname.Snake(key), declared.Type),
		})

		hand.Fields = append(hand.Fields, specField{Ident: field, RustType: rustType(declared.Type)})

		p.Keys = append(p.Keys, specKey{
			Key:         key,
			Type:        specType(declared.Type),
			Default:     declared.Default,
			Description: declared.Description,
		})
	}

	imports[p.Crate+"::adapter::{"+cp.Type+", "+cp.ConfigType+"}"] = true

	p.HandModules = append(p.HandModules, module)
	p.HandConfigs = append(p.HandConfigs, hand)

	return cp, nil
}

func checkFieldType(trait, name, field, fieldType string) error {
	for _, known := range cellmanifest.FieldTypes() {
		if fieldType == known {
			return nil
		}
	}

	return fmt.Errorf(
		"wiring port %q: candidate %q field %q has type %q which is not one of %s",
		trait, name, field, fieldType, list(cellmanifest.FieldTypes()),
	)
}

func keysOfDrivers(drivers map[string]cellmanifest.DriverEntry) map[string]bool {
	out := map[string]bool{}
	for name := range drivers {
		out[name] = true
	}

	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}
