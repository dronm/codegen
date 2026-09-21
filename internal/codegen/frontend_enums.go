package codegen

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type FrontendEnumView struct {
	Name         string
	FrontendName string
	FileBase     string
	ValuesName   string
	Values       []EnumValueSpec
}

type FrontendPropertyView struct {
	Name    string
	Literal string
}

type FrontendLocaleView struct {
	Objects []ObjectView
	Enums   []FrontendEnumView
}

func buildFrontendEnum(enum EnumSpec) FrontendEnumView {
	return FrontendEnumView{
		Name: enum.Name, FrontendName: enum.FrontendName,
		FileBase:   CamelCase(enum.FrontendName),
		ValuesName: strings.ToUpper(SnakeCase(enum.FrontendName)) + "_VALUES",
		Values:     append([]EnumValueSpec(nil), enum.Values...),
	}
}

func frontendEnumsForFields(fields []FieldView) []FrontendEnumView {
	byName := make(map[string]FrontendEnumView)
	for _, field := range fields {
		if field.EnumDefinition != nil {
			byName[field.EnumDefinition.Name] = *field.EnumDefinition
		}
	}
	return sortedFrontendEnums(byName)
}

func sortedFrontendEnums(byName map[string]FrontendEnumView) []FrontendEnumView {
	result := make([]FrontendEnumView, 0, len(byName))
	for _, enum := range byName {
		result = append(result, enum)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].FrontendName < result[j].FrontendName })
	return result
}

func collectFrontendEnums(objects []ObjectView) []FrontendEnumView {
	byName := make(map[string]FrontendEnumView)
	for _, object := range objects {
		for _, fields := range [][]FieldView{object.FrontendFields, object.FrontendDetailFields, object.FrontendListFields} {
			for _, enum := range frontendEnumsForFields(fields) {
				byName[enum.Name] = enum
			}
		}
	}
	return sortedFrontendEnums(byName)
}

func withFrontendEnumImports(imports []FrontendImportView, enums []FrontendEnumView, schema bool) []FrontendImportView {
	if len(enums) == 0 {
		return imports
	}
	result := append([]FrontendImportView(nil), imports...)
	for _, enum := range enums {
		name, from := enum.FrontendName, "@/types/enums/"+enum.FileBase+".gen"
		if schema {
			name, from = "create"+enum.FrontendName+"Schemas", "@/schemas/enums/"+enum.FileBase+".gen"
		}
		found := false
		for i := range result {
			if result[i].From != from || result[i].TypeOnly != !schema {
				continue
			}
			found = true
			names := append([]string(nil), result[i].Names...)
			if !containsString(names, name) {
				names = append(names, name)
			}
			sort.Strings(names)
			result[i].Names = names
			break
		}
		if !found {
			result = append(result, FrontendImportView{From: from, Names: []string{name}, TypeOnly: !schema})
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].From < result[j].From })
	return result
}

func containsString(items []string, wanted string) bool {
	for _, item := range items {
		if item == wanted {
			return true
		}
	}
	return false
}

func validateEnumNamespaces(objects []ObjectView) error {
	modelNames := make(map[string]string)
	for _, object := range objects {
		for _, name := range []string{object.Name, object.ListModelName, object.DetailModelName} {
			modelNames[name] = object.Name
			modelNames[name+"DTO"] = object.Name
		}
		for _, suffix := range []string{"Key", "New", "Upd", "Update", "FormModel"} {
			modelNames[object.Name+suffix] = object.Name
		}
	}
	for _, enum := range collectFrontendEnums(objects) {
		if owner, exists := modelNames[enum.FrontendName]; exists {
			return fmt.Errorf("enum %q frontendName %q conflicts with model %s and its generated schema/locale namespace", enum.Name, enum.FrontendName, owner)
		}
		for _, common := range commonSchemaNames {
			if enum.FrontendName+"Schema" == common {
				return fmt.Errorf("enum %q frontendName %q conflicts with common schema %s", enum.Name, enum.FrontendName, common)
			}
		}
	}
	return nil
}

func validateFrontendEnumView(object ObjectView) error {
	for _, imports := range [][]FrontendImportView{
		object.Frontend.TypeImports, object.Frontend.SchemaImports,
		object.Frontend.DetailTypeImports, object.Frontend.DetailSchemaImports,
		object.Frontend.ListTypeImports, object.Frontend.ListSchemaImports,
	} {
		owners := make(map[string]string)
		for _, item := range imports {
			for _, name := range item.Names {
				if previous, exists := owners[name]; exists {
					return fmt.Errorf("model %s: duplicate frontend import %s from %q and %q", object.Name, name, previous, item.From)
				}
				owners[name] = item.From
			}
		}
	}
	if err := validateEnumNamespaces([]ObjectView{object}); err != nil {
		return err
	}
	if !object.Frontend.List.Enabled || !object.Frontend.List.CanInlineCreate {
		return nil
	}
	for _, group := range [][]FieldView{object.FrontendListFields, object.FrontendCreateFields} {
		for _, field := range group {
			if field.EnumDefinition == nil || field.EnumDefault != nil || field.Nullable || field.TSOptional {
				continue
			}
			return fmt.Errorf("model %s: inline enum field %q (%s) requires an explicit valid frontendDefault (or a literal SQL default); a non-nullable enum draft cannot use an empty string or the first option automatically", object.Name, field.TSName, field.Enum)
		}
	}
	return nil
}

func validateEnumColumnSpec(column FrontendListColumnSpec, fields map[string]FieldSpec) error {
	var field FieldSpec
	for _, candidate := range fields {
		if candidate.Name == column.Field || enumFieldJSONName(candidate) == column.Field {
			field = candidate
			break
		}
	}
	isEnum := field.enumDefinition != nil
	if isEnum && (column.Reference != nil || (column.DataType != "" && column.DataType != "enum")) {
		return fmt.Errorf("frontend enum column %q (%s) must use dataType enum and cannot use a reference editor", column.Field, field.Enum)
	}
	if column.DataType == "enum" && !isEnum {
		return fmt.Errorf("frontend enum column %q requires a field with a defined enum and values", column.Field)
	}
	if column.SearchDataType != "" {
		switch column.SearchDataType {
		case "string", "number", "boolean", "date", "datetime", "reference", "enum":
		default:
			return fmt.Errorf("frontend column %q: unsupported searchDataType %q", column.Field, column.SearchDataType)
		}
	}
	for _, operation := range column.SearchOperations {
		switch operation {
		case "begins", "contains", "exact", "range":
		default:
			return fmt.Errorf("frontend column %q: unsupported search operation %q", column.Field, operation)
		}
	}
	if value, exists := column.EditorProps["showClear"]; exists {
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("frontend column %q editorProps.showClear must be a boolean", column.Field)
		}
	}
	if _, err := json.Marshal(column.EditorProps); err != nil {
		return fmt.Errorf("frontend column %q editorProps must contain JSON-compatible values: %w", column.Field, err)
	}
	return nil
}

func frontendEnumEditorProps(field FieldView, explicit map[string]any) []FrontendPropertyView {
	values := make(map[string]any, len(explicit)+1)
	if field.EnumDefinition != nil {
		// Clearing produces null. Optional-but-non-nullable fields still cannot
		// accept null, even when their property can be absent in a DTO.
		values["showClear"] = field.Nullable && !field.Required
	}
	for name, value := range explicit {
		values[name] = value
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]FrontendPropertyView, 0, len(names))
	for _, name := range names {
		literal, _ := json.Marshal(values[name]) // validated before rendering
		result = append(result, FrontendPropertyView{Name: name, Literal: string(literal)})
	}
	return result
}
