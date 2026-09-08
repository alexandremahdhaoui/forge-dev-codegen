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

package restrust

import (
	"fmt"
	"sort"
	"strings"

	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/rustname"
)

type view struct {
	Header           string
	Service          string
	Cell             string
	CratePath        string
	ModulePrefix     string
	DriverName       string
	DefaultAddress   string
	DefaultStorePath string
	DefaultBaseURL   string
	Server           bool
	Client           bool
	Auth             bool
	HasStream        bool
	Types            []typeView
	Stores           []storeView
	UsedStores       []storeView
	Events           []eventView
	Controllers      []controllerView
	Clients          []clientView
	Routes           []opView
	WireImports      []string
	UsesPath         bool
	UsesJson         bool
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
	Name         string
	Snake        string
	Upper        string
	Port         string
	PortSnake    string
	Struct       string
	ConfigStruct string
	AdapterName  string
	Module       string
	DefaultPath  string
}

type eventView struct {
	Name      string
	Snake     string
	Port      string
	PortSnake string
}

type portView struct {
	Port      string
	PortSnake string
	Snake     string
	Reaching  string
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
	ID                string
	Ident             string
	Method            string
	MethodLower       string
	Path              string
	Params            []paramView
	Body              string
	Response          string
	Auth              bool
	Stream            bool
	ControllerSnake   string
	ControllerPascal  string
	Ports             []portView
	TraitArgs         string
	ReturnType        string
	Extractors        string
	ControllerCall    string
	StatusExpr        string
	InvalidStatusExpr string
	HandlerReturn     string
	OkExpr            string
	ClientArgs        string
	ClientReturn      string
	URLExpr           string
}

type controllerView struct {
	Name        string
	Snake       string
	Pascal      string
	Auth        bool
	Ports       []portView
	Ops         []opView
	TypeImports []importView
}

type clientView struct {
	Snake       string
	Pascal      string
	Trait       string
	Error       string
	Struct      string
	Config      string
	Module      string
	PortModule  string
	AdapterName string
	Ops         []opView
	TypeImports []importView
	HasStream   bool
	UsesJson    bool
}

