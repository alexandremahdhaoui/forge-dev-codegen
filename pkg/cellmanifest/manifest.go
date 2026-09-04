package cellmanifest

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"sigs.k8s.io/yaml"
)

const (
	FileName = "zz_generated_cell.yaml"
	Version  = "1"
)

//go:embed cell.schema.json
var schema []byte

func Schema() []byte {
	out := make([]byte, len(schema))
	copy(out, schema)

	return out
}

type Manifest struct {
	Version   string   `json:"version" yaml:"version"`
	Cell      string   `json:"cell" yaml:"cell"`
	Generator string   `json:"generator" yaml:"generator"`
	Provides  Provides `json:"provides" yaml:"provides"`
	Requires  Requires `json:"requires" yaml:"requires"`
}

type Provides struct {
	Drivers     []Driver     `json:"drivers,omitempty" yaml:"drivers,omitempty"`
	Adapters    []Adapter    `json:"adapters,omitempty" yaml:"adapters,omitempty"`
	Controllers []Controller `json:"controllers,omitempty" yaml:"controllers,omitempty"`
	Ports       []Port       `json:"ports,omitempty" yaml:"ports,omitempty"`
}

type Requires struct {
	Ports []string `json:"ports" yaml:"ports"`
}

type Driver struct {
	Name     string                 `json:"name" yaml:"name"`
	Type     string                 `json:"type" yaml:"type"`
	Module   string                 `json:"module" yaml:"module"`
	Requires []string               `json:"requires,omitempty" yaml:"requires,omitempty"`
	Config   map[string]ConfigField `json:"config,omitempty" yaml:"config,omitempty"`
}

type Adapter struct {
	Name       string                 `json:"name" yaml:"name"`
	Type       string                 `json:"type" yaml:"type"`
	Module     string                 `json:"module" yaml:"module"`
	Implements string                 `json:"implements" yaml:"implements"`
	Fallible   bool                   `json:"fallible,omitempty" yaml:"fallible,omitempty"`
	Config     map[string]ConfigField `json:"config,omitempty" yaml:"config,omitempty"`
}

type Controller struct {
	Trait  string   `json:"trait" yaml:"trait"`
	Impl   string   `json:"impl" yaml:"impl"`
	Module string   `json:"module" yaml:"module"`
	Ports  []string `json:"ports" yaml:"ports"`
}

type Port struct {
	Trait  string `json:"trait" yaml:"trait"`
	Module string `json:"module" yaml:"module"`
}

