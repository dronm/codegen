package codegen

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// AccessSpec is deliberately metadata only. Principal resolution and policy
// factories belong to the application, never to the schema or generator.
type AccessSpec struct {
	Enabled       bool                              `yaml:"enabled"`
	Resource      yaml.Node                         `yaml:"resource,omitempty"`
	Bindings      AccessBindingsSpec                `yaml:"bindings"`
	ServiceInputs map[string]AccessServiceInputSpec `yaml:"serviceInputs"`
}

type AccessBindingsSpec struct {
	Model  *AccessBindingSpec `yaml:"model"`
	List   *AccessBindingSpec `yaml:"list"`
	Detail *AccessBindingSpec `yaml:"detail"`
	Key    *AccessBindingSpec `yaml:"key"`
}

// ServiceInputs is only needed for additional handwritten service methods or
// explicit handwritten decoders. Standard CRUD signatures need no extra YAML.
type AccessServiceInputSpec struct {
	Operation string             `yaml:"operation"`
	Decoder   *AccessDecoderSpec `yaml:"decoder"`
}

type AccessDecoderSpec struct {
	Name   string `yaml:"name"`
	Import string `yaml:"import"`
}

type AccessView struct {
	Enabled           bool
	Resource          string
	Models            []AccessModelView
	Decoders          []AccessDecoderView
	DecoderImports    []AccessImportView
	NeedsModelsImport bool
	CreateBinder      string
	UpdateKeyType     string
	UpdateModelType   string
	UpdateKeyValue    bool
	UpdateModelValue  bool
	CreateValue       bool
	CreateKind        string
	CreateModelType   string
	HTTPModelbind     bool
	DecoderAdapters   []AccessDecoderAdapterView
	AdapterModelbind  bool
}

type AccessDecoderAdapterView struct {
	Name       string
	Kind       string
	Model      string
	Key        string
	ModelValue bool
	KeyValue   bool
	ForInsert  bool
}

type AccessModelView struct {
	Representation string
	Name           string
	Resource       string
	Relation       string
	Rows           []AccessFieldView
	Writes         []AccessFieldView
}

type AccessFieldView struct {
	Field  string
	Column string
}

type AccessDecoderView struct {
	Method     string
	Expression string
}

type AccessImportView struct {
	Name string
	Path string
}

func accessError(spec ObjectSpec, format string, args ...any) error {
	return fmt.Errorf("access for %s (%s): %s", spec.Name, sourceLabel(spec.SourceFile), fmt.Sprintf(format, args...))
}