func buildView(spec *Spec, opts Options) view {
	cratePath := "crate::" + opts.Cell + "::"
	modulePrefix := opts.Cell + "::"

	v := view{
		Header:           header,
		Service:          opts.Service,
		Cell:             opts.Cell,
		CratePath:        cratePath,
		ModulePrefix:     modulePrefix,
		DriverName:       opts.Cell,
		DefaultAddress:   DefaultAddress,
		DefaultStorePath: DefaultStorePath,
		DefaultBaseURL:   DefaultBaseURL,
		Server:           opts.Side != SideClient,
		Client:           opts.Side != SideServer,
		Auth:             spec.Auth,
	}

	for _, t := range spec.Types {
		v.Types = append(v.Types, buildTypeView(t))
	}

	portsByName := map[string]portView{}

	for _, s := range spec.Stores {
		adapterName := "sqlite"
		if len(spec.Stores) > 1 {
			adapterName = s.Snake + "_sqlite"
		}

		sv := storeView{
			Name:         s.Name,
			Snake:        s.Snake,
			Upper:        rustname.Upper(s.Name),
			Port:         s.Name + StorePortSuffix,
			PortSnake:    s.Snake + "_store",
			Struct:       s.Name + "SqliteStore",
			ConfigStruct: s.Name + "SqliteStoreConfig",
			AdapterName:  adapterName,
			Module:       s.Snake + "_sqlite",
			DefaultPath:  DefaultStorePath,
		}
		v.Stores = append(v.Stores, sv)
		portsByName[sv.Port] = portView{
			Port:      sv.Port,
			PortSnake: sv.PortSnake,
			Snake:     sv.Snake,
			Reaching:  "reaching " + sv.Snake + " store for",
		}
	}

	for _, e := range spec.Events {
		ev := eventView{
			Name:      e.Name,
			Snake:     e.Snake,
			Port:      e.Name + SubscribePortSuffix,
			PortSnake: e.Snake + "_subscribe",
		}
		v.Events = append(v.Events, ev)
		portsByName[ev.Port] = portView{
			Port:      ev.Port,
			PortSnake: ev.PortSnake,
			Snake:     ev.Snake,
			Reaching:  "subscribing to " + ev.Snake + " events for",
		}
	}

	used := map[string]bool{}

	for _, c := range spec.Controllers {
		cv := buildControllerView(c, portsByName)
		v.Controllers = append(v.Controllers, cv)
		v.Routes = append(v.Routes, cv.Ops...)
		v.Clients = append(v.Clients, buildClientView(c, cv, opts.Cell, len(spec.Controllers) == 1))

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
			v.UsesJson = true
		}

		if r.Response != "" {
			wire[r.Response] = true
		}

		if r.Response != "" && !r.Stream {
			v.UsesJson = true
		}

		if len(r.Params) > 0 {
			v.UsesPath = true
		}

		v.HasStream = v.HasStream || r.Stream
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
		return "super::" + rustname.Snake(ft.Ref) + "::" + ft.Ref
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

func buildControllerView(c Controller, portsByName map[string]portView) controllerView {
	cv := controllerView{Name: c.Name, Snake: c.Snake, Pascal: c.Pascal}

	for _, p := range c.Ports {
		cv.Ports = append(cv.Ports, portsByName[p])
	}

	for _, op := range c.Operations {
		cv.Ops = append(cv.Ops, buildOpView(op, c, portsByName))
		cv.Auth = cv.Auth || op.Auth
	}

	cv.TypeImports = typeImports(c.Operations)

	return cv
}

func typeImports(ops []Operation) []importView {
	imports := map[string]bool{}

	for _, op := range ops {
		if op.Body != "" {
			imports[op.Body] = true
		}

		if op.Response != "" {
			imports[op.Response] = true
		}
	}

	out := []importView{}

	for _, name := range sortedKeys(imports) {
		out = append(out, importView{Snake: rustname.Snake(name), Name: name})
	}

	return out
}

func buildClientView(c Controller, cv controllerView, cell string, only bool) clientView {
	adapterName := cell + "_client"
	if !only {
		adapterName = cell + "_" + c.Snake + "_client"
	}

	client := clientView{
		Snake:       c.Snake,
		Pascal:      c.Pascal,
		Trait:       c.Pascal + "Client",
		Error:       c.Pascal + "ClientError",
		Struct:      c.Pascal + "RestClient",
		Config:      c.Pascal + "RestClientConfig",
		Module:      c.Snake + "_rest_client",
		PortModule:  c.Snake + "_client",
		AdapterName: adapterName,
		Ops:         cv.Ops,
		TypeImports: cv.TypeImports,
	}

	for _, op := range cv.Ops {
		client.HasStream = client.HasStream || op.Stream
		client.UsesJson = client.UsesJson || op.Body != "" || (op.Response != "" && !op.Stream)
	}

	return client
}

func buildOpView(op Operation, c Controller, portsByName map[string]portView) opView {
	ov := opView{
		ID:                op.ID,
		Ident:             op.Ident,
		Method:            op.Method,
		MethodLower:       op.MethodLower,
		Path:              op.Path,
		Body:              op.Body,
		Response:          op.Response,
		Auth:              op.Auth,
		Stream:            op.Stream,
		ControllerSnake:   c.Snake,
		ControllerPascal:  c.Pascal,
		ReturnType:        "()",
		ClientReturn:      "()",
		StatusExpr:        statusExpr(op.Status),
		InvalidStatusExpr: statusExpr(op.InvalidStatus),
	}

	if op.Response != "" {
		ov.ReturnType = op.Response
		ov.ClientReturn = op.Response
	}

	if op.Stream {
		ov.ReturnType = "std::sync::mpsc::Receiver<" + op.Response + ">"
		ov.ClientReturn = ov.ReturnType
	}

	for _, p := range op.Ports {
		ov.Ports = append(ov.Ports, portsByName[p])
	}

	traitArgs := []string{}
	extractors := []string{"State(state): State<HttpState>"}
	controllerCall := []string{}

	if op.Auth {
		traitArgs = append(traitArgs, "subject: Subject")
		extractors = append(extractors, "headers: HeaderMap")
		controllerCall = append(controllerCall, "subject")
	}

	pathIdents := []string{}
	pathTypes := []string{}
	clientArgs := []string{}

	for _, p := range op.Params {
		pv := paramView{Ident: p.Ident, CoreType: scalarType(p.Kind), ArgType: scalarType(p.Kind)}
		call := p.Ident

		if p.Kind == "string" {
			pv.ArgType = "&str"
			call = "&" + p.Ident
		}

		ov.Params = append(ov.Params, pv)
		traitArgs = append(traitArgs, pv.Ident+": "+pv.ArgType)
		clientArgs = append(clientArgs, pv.Ident+": "+pv.ArgType)
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
		clientArgs = append(clientArgs, "body: "+op.Body)
		extractors = append(extractors, "Json(body): Json<"+op.Body+"Wire>")
		controllerCall = append(controllerCall, "body.into()")
	}

	ov.TraitArgs = strings.Join(traitArgs, ", ")
	ov.ClientArgs = strings.Join(clientArgs, ", ")
	ov.Extractors = strings.Join(extractors, ", ")
	ov.ControllerCall = strings.Join(controllerCall, ", ")
	ov.URLExpr = urlExpr(op.Path, pathIdents)

	switch {
	case op.Stream:
		ov.HandlerReturn = "Result<Response, Rejection>"
		ov.OkExpr = "event_stream::<" + op.Response + ", " + op.Response + "Wire>(events)"
	case op.Response == "":
		ov.HandlerReturn = "Result<StatusCode, Rejection>"
		ov.OkExpr = ov.StatusExpr
	default:
		ov.HandlerReturn = "Result<(StatusCode, Json<" + op.Response + "Wire>), Rejection>"
		ov.OkExpr = "(" + ov.StatusExpr + ", Json(out.into()))"
	}

	return ov
}

func urlExpr(path string, idents []string) string {
	format := pathParamPattern.ReplaceAllString(path, "{}")

	args := []string{"self.base_url"}
	args = append(args, idents...)

	return fmt.Sprintf("format!(\"{}%s\", %s)", format, strings.Join(args, ", "))
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
