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
	"strconv"
	"strings"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/taxonomy"
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
	HandPorts        []handPortView
	Controllers      []controllerView
	Clients          []clientView
	Routes           []opView
	WireImports      []string
	UsesPath         bool
	UsesQuery        bool
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

type lookupView struct {
	By     string
	Ident  string
	Method string
	Page   bool
}

type publishView struct {
	Name      string
	Snake     string
	Port      string
	PortSnake string
	Assigns   []string
}

type storeView struct {
	Name               string
	Snake              string
	Upper              string
	Port               string
	PortSnake          string
	Key                string
	KeyIdent           string
	Lookups            []lookupView
	Struct             string
	ConfigStruct       string
	AdapterName        string
	Module             string
	HasMemory          bool
	MemoryStruct       string
	MemoryConfigStruct string
	MemoryAdapterName  string
	MemoryModule       string
	HasSqlite          bool
	HasOneLookup       bool
	DefaultPath        string
	DefaultCapacity    int
	Publishes          *publishView
}

type eventView struct {
	Name              string
	Snake             string
	Port              string
	PortSnake         string
	From              string
	FromSnake         string
	HasMemory         bool
	MemoryStruct      string
	MemoryConfigStruct string
	MemoryAdapterName string
	MemoryModule      string
}

type portView struct {
	Port      string
	PortSnake string
	Snake     string
	Reaching  string
}

type handMethodView struct {
	Ident  string
	Args   string
	Return string
}

type handPortView struct {
	Name               string
	Snake              string
	PortSnake          string
	Error              string
	Kind               string
	Instant            string
	InstantSnake       string
	Span               string
	SpanSnake          string
	HasMemory          bool
	MemoryStruct       string
	MemoryConfigStruct string
	MemoryAdapterName  string
	MemoryModule       string
	Imports            []importView
	Methods            []handMethodView
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
	HasQuery          bool
	QueryLines        []string
	ClientQueryLines  []string
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
	Taxonomy    []taxonomy.Member
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
	Auth         bool
	HasStream    bool
	HasQuery     bool
	UsesJson     bool
	WireRuntime  taxonomy.WireMember
	WireTaxonomy []taxonomy.WireMember
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

	publishedBy := map[string]publishView{}

	for _, e := range spec.Events {
		publishedBy[e.From.Name] = publishView{
			Name:      e.Name,
			Snake:     e.Snake,
			Port:      e.Name + SubscribePortSuffix,
			PortSnake: e.Snake + "_subscribe",
			Assigns:   publishAssigns(e.TypeDef),
		}
	}

	for _, s := range spec.Stores {
		sv := storeView{
			Name:               s.Name,
			Snake:              s.Snake,
			Upper:              rustname.Upper(s.Name),
			Port:               s.Name + StorePortSuffix,
			PortSnake:          s.Snake + "_store",
			Key:                s.Store.Key,
			KeyIdent:           s.Store.KeyIdent,
			Struct:             s.Name + "SqliteStore",
			ConfigStruct:       s.Name + "SqliteStoreConfig",
			AdapterName:        adapterName(spec.Stores, s, "sqlite"),
			Module:             s.Snake + "_sqlite",
			MemoryStruct:       s.Name + "MemoryStore",
			MemoryConfigStruct: s.Name + "MemoryStoreConfig",
			MemoryAdapterName:  adapterName(spec.Stores, s, "memory"),
			MemoryModule:       s.Snake + "_memory",
			DefaultPath:        DefaultStorePath,
			DefaultCapacity:    DefaultStoreCapacity,
		}

		for _, kind := range s.Store.Adapters {
			sv.HasSqlite = sv.HasSqlite || kind == StoreAdapterSqlite
			sv.HasMemory = sv.HasMemory || kind == StoreAdapterMemory
		}

		for _, l := range s.Store.Lookups {
			sv.HasOneLookup = sv.HasOneLookup || l.Answers == LookupOne

			sv.Lookups = append(sv.Lookups, lookupView{
				By:     l.By,
				Ident:  l.Ident,
				Method: lookupMethod(l),
				Page:   l.Answers == LookupPage,
			})
		}

		if published, feeds := publishedBy[s.Name]; feeds {
			sv.Publishes = &published
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
			Name:               e.Name,
			Snake:              e.Snake,
			Port:               e.Name + SubscribePortSuffix,
			PortSnake:          e.Snake + "_subscribe",
			From:               e.From.Name,
			FromSnake:          e.From.Snake,
			MemoryStruct:       e.Name + "MemoryFeed",
			MemoryConfigStruct: e.Name + "MemoryFeedConfig",
			MemoryAdapterName:  feedAdapterName(spec.Events, e, "memory"),
			MemoryModule:       e.Snake + "_memory",
		}

		for _, kind := range e.Adapters {
			ev.HasMemory = ev.HasMemory || kind == FeedAdapterMemory
		}

		v.Events = append(v.Events, ev)
		portsByName[ev.Port] = portView{
			Port:      ev.Port,
			PortSnake: ev.PortSnake,
			Snake:     ev.Snake,
			Reaching:  "subscribing to " + ev.Snake + " events for",
		}
	}

	for _, h := range spec.HandPorts {
		hv := buildHandPortView(h)
		v.HandPorts = append(v.HandPorts, hv)
		portsByName[hv.Name] = portView{
			Port:      hv.Name,
			PortSnake: hv.PortSnake,
			Snake:     hv.Snake,
			Reaching:  "calling the " + hv.Snake + " port for",
		}
	}

	used := map[string]bool{}

	for _, c := range spec.Controllers {
		cv := buildControllerView(c, portsByName)
		v.Controllers = append(v.Controllers, cv)
		v.Routes = append(v.Routes, cv.Ops...)
		v.Clients = append(v.Clients, buildClientView(c, cv, opts.Cell))

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

		if r.HasQuery {
			v.UsesQuery = true
		}

		v.HasStream = v.HasStream || r.Stream
	}

	v.WireImports = sortedKeys(wire)

	return v
}