func buildAccessView(cfg Config, spec ObjectSpec, object ObjectView) (AccessView, error) {
	if spec.Access == nil || !spec.Access.Enabled {
		return AccessView{}, nil
	}
	resource := strings.TrimSpace(spec.Name)
	if spec.Access.Resource.Kind != 0 {
		// A Node preserves omission versus explicit YAML null without replacing
		// the normal strict decoder or breaking aliases outside this section.
		node := &spec.Access.Resource
		seen := map[*yaml.Node]bool{}
		for node.Kind == yaml.AliasNode && node.Alias != nil && !seen[node] {
			seen[node] = true
			node = node.Alias
		}
		if node.Kind != yaml.ScalarNode || node.ShortTag() != "!!str" || strings.TrimSpace(node.Value) == "" {
			return AccessView{}, accessError(spec, "resource must be a non-empty string, not null or whitespace-only (omit it to use the object name)")
		}
		resource = strings.TrimSpace(node.Value)
	}
	if strings.IndexFunc(resource, unicode.IsControl) >= 0 {
		return AccessView{}, accessError(spec, "resource must not contain control characters")
	}
	result := AccessView{Enabled: true, Resource: resource}
	bindings := spec.Access.Bindings
	writable := object.CRUD.Create || object.CRUD.Update
	for _, input := range spec.Access.ServiceInputs {
		if input.Operation == "insert" || input.Operation == "update" {
			writable = true
		}
	}
	add := func(representation, name, relation string, table TableSpec, binding *AccessBindingSpec, write bool) error {
		if binding == nil || binding.Rows == nil {
			return accessError(spec, "bindings.%s.rows is required for concrete model %s (use {} for an intentional empty mapping)", representation, name)
		}
		if write && binding.Writes == nil {
			return accessError(spec, "bindings.%s.writes is required for writable model %s (use {} when no write fields are mapped)", representation, name)
		}
		if !write && binding.Writes != nil {
			return accessError(spec, "bindings.%s.writes is inapplicable: %s is not a writable representation", representation, name)
		}
		view := AccessModelView{Representation: representation, Name: name, Resource: resource, Relation: relation}
		for _, field := range sortedAccessFields(binding.Rows) {
			if !accessIdentifier(field) {
				return accessError(spec, "bindings.%s.rows field %q must be a logical identifier", representation, field)
			}
			column := binding.Rows[field]
			if err := validateAccessRowColumn(table, column); err != nil {
				return accessError(spec, "bindings.%s.rows field %q: %v", representation, field, err)
			}
			view.Rows = append(view.Rows, AccessFieldView{Field: field, Column: column})
		}
		usedColumns := map[string]string{}
		for _, field := range sortedAccessFields(binding.Writes) {
			column := binding.Writes[field]
			if !accessIdentifier(field) {
				return accessError(spec, "bindings.%s.writes field %q must be a logical identifier", representation, field)
			}
			canonicalColumn, err := canonicalAccessWriteColumn(column)
			if err != nil {
				return accessError(spec, "bindings.%s.writes field %q: %v", representation, field, err)
			}
			if previous, ok := usedColumns[canonicalColumn]; ok {
				return accessError(spec, "bindings.%s.writes fields %q and %q conflict on column %q", representation, previous, field, column)
			}
			usedColumns[canonicalColumn] = field
			// SQL identifiers here are unquoted. Normalize before both metadata
			// validation and emission: webapp matches write columns to modelbind
			// field IDs case-sensitively during effective-input preparation.
			column = canonicalColumn
			// Unknown columns are intentional explicit declarations of external
			// schema facts, not inferred fields. Known columns can be checked.
			for _, modelField := range object.Fields {
				if modelField.Name != column && modelField.JSONName != column {
					continue
				}
				if modelField.Name != column || modelField.JSONName != column {
					return accessError(spec, "bindings.%s.writes field %q: column %q must match the modelbind field ID and database column; field %s uses JSON name %q", representation, field, column, modelField.Name, modelField.JSONName)
				}
				if modelField.ServerGenerated || modelField.SrvCalc || modelField.AutoIncrement || modelField.ReadOnly || !modelField.JSONEnabled {
					return accessError(spec, "bindings.%s.writes field %q targets non-writable model field %q; policies cannot change srvCalc/database-generated semantics", representation, field, column)
				}
			}
			view.Writes = append(view.Writes, AccessFieldView{Field: field, Column: column})
		}
		result.Models = append(result.Models, view)
		return nil
	}
	if err := add("model", object.Name, object.Relation, spec.Table, bindings.Model, writable); err != nil {
		return AccessView{}, err
	}
	// Reused representations have no independent binding. An explicitly
	// repeated rows map is accepted only as an equality assertion, never merged.
	projection := func(label, name, relation string, projection *ListSpec, binding *AccessBindingSpec) error {
		if projection != nil {
			return add(label, name, relation, projection.Table, binding, false)
		}
		if binding != nil && (binding.Rows == nil || binding.Writes != nil || !reflect.DeepEqual(binding.Rows, bindings.Model.Rows)) {
			return accessError(spec, "bindings.%s reuses concrete model %s: omit it or repeat exactly bindings.model.rows without writes", label, object.Name)
		}
		return nil
	}
	if err := projection("list", object.ListModelName, object.ListRelation, spec.List, bindings.List); err != nil {
		return AccessView{}, err
	}
	if err := projection("detail", object.DetailModelName, object.DetailRelation, spec.Detail, bindings.Detail); err != nil {
		return AccessView{}, err
	}
	if err := add("key", object.Name+"Key", object.Relation, spec.Table, bindings.Key, false); err != nil {
		return AccessView{}, err
	}
	seenTypes := map[string]bool{}
	for _, model := range result.Models {
		if seenTypes[model.Name] {
			return AccessView{}, accessError(spec, "representation %s conflicts with another generated concrete type %s", model.Representation, model.Name)
		}
		seenTypes[model.Name] = true
	}
	if err := buildAccessDecoders(cfg, spec, object, &result); err != nil {
		return AccessView{}, err
	}
	return result, nil
}

func sortedAccessFields[T any](fields map[string]T) []string {
	keys := make([]string, 0, len(fields))
	for field := range fields {
		keys = append(keys, field)
	}
	sort.Strings(keys)
	return keys
}

func validateAccessRowColumn(table TableSpec, column string) error {
	qualifiers := []string{table.Name, schemaOrPublic(table.Schema) + "." + table.Name}
	if table.Alias != "" {
		qualifiers = []string{table.Alias}
	}
	return validateAccessColumnReference(column, qualifiers...)
}