type ConfigField struct {
	Type        string `json:"type" yaml:"type"`
	Required    bool   `json:"required,omitempty" yaml:"required,omitempty"`
	Default     any    `json:"default,omitempty" yaml:"default,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

const (
	FieldTypeString   = "string"
	FieldTypeInteger  = "integer"
	FieldTypeBoolean  = "boolean"
	FieldTypeDuration = "duration"
)

var (
	snakeNamePattern  = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	rustIdentPattern  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	modulePathPattern = regexp.MustCompile(`^[a-z_][a-z0-9_]*(::[a-z_][a-z0-9_]*)*$`)
)

func FieldTypes() []string {
	return []string{FieldTypeString, FieldTypeInteger, FieldTypeBoolean, FieldTypeDuration}
}

func knownFieldType(fieldType string) bool {
	for _, known := range FieldTypes() {
		if fieldType == known {
			return true
		}
	}

	return false
}

func Parse(data []byte) (Manifest, error) {
	var m Manifest

	if err := refuseNullKeys(data); err != nil {
		return Manifest{}, fmt.Errorf("parsing cell manifest: %w", err)
	}

	if err := yaml.UnmarshalStrict(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("parsing cell manifest: %w", err)
	}

	if err := m.Validate(); err != nil {
		return Manifest{}, fmt.Errorf("parsing cell manifest: %w", err)
	}

	return m, nil
}

func refuseNullKeys(data []byte) error {
	var document map[string]any
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil
	}

	return refuseNullValue(document, "", "")
}

func refuseNullValue(value any, key, path string) error {
	if value == nil {
		return nullError(key, path)
	}

	switch typed := value.(type) {
	case map[string]any:
		return refuseNullFields(typed, key, path)
	case []any:
		return refuseNullItems(typed, key, path)
	}

	return nil
}

func refuseNullFields(fields map[string]any, key, path string) error {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		if err := refuseNullValue(fields[name], fieldKey(key, name), join(path, name)); err != nil {
			return err
		}
	}

	return nil
}

func refuseNullItems(items []any, key, path string) error {
	for index, item := range items {
		if err := refuseNullValue(item, key+itemSuffix, fmt.Sprintf("%s[%d]", path, index)); err != nil {
			return err
		}
	}

	return nil
}

func fieldKey(key, name string) string {
	named := join(key, name)
	if _, known := schemaKeys[named]; known {
		return named
	}

	return join(key, anyNameSegment)
}

func nullError(key, path string) error {
	node, known := schemaKeys[key]
	if !known || !node.refusesNull {
		return nil
	}

	return fmt.Errorf("key %q is null, %s", path, nullAdvice(node.kind))
}

func nullAdvice(kind string) string {
	switch kind {
	case kindList:
		return "write an empty list"
	case kindMap:
		return "write an empty map"
	}

	return "write a value"
}

func Read(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("reading cell manifest %q: %w", path, err)
	}

	m, err := Parse(data)
	if err != nil {
		return Manifest{}, fmt.Errorf("reading cell manifest %q: %w", path, err)
	}

	return m, nil
}

func Write(path string, m Manifest) error {
	data, err := Marshal(m)
	if err != nil {
		return fmt.Errorf("writing cell manifest %q: %w", path, err)
	}

	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("writing cell manifest %q: %w", path, err)
		}
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing cell manifest %q: %w", path, err)
	}

	return nil
}

func Marshal(m Manifest) ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("marshalling cell manifest: %w", err)
	}

	body, err := yaml.Marshal(withEmptySlices(m))
	if err != nil {
		return nil, fmt.Errorf("marshalling cell manifest: %w", err)
	}

	header := fmt.Sprintf("# Code generated by %s. DO NOT EDIT.\n", m.Generator)

	return append([]byte(header), body...), nil
}

func withEmptySlices(m Manifest) Manifest {
	m.Requires.Ports = copyStrings(m.Requires.Ports)

	drivers := make([]Driver, 0, len(m.Provides.Drivers))
	for _, driver := range m.Provides.Drivers {
		driver.Requires = copyStrings(driver.Requires)
		drivers = append(drivers, driver)
	}

	controllers := make([]Controller, 0, len(m.Provides.Controllers))
	for _, controller := range m.Provides.Controllers {
		controller.Ports = copyStrings(controller.Ports)
		controllers = append(controllers, controller)
	}

	m.Provides.Drivers = drivers
	m.Provides.Controllers = controllers
	m.Provides.Adapters = append(make([]Adapter, 0, len(m.Provides.Adapters)), m.Provides.Adapters...)
	m.Provides.Ports = append(make([]Port, 0, len(m.Provides.Ports)), m.Provides.Ports...)

	return m
}

func copyStrings(values []string) []string {
	return append(make([]string, 0, len(values)), values...)
}

func (m Manifest) Validate() error {
	if m.Cell == "" {
		return fmt.Errorf("cell name is empty")
	}

	if !snakeNamePattern.MatchString(m.Cell) {
		return fmt.Errorf("cell name %q is not snake case", m.Cell)
	}

	if m.Version != Version {
		return fmt.Errorf(
			"cell %q declares version %q and the only version is %q",
			m.Cell, m.Version, Version,
		)
	}

	if m.Generator == "" {
		return fmt.Errorf("cell %q names no generator", m.Cell)
	}

	for _, driver := range m.Provides.Drivers {
		if err := driver.validate(m.Cell); err != nil {
			return err
		}
	}

	for _, adapter := range m.Provides.Adapters {
		if err := adapter.validate(m.Cell); err != nil {
			return err
		}
	}

	for _, controller := range m.Provides.Controllers {
		if err := controller.validate(m.Cell); err != nil {
			return err
		}
	}

	for _, port := range m.Provides.Ports {
		if err := port.validate(m.Cell); err != nil {
			return err
		}
	}

	for _, trait := range m.Requires.Ports {
		if !rustIdentPattern.MatchString(trait) {
			return fmt.Errorf("cell %q requires port %q which is not a Rust ident", m.Cell, trait)
		}
	}

	return nil
}

func (d Driver) validate(cell string) error {
	if d.Name == "" {
		return fmt.Errorf("cell %q provides a driver with no name", cell)
	}

	if !snakeNamePattern.MatchString(d.Name) {
		return fmt.Errorf("driver name %q in cell %q is not snake case", d.Name, cell)
	}

	owner := fmt.Sprintf("driver %q in cell %q", d.Name, cell)

	if err := validateType(owner, d.Type); err != nil {
		return err
	}

	if err := validateModule(owner, d.Module); err != nil {
		return err
	}

	if len(d.Requires) == 0 {
		return fmt.Errorf("driver %q in cell %q requires no controller trait", d.Name, cell)
	}

	for _, trait := range d.Requires {
		if !rustIdentPattern.MatchString(trait) {
			return fmt.Errorf("%s requires %q which is not a Rust ident", owner, trait)
		}
	}

	return validateConfig(owner, d.Config)
}

func (a Adapter) validate(cell string) error {
	if a.Name == "" {
		return fmt.Errorf("cell %q provides an adapter with no name", cell)
	}

	if !snakeNamePattern.MatchString(a.Name) {
		return fmt.Errorf("adapter name %q in cell %q is not snake case", a.Name, cell)
	}

	owner := fmt.Sprintf("adapter %q in cell %q", a.Name, cell)

	if err := validateType(owner, a.Type); err != nil {
		return err
	}

	if err := validateModule(owner, a.Module); err != nil {
		return err
	}

	if a.Implements == "" {
		return fmt.Errorf("adapter %q in cell %q implements no port", a.Name, cell)
	}

	if !rustIdentPattern.MatchString(a.Implements) {
		return fmt.Errorf("%s implements %q which is not a Rust ident", owner, a.Implements)
	}

	return validateConfig(owner, a.Config)
}

func (c Controller) validate(cell string) error {
	if c.Trait == "" {
		return fmt.Errorf("cell %q provides a controller with no trait", cell)
	}

	if !rustIdentPattern.MatchString(c.Trait) {
		return fmt.Errorf("controller trait %q in cell %q is not a Rust ident", c.Trait, cell)
	}

	owner := fmt.Sprintf("controller %q in cell %q", c.Trait, cell)

	if c.Impl == "" {
		return fmt.Errorf("%s names no impl", owner)
	}

	if !rustIdentPattern.MatchString(c.Impl) {
		return fmt.Errorf("%s has impl %q which is not a Rust ident", owner, c.Impl)
	}

	if err := validateModule(owner, c.Module); err != nil {
		return err
	}

	for _, trait := range c.Ports {
		if !rustIdentPattern.MatchString(trait) {
			return fmt.Errorf("%s consumes port %q which is not a Rust ident", owner, trait)
		}
	}

	return nil
}

func (p Port) validate(cell string) error {
	if p.Trait == "" {
		return fmt.Errorf("cell %q provides a port with no trait", cell)
	}

	if !rustIdentPattern.MatchString(p.Trait) {
		return fmt.Errorf("port trait %q in cell %q is not a Rust ident", p.Trait, cell)
	}

	return validateModule(fmt.Sprintf("port %q in cell %q", p.Trait, cell), p.Module)
}

func validateType(owner, fieldType string) error {
	if fieldType == "" {
		return fmt.Errorf("%s names no type", owner)
	}

	if !rustIdentPattern.MatchString(fieldType) {
		return fmt.Errorf("%s has type %q which is not a Rust ident", owner, fieldType)
	}

	return nil
}

func validateModule(owner, module string) error {
	if module == "" {
		return fmt.Errorf("%s names no module", owner)
	}

	if !modulePathPattern.MatchString(module) {
		return fmt.Errorf("%s has module %q which is not a :: path", owner, module)
	}

	return nil
}

func validateConfig(owner string, config map[string]ConfigField) error {
	names := make([]string, 0, len(config))
	for name := range config {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		if !snakeNamePattern.MatchString(name) {
			return fmt.Errorf("config field %q of %s is not snake case", name, owner)
		}

		if !knownFieldType(config[name].Type) {
			return fmt.Errorf(
				"config field %q of %s has type %q which is not one of string, integer, boolean, duration",
				name, owner, config[name].Type,
			)
		}
	}

	return nil
}