func adapterName(stores []TypeDef, store TypeDef, kind string) string {
	if len(stores) == 1 {
		return kind
	}

	return store.Snake + "_" + kind
}

func feedAdapterName(events []Event, event Event, kind string) string {
	if len(events) == 1 {
		return kind + "_feed"
	}

	return event.Snake + "_" + kind + "_feed"
}

func lookupMethod(l Lookup) string {
	if l.Answers == LookupPage {
		return "page_by_" + rustname.Snake(l.By)
	}

	return "get_by_" + rustname.Snake(l.By)
}

func publishAssigns(event TypeDef) []string {
	assigns := make([]string, 0, len(event.Fields))

	for _, f := range event.Fields {
		read := "v." + f.Ident
		if !isCopy(f) {
			read += ".clone()"
		}

		assigns = append(assigns, f.Ident+": "+read+",")
	}

	return assigns
}

func isCopy(f Field) bool {
	if f.Optional {
		return false
	}

	switch f.Type.Kind {
	case "integer", "number", "boolean":
		return true
	default:
		return false
	}
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

func buildHandPortView(h HandPort) handPortView {
	hv := handPortView{
		Name:               h.Name,
		Snake:              h.Snake,
		PortSnake:          h.Snake,
		Error:              h.Name + "Error",
		Kind:               h.Kind,
		Instant:            h.Instant,
		InstantSnake:       rustname.Snake(h.Instant),
		Span:               h.Span,
		SpanSnake:          rustname.Snake(h.Span),
		MemoryStruct:       h.Name + "Memory",
		MemoryConfigStruct: h.Name + "MemoryConfig",
		MemoryAdapterName:  "memory",
		MemoryModule:       h.Snake + "_memory",
	}

	for _, kind := range h.Adapters {
		hv.HasMemory = hv.HasMemory || kind == ClockAdapterMemory
	}

	imports := map[string]bool{}

	for _, m := range h.Methods {
		mv := handMethodView{Ident: m.Ident, Return: "()"}

		if m.Request != "" {
			mv.Args = "request: " + m.Request
			imports[m.Request] = true
		}

		if m.Reply != "" {
			mv.Return = m.Reply
			imports[m.Reply] = true
		}

		hv.Methods = append(hv.Methods, mv)
	}

	for _, name := range sortedKeys(imports) {
		hv.Imports = append(hv.Imports, importView{Snake: rustname.Snake(name), Name: name})
	}

	return hv
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
	cv.Taxonomy = taxonomy.Detailed(c.Snake)

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

func buildClientView(c Controller, cv controllerView, cell string) clientView {
	adapterName := cell + "_" + c.Snake + "_client"

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
		Ops:          cv.Ops,
		TypeImports:  cv.TypeImports,
		WireRuntime:  taxonomy.WireRuntime(),
		WireTaxonomy: taxonomy.WireDetailed(),
	}

	for _, op := range cv.Ops {
		client.Auth = client.Auth || op.Auth
		client.HasStream = client.HasStream || op.Stream
		client.HasQuery = client.HasQuery || op.HasQuery
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

	if len(op.Query) > 0 {
		ov.HasQuery = true

		extractors = append(extractors, "Query(query): Query<QueryMap>")
	}

	for _, q := range op.Query {
		traitArgs = append(traitArgs, q.Ident+": "+queryArgType(q))
		clientArgs = append(clientArgs, q.Ident+": "+queryArgType(q))
		controllerCall = append(controllerCall, queryCall(q))
		ov.QueryLines = append(ov.QueryLines, queryLine(q, ov.InvalidStatusExpr))
	}

	ov.ClientQueryLines = clientQueryLines(op.Query)

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

	if ov.HasQuery {
		ov.URLExpr = "with_query(" + ov.URLExpr + ", query)"
	}

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

func queryArgType(q QueryParam) string {
	if q.Required && q.Kind == "string" {
		return "&str"
	}

	if q.Required {
		return scalarType(q.Kind)
	}

	return "Option<" + scalarType(q.Kind) + ">"
}

func queryCall(q QueryParam) string {
	if q.Required && q.Kind == "string" {
		return "&" + q.Ident
	}

	return q.Ident
}

func queryLine(q QueryParam, invalid string) string {
	name := strconv.Quote(q.Name)

	switch {
	case q.Required && q.Kind == "string":
		return fmt.Sprintf("let %s = required_query(&query, %s, %s)?;", q.Ident, name, invalid)
	case q.Required:
		return fmt.Sprintf("let %s = required_integer(&query, %s, %s)?;", q.Ident, name, invalid)
	case q.Kind == "string":
		return fmt.Sprintf("let %s = query.get(%s).cloned();", q.Ident, name)
	default:
		return fmt.Sprintf("let %s = optional_integer(&query, %s, %s)?;", q.Ident, name, invalid)
	}
}

func clientQueryLines(query []QueryParam) []string {
	if len(query) == 0 {
		return nil
	}

	required := []string{}
	optional := []string{}

	for _, q := range query {
		name := strconv.Quote(q.Name)

		if q.Required {
			required = append(required, "("+name+", "+q.Ident+".to_string())")

			continue
		}

		pushed := "value.to_string()"
		if q.Kind == "string" {
			pushed = "value"
		}

		optional = append(optional, fmt.Sprintf(
			"if let Some(value) = %s {\n            query.push((%s, %s));\n        }",
			q.Ident, name, pushed,
		))
	}

	declaration := "let mut query: Vec<(&str, String)> = Vec::new();"

	switch {
	case len(required) > 0 && len(optional) > 0:
		declaration = "let mut query: Vec<(&str, String)> = vec![" + strings.Join(required, ", ") + "];"
	case len(required) > 0:
		declaration = "let query: Vec<(&str, String)> = vec![" + strings.Join(required, ", ") + "];"
	}

	return append([]string{declaration}, optional...)
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
