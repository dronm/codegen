package codegen

import (
	"fmt"
	"go/token"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

// EnumSpec describes a shared application enum. PostgreSQL type creation and Go
// enum declarations/registration remain application-owned.
type EnumSpec struct {
	Name         string          `yaml:"name"`
	GoType       string          `yaml:"goType"`
	SQLType      string          `yaml:"sqlType"`
	FrontendName string          `yaml:"frontendName"`
	Values       []EnumValueSpec `yaml:"values"`
}

type EnumValueSpec struct {
	Value string `yaml:"value"`
	Label string `yaml:"label"`
}

func validateEnumDefinition(enum EnumSpec) error {
	if strings.TrimSpace(enum.Name) == "" {
		return fmt.Errorf("enum name is required")
	}
	if enum.Name != strings.TrimSpace(enum.Name) || strings.ContainsAny(enum.Name, "\r\n\t") {
		return fmt.Errorf("enum %q: name must not contain surrounding or control whitespace", enum.Name)
	}
	if !token.IsIdentifier(enum.GoType) || enum.GoType == "_" {
		return fmt.Errorf("enum %q: goType is required and must be a Go type identifier", enum.Name)
	}
	if !isTypeScriptIdentifier(enum.FrontendName) || enum.FrontendName == "" || isReservedEnumTypeName(enum.FrontendName) {
		return fmt.Errorf("enum %q: frontendName is required and must be a TypeScript type identifier", enum.Name)
	}
	if enum.SQLType != "" {
		parts := strings.Split(enum.SQLType, ".")
		if len(parts) > 2 {
			return fmt.Errorf("enum %q: sqlType must be a type name or schema-qualified type name", enum.Name)
		}
		for _, part := range parts {
			if !isSQLIdentifier(part) {
				return fmt.Errorf("enum %q: invalid sqlType %q", enum.Name, enum.SQLType)
			}
		}
	}
	if len(enum.Values) == 0 {
		return fmt.Errorf("enum %q has no values", enum.Name)
	}
	seen := make(map[string]bool)
	for _, item := range enum.Values {
		if strings.TrimSpace(item.Value) == "" {
			return fmt.Errorf("enum %q contains an empty value", enum.Name)
		}
		if seen[item.Value] {
			return fmt.Errorf("enum %q contains duplicate value %q", enum.Name, item.Value)
		}
		seen[item.Value] = true
	}
	return nil
}

func isReservedEnumTypeName(name string) bool {
	switch name {
	case "Date", "Record", "Partial", "Pick", "Promise", "Map", "Set", "any", "boolean", "number", "string", "unknown", "never", "object", "symbol", "bigint", "undefined", "null", "void", "type", "enum", "interface", "class", "const", "let", "var", "export", "import", "default", "new", "return", "function", "extends", "implements", "private", "protected", "public", "static", "true", "false", "v", "createCommonSchemas", "defaultTranslate", "TranslateFn":
		return true
	}
	return false
}

// resolveProjectEnums runs after all YAML files have been decoded, so a field
// may reference a definition declared in any active object specification.
func resolveProjectEnums(specs []ObjectSpec) ([]ObjectSpec, error) {
	registry := make(map[string]EnumSpec)
	owners := make(map[string]string)
	for _, spec := range specs {
		seen := make(map[string]bool)
		for _, enum := range spec.Enums {
			if err := validateEnumDefinition(enum); err != nil {
				return nil, fmt.Errorf("model %s (%s): %w", spec.Name, sourceLabel(spec.SourceFile), err)
			}
			if seen[enum.Name] {
				return nil, fmt.Errorf("model %s (%s): duplicate enum name %q", spec.Name, sourceLabel(spec.SourceFile), enum.Name)
			}
			seen[enum.Name] = true
			if previous, exists := registry[enum.Name]; exists && !reflect.DeepEqual(previous, enum) {
				return nil, fmt.Errorf("model %s (%s): conflicting definition of enum %q (previously defined by %s); metadata, ordered values, and labels must agree", spec.Name, sourceLabel(spec.SourceFile), enum.Name, owners[enum.Name])
			}
			registry[enum.Name] = enum
			owners[enum.Name] = spec.Name + " (" + sourceLabel(spec.SourceFile) + ")"
		}
	}
	if err := validateEnumRegistryNames(registry); err != nil {
		return nil, err
	}
	result := make([]ObjectSpec, 0, len(specs))
	for _, spec := range specs {
		normalized, err := resolveObjectEnums(spec, registry)
		if err != nil {
			return nil, err
		}
		result = append(result, normalized)
	}
	return result, nil
}

func validateEnumRegistryNames(registry map[string]EnumSpec) error {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	owners := make(map[string]string)
	for _, name := range names {
		enum := registry[name]
		view := buildFrontendEnum(enum)
		for _, identity := range []string{"type:" + enum.FrontendName, "file:" + strings.ToLower(view.FileBase), "const:" + view.ValuesName} {
			if previous, exists := owners[identity]; exists && previous != name {
				return fmt.Errorf("enums %q and %q conflict on generated frontend identity %q", previous, name, identity)
			}
			owners[identity] = name
		}
	}
	return nil
}

func resolveObjectEnums(spec ObjectSpec, shared map[string]EnumSpec) (ObjectSpec, error) {
	registry := make(map[string]EnumSpec, len(shared)+len(spec.Enums))
	for name, enum := range shared {
		registry[name] = enum
	}
	seen := make(map[string]bool)
	for _, enum := range spec.Enums {
		if err := validateEnumDefinition(enum); err != nil {
			return spec, fmt.Errorf("model %s: %w", spec.Name, err)
		}
		if seen[enum.Name] {
			return spec, fmt.Errorf("model %s: duplicate enum name %q", spec.Name, enum.Name)
		}
		seen[enum.Name] = true
		if old, exists := registry[enum.Name]; exists && !reflect.DeepEqual(old, enum) {
			return spec, fmt.Errorf("model %s: conflicting definition of enum %q", spec.Name, enum.Name)
		}
		registry[enum.Name] = enum
	}
	if err := validateEnumRegistryNames(registry); err != nil {
		return spec, fmt.Errorf("model %s: %w", spec.Name, err)
	}
	spec.enumRegistry = registry
	resolveFields := func(fields []FieldSpec, location string) ([]FieldSpec, error) {
		result := append([]FieldSpec(nil), fields...)
		for i, field := range result {
			normalized, err := resolveEnumField(field, registry)
			if err != nil {
				return nil, fmt.Errorf("model %s (%s), %s field %q: %w", spec.Name, sourceLabel(spec.SourceFile), location, field.Name, err)
			}
			result[i] = normalized
		}
		return result, nil
	}
	var err error
	spec.Fields, err = resolveFields(spec.Fields, "base")
	if err != nil {
		return spec, err
	}
	for _, entry := range []struct {
		projection **ListSpec
		location   string
	}{{&spec.List, "list"}, {&spec.Detail, "detail"}} {
		if *entry.projection == nil {
			continue
		}
		copy := **entry.projection
		copy.Fields, err = resolveFields(copy.Fields, entry.location)
		if err != nil {
			return spec, err
		}
		// Defaults belong to the writable base field, not to a read projection.
		// Inherit enum defaults only; preserve historical non-enum generation.
		for i, field := range copy.Fields {
			for _, base := range spec.Fields {
				if enumFieldJSONName(base) != enumFieldJSONName(field) || (base.enumDefinition == nil && field.enumDefinition == nil) {
					continue
				}
				if base.enumDefinition == nil || field.enumDefinition == nil || base.Enum != field.Enum || base.Nullable != field.Nullable || base.JSONOmitEmpty != field.JSONOmitEmpty {
					return spec, fmt.Errorf("model %s: %s enum field %q must preserve the base field's enum, nullability, and optionality", spec.Name, entry.location, field.Name)
				}
				if field.FrontendDefault == nil && strings.TrimSpace(field.Default) == "" {
					copy.Fields[i].FrontendDefault = base.FrontendDefault
					copy.Fields[i].Default = base.Default
				}
			}
		}
		*entry.projection = &copy
	}
	return spec, nil
}

func enumFieldJSONName(field FieldSpec) string {
	if field.JSONName != "" {
		return field.JSONName
	}
	return field.Name
}

func resolveEnumField(field FieldSpec, registry map[string]EnumSpec) (FieldSpec, error) {
	field.enumDefinition = nil
	if field.Type != "enum" && field.Enum == "" {
		if field.FrontendDefault != nil {
			return field, fmt.Errorf("frontendDefault currently applies to enum fields only")
		}
		return field, nil
	}
	if strings.TrimSpace(field.Enum) == "" {
		return field, fmt.Errorf("type enum requires an enum registry name")
	}
	enum, exists := registry[field.Enum]
	if !exists {
		// Legacy backend fields supply their own Go type and registry name.
		// Frontend generation validates separately and never emits string fallback.
		if strings.TrimSpace(field.GoType) != "" && field.FrontendDefault == nil {
			return field, nil
		}
		return field, fmt.Errorf("references unknown enum %q; define enums[].values (legacy backend-only fields must supply goType)", field.Enum)
	}
	field.enumDefinition = &enum
	if field.GoType != "" && strings.TrimSpace(field.GoType) != enum.GoType && !(field.Nullable && strings.TrimSpace(field.GoType) == "*"+enum.GoType) {
		return field, fmt.Errorf("enum %q: goType %q conflicts with definition %q", enum.Name, field.GoType, enum.GoType)
	}
	if enum.SQLType != "" && field.SQLType != "" && strings.TrimSpace(field.SQLType) != enum.SQLType {
		return field, fmt.Errorf("enum %q: sqlType %q conflicts with definition %q", enum.Name, field.SQLType, enum.SQLType)
	}
	if field.GoType == "" {
		field.GoType = enum.GoType
	}
	if field.SQLType == "" {
		field.SQLType = enum.SQLType
	}
	if _, err := enumDefaultValue(field); err != nil {
		return field, err
	}
	return field, nil
}

func validateFrontendEnumDefinitions(spec ObjectSpec, enabled bool) error {
	if !enabled {
		return nil
	}
	groups := [][]FieldSpec{spec.Fields}
	if spec.List != nil {
		groups = append(groups, spec.List.Fields)
	}
	if spec.Detail != nil {
		groups = append(groups, spec.Detail.Fields)
	}
	for _, fields := range groups {
		for _, field := range fields {
			if field.JSON != nil && !*field.JSON {
				continue
			}
			if field.Type != "enum" && field.Enum == "" {
				continue
			}
			if field.enumDefinition == nil {
				return fmt.Errorf("model %s: frontend enum field %q references enum %q, but no enum values are defined", spec.Name, field.Name, field.Enum)
			}
			expectedType := frontendEnumFieldType(field)
			for _, explicit := range []string{field.TSType, field.TSDTOType} {
				if explicit != "" && strings.TrimSpace(explicit) != expectedType {
					return fmt.Errorf("model %s: enum field %q (%s) frontend type %q conflicts with %q", spec.Name, field.Name, field.Enum, explicit, expectedType)
				}
			}
			for _, expression := range []string{field.Valibot, field.ValibotDTO, field.ValibotModel} {
				if expression != "" && !containsTypeScriptIdentifier(expression, field.enumDefinition.FrontendName+"Schema") {
					return fmt.Errorf("model %s: enum field %q (%s) custom validation must use %sSchema", spec.Name, field.Name, field.Enum, field.enumDefinition.FrontendName)
				}
			}
		}
	}
	for _, formField := range spec.Frontend.Form.Fields {
		for _, field := range spec.Fields {
			if field.enumDefinition == nil || (field.Name != formField.Field && enumFieldJSONName(field) != formField.Field) {
				continue
			}
			switch strings.ToLower(strings.TrimSpace(formField.Component)) {
			case "text", "inputtext", "textarea", "number", "inputnumber", "checkbox", "bool", "boolean", "date", "datetime":
				return fmt.Errorf("model %s: enum form field %q (%s) requires a Select or a custom enum component, not %q", spec.Name, field.Name, field.Enum, formField.Component)
			}
			if formField.Default == nil {
				continue
			}
			value, ok := formField.Default.(string)
			if !ok || !enumContains(*field.enumDefinition, value) {
				return fmt.Errorf("model %s: enum form field %q (%s) has invalid default %v", spec.Name, field.Name, field.Enum, formField.Default)
			}
		}
	}
	return nil
}

func frontendEnumFieldType(field FieldSpec) string {
	name := field.enumDefinition.FrontendName
	if field.Nullable {
		return name + " | null"
	}
	return name
}

var enumSQLLiteral = regexp.MustCompile(`^'((?:[^']|'')*)'(?:\s*::\s*[A-Za-z_][A-Za-z0-9_.]*)?$`)

func enumDefaultValue(field FieldSpec) (*string, error) {
	if field.enumDefinition == nil {
		return nil, nil
	}
	var value *string
	if field.FrontendDefault != nil {
		value = field.FrontendDefault
	} else if match := enumSQLLiteral.FindStringSubmatch(strings.TrimSpace(field.Default)); match != nil {
		literal := strings.ReplaceAll(match[1], "''", "'")
		value = &literal
	}
	if value != nil && !enumContains(*field.enumDefinition, *value) {
		return nil, fmt.Errorf("enum %q: default %q is not one of its values", field.Enum, *value)
	}
	return value, nil
}

func enumContains(enum EnumSpec, value string) bool {
	for _, item := range enum.Values {
		if item.Value == value {
			return true
		}
	}
	return false
}
