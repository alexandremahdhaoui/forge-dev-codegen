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
)

type view struct {
	Header       string
	Service      string
	ServiceUpper string
	CoreCrate    string
	AppCrate     string
	Types        []typeView
	Stores       []storeView
	UsedStores   []storeView
	Controllers  []controllerView
	Routes       []opView
	WireImports  []string
	UsesPath     bool
}

type fieldView struct {
	Ident    string
	Name     string
	Renamed  bool
	Optional bool
	CoreType string
	WireType string
	ToCore   string
	ToWire   string
}

type typeView struct {
	Name   string
	Snake  string
	Fields []fieldView
}

type storeView struct {
	Name      string
	Snake     string
	Upper     string
	Port      string
	PortSnake string
	Generic   string
}

type paramView struct {
	Ident    string
	CoreType string
	ArgType  string
}

type importView struct {
	Snake string
	Name  string
}

type opView struct {
	Ident             string
	Method            string
	MethodLower       string
	Path              string
	Params            []paramView
	Body              string
	Response          string
	ControllerSnake   string
	ControllerPascal  string
	Ports             []storeView
	TraitArgs         string
	ReturnType        string
	HandGenerics      string
	HandParams        string
	HandCall          string
	Unused            string
	Extractors        string
	ControllerCall    string
	StatusExpr        string
	InvalidStatusExpr string
	HandlerReturn     string
	OkExpr            string
}

type controllerView struct {
	Name        string
	Snake       string
	Pascal      string
	Ports       []storeView
	Ops         []opView
	Generics    string
	GenericArgs string
	TypeImports []importView
}

func buildView(spec *Spec, opts Options) view {
	v := view{
		Header:       header,
		Service:      opts.Service,
		ServiceUpper: Upper(opts.Service),
		CoreCrate:    Snake(opts.Service) + "_core",
		AppCrate:     Snake(opts.Service) + "_app",
	}

	for _, t := range spec.Types {
		v.Types = append(v.Types, buildTypeView(t))
	}

	storesByPort := map[string]storeView{}

	for _, s := range spec.Stores {
		sv := storeView{
			Name:      s.Name,
			Snake:     s.Snake,
			Upper:     Upper(s.Name),
			Port:      s.Name + "Store",
			PortSnake: s.Snake + "_store",
			Generic:   s.Name + "StorePort",
		}
		v.Stores = append(v.Stores, sv)
		storesByPort[sv.Port] = sv
	}

	used := map[string]bool{}

	for _, c := range spec.Controllers {
		cv := buildControllerView(c, storesByPort)
		v.Controllers = append(v.Controllers, cv)
		v.Routes = append(v.Routes, cv.Ops...)

		for _, p := range c.Ports {
			used[p] = true
		}
	}

	for _, s := range v.Stores {
		if used[s.Port] {
			v.UsedStores = append(v.UsedStores, s)
		}
	}

	sort.Slice(v.Routes, func(i, j int) bool {
		if v.Routes[i].Path != v.Routes[j].Path {
			return v.Routes[i].Path < v.Routes[j].Path
		}

		return v.Routes[i].Method < v.Routes[j].Method
	})

	wire := map[string]bool{}

	for _, r := range v.Routes {
		if r.Body != "" {
			wire[r.Body] = true
		}

		if r.Response != "" {
			wire[r.Response] = true
		}

		if len(r.Params) > 0 {
			v.UsesPath = true
		}
	}

	v.WireImports = sortedKeys(wire)

	return v
}

func buildTypeView(t TypeDef) typeView {
	tv := typeView{Name: t.Name, Snake: t.Snake}

	for _, f := range t.Fields {
		toCore, _ := convert("w."+f.Ident, f.Type, f.Optional)
		toWire, _ := convert("v."+f.Ident, f.Type, f.Optional)

		tv.Fields = append(tv.Fields, fieldView{
			Ident:    f.Ident,
			Name:     f.Name,
			Renamed:  f.Renamed,
			Optional: f.Optional,
			CoreType: wrapOptional(coreType(f.Type), f.Optional),
			WireType: wrapOptional(wireType(f.Type), f.Optional),
			ToCore:   toCore,
			ToWire:   toWire,
		})
	}

	return tv
}

func coreType(ft fieldType) string {
	switch ft.Kind {
	case "ref":
		return "crate::types::" + Snake(ft.Ref) + "::" + ft.Ref
	case "array":
		return "Vec<" + coreType(*ft.Item) + ">"
	default:
		return scalarType(ft.Kind)
	}
}

func wireType(ft fieldType) string {
	switch ft.Kind {
	case "ref":
		return ft.Ref + "Wire"
	case "array":
		return "Vec<" + wireType(*ft.Item) + ">"
	default:
		return scalarType(ft.Kind)
	}
}

func scalarType(kind string) string {
	switch kind {
	case "string":
		return "String"
	case "integer":
		return "i64"
	case "number":
		return "f64"
	case "boolean":
		return "bool"
	default:
		return "serde_json::Value"
	}
}

func wrapOptional(t string, optional bool) string {
	if optional {
		return "Option<" + t + ">"
	}

	return t
}

func convert(expr string, ft fieldType, optional bool) (string, bool) {
	if optional {
		inner, converts := convert("x", ft, false)
		if !converts {
			return expr, false
		}

		return expr + ".map(|x| " + inner + ")", true
	}

	switch ft.Kind {
	case "ref":
		return expr + ".into()", true
	case "array":
		inner, converts := convert("x", *ft.Item, false)
		if !converts {
			return expr, false
		}

		return expr + ".into_iter().map(|x| " + inner + ").collect()", true
	default:
		return expr, false
	}
}

func buildControllerView(c Controller, storesByPort map[string]storeView) controllerView {
	cv := controllerView{Name: c.Name, Snake: c.Snake, Pascal: c.Pascal}

	for _, p := range c.Ports {
		cv.Ports = append(cv.Ports, storesByPort[p])
	}

	cv.Generics = genericBounds(cv.Ports)
	cv.GenericArgs = genericArgs(cv.Ports)

	imports := map[string]bool{}

	for _, op := range c.Operations {
		cv.Ops = append(cv.Ops, buildOpView(op, c, storesByPort))

		if op.Body != "" {
			imports[op.Body] = true
		}

		if op.Response != "" {
			imports[op.Response] = true
		}
	}

	for _, name := range sortedKeys(imports) {
		cv.TypeImports = append(cv.TypeImports, importView{Snake: Snake(name), Name: name})
	}

	return cv
}

func genericBounds(ports []storeView) string {
	if len(ports) == 0 {
		return ""
	}

	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		parts = append(parts, p.Generic+": "+p.Port)
	}

	return "<" + strings.Join(parts, ", ") + ">"
}

func genericArgs(ports []storeView) string {
	if len(ports) == 0 {
		return ""
	}

	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		parts = append(parts, p.Generic)
	}

	return "<" + strings.Join(parts, ", ") + ">"
}

func buildOpView(op Operation, c Controller, storesByPort map[string]storeView) opView {
	ov := opView{
		Ident:             op.Ident,
		Method:            op.Method,
		MethodLower:       op.MethodLower,
		Path:              op.Path,
		Body:              op.Body,
		Response:          op.Response,
		ControllerSnake:   c.Snake,
		ControllerPascal:  c.Pascal,
		ReturnType:        "()",
		StatusExpr:        statusExpr(op.Status),
		InvalidStatusExpr: statusExpr(op.InvalidStatus),
	}

	if op.Response != "" {
		ov.ReturnType = op.Response
	}

	for _, p := range op.Ports {
		ov.Ports = append(ov.Ports, storesByPort[p])
	}

	traitArgs := []string{}
	handParams := []string{}
	handCall := []string{}
	unused := []string{}
	extractors := []string{"State(driver): State<HttpDriver>"}
	controllerCall := []string{}

	for _, p := range ov.Ports {
		handParams = append(handParams, p.PortSnake+": &"+p.Generic)
		handCall = append(handCall, "&self."+p.PortSnake)
		unused = append(unused, p.PortSnake)
	}

	pathIdents := []string{}
	pathTypes := []string{}

	for _, p := range op.Params {
		pv := paramView{Ident: p.Ident, CoreType: scalarType(p.Kind), ArgType: scalarType(p.Kind)}
		call := p.Ident

		if p.Kind == "string" {
			pv.ArgType = "&str"
			call = "&" + p.Ident
		}

		ov.Params = append(ov.Params, pv)
		traitArgs = append(traitArgs, pv.Ident+": "+pv.ArgType)
		handParams = append(handParams, pv.Ident+": "+pv.ArgType)
		handCall = append(handCall, pv.Ident)
		unused = append(unused, pv.Ident)
		controllerCall = append(controllerCall, call)
		pathIdents = append(pathIdents, pv.Ident)
		pathTypes = append(pathTypes, pv.CoreType)
	}

	switch len(op.Params) {
	case 0:
	case 1:
		extractors = append(extractors, fmt.Sprintf("Path(%s): Path<%s>", pathIdents[0], pathTypes[0]))
	default:
		extractors = append(extractors, fmt.Sprintf("Path((%s)): Path<(%s)>", strings.Join(pathIdents, ", "), strings.Join(pathTypes, ", ")))
	}

	if op.Body != "" {
		traitArgs = append(traitArgs, "body: "+op.Body)
		handParams = append(handParams, "body: "+op.Body)
		handCall = append(handCall, "body")
		unused = append(unused, "body")
		extractors = append(extractors, "Json(body): Json<"+op.Body+"Wire>")
		controllerCall = append(controllerCall, "body.into()")
	}

	ov.TraitArgs = strings.Join(traitArgs, ", ")
	ov.HandGenerics = genericBounds(ov.Ports)
	ov.HandParams = strings.Join(handParams, ", ")
	ov.HandCall = strings.Join(handCall, ", ")
	ov.Unused = unusedExpr(unused)
	ov.Extractors = strings.Join(extractors, ", ")
	ov.ControllerCall = strings.Join(controllerCall, ", ")

	if op.Response == "" {
		ov.HandlerReturn = "Result<StatusCode, Rejection>"
		ov.OkExpr = ov.StatusExpr
	} else {
		ov.HandlerReturn = "Result<(StatusCode, Json<" + op.Response + "Wire>), Rejection>"
		ov.OkExpr = "(" + ov.StatusExpr + ", Json(out.into()))"
	}

	return ov
}

func unusedExpr(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	default:
		return "(" + strings.Join(names, ", ") + ")"
	}
}

func statusExpr(status int) string {
	switch status {
	case 200:
		return "StatusCode::OK"
	case 201:
		return "StatusCode::CREATED"
	case 202:
		return "StatusCode::ACCEPTED"
	case 204:
		return "StatusCode::NO_CONTENT"
	case 400:
		return "StatusCode::BAD_REQUEST"
	case 404:
		return "StatusCode::NOT_FOUND"
	case 422:
		return "StatusCode::UNPROCESSABLE_ENTITY"
	default:
		return fmt.Sprintf("StatusCode::from_u16(%d).unwrap_or(StatusCode::OK)", status)
	}
}
