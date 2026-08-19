package codegen

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

var commonSchemaNames = []string{
	"RequiredStringSchema",
	"StringSchema",
	"RequiredTextSchema",
	"TextSchema",
	"DateStringSchema",
	"IntSchema",
	"IdSchema",
	"NumberSchema",
	"AttrsSchema",
	"YearSchema",
}

func buildFrontendView(spec ObjectSpec, object ObjectView) FrontendView {
	frontend := FrontendView{
		Scaffold:          spec.Frontend.Scaffold,
		Title:             strings.TrimSpace(spec.Frontend.Title),
		TypeImports:       buildFrontendImports(spec.Frontend.TypeImports),
		SchemaImports:     buildFrontendImports(spec.Frontend.SchemaImports),
		ListTypeImports:   buildFrontendImports(spec.Frontend.ListTypeImports),
		ListSchemaImports: buildFrontendImports(spec.Frontend.ListSchemaImports),
	}
	if frontend.Title == "" {
		frontend.Title = strings.TrimSpace(object.ApplicationRoute.Description)
	}
	if frontend.Title == "" {
		frontend.Title = object.HumanPlural
	}

	frontend.Routes = buildFrontendRoutes(spec, object)
	frontend.Form = buildFrontendForm(spec, object)
	frontend.List = buildFrontendList(spec, object)
	frontend.LocaleFields = buildFrontendLocaleFields(frontend.Form, frontend.List)

	return frontend
}

func buildFrontendLocaleFields(form FrontendFormView, list FrontendListView) []FrontendLocaleFieldView {
	labels := make(map[string]string)
	for _, field := range form.Fields {
		labels[field.Field.TSName] = field.Label
	}
	for _, column := range list.Columns {
		if _, exists := labels[column.Field.TSName]; !exists {
			labels[column.Field.TSName] = column.Label
		}
	}
	names := make([]string, 0, len(labels))
	for name := range labels {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]FrontendLocaleFieldView, 0, len(names))
	for _, name := range names {
		result = append(result, FrontendLocaleFieldView{Name: name, Label: labels[name]})
	}
	return result
}

func frontendTitle(spec ObjectSpec, object ObjectView) string {
	title := strings.TrimSpace(spec.Frontend.Title)
	if title == "" {
		title = strings.TrimSpace(object.ApplicationRoute.Description)
	}
	if title == "" {
		title = object.HumanPlural
	}
	return title
}

func buildFrontendImports(specs []FrontendImportSpec) []FrontendImportView {
	imports := make([]FrontendImportView, 0, len(specs))
	for _, item := range specs {
		names := append([]string(nil), item.Names...)
		sort.Strings(names)
		imports = append(imports, FrontendImportView{
			From:     strings.TrimSpace(item.From),
			Names:    names,
			TypeOnly: item.TypeOnly,
		})
	}
	sort.Slice(imports, func(i int, j int) bool {
		if imports[i].From == imports[j].From {
			return strings.Join(imports[i].Names, ",") < strings.Join(imports[j].Names, ",")
		}
		return imports[i].From < imports[j].From
	})
	return imports
}

func buildFrontendRoutes(spec ObjectSpec, object ObjectView) FrontendRoutesView {
	listName := strings.TrimSpace(spec.Frontend.Routes.ListName)
	if listName == "" && object.ApplicationRoute.Enabled {
		listName = object.ApplicationRoute.Name
	}
	if listName == "" {
		listName = CamelCase(object.SnakePlural)
	}

	createName := strings.TrimSpace(spec.Frontend.Routes.CreateName)
	if createName == "" {
		createName = object.Camel + "Create"
	}
	editName := strings.TrimSpace(spec.Frontend.Routes.EditName)
	if editName == "" {
		editName = object.Camel + "Edit"
	}

	listPath := object.Route
	if object.ApplicationRoute.Enabled && strings.TrimSpace(object.ApplicationRoute.Path) != "" {
		listPath = object.ApplicationRoute.Path
	}
	editPath := listPath
	for _, key := range object.Keys {
		editPath += "/:" + key.PathName
	}

	return FrontendRoutesView{
		ListName:   listName,
		CreateName: createName,
		EditName:   editName,
		ListPath:   listPath,
		CreatePath: listPath + "/new",
		EditPath:   editPath,
	}
}

func buildFrontendForm(spec ObjectSpec, object ObjectView) FrontendFormView {
	enabled := spec.Frontend.Scaffold && object.CRUD.Create && object.CRUD.Detail && object.CRUD.Update
	if spec.Frontend.Form.Enabled != nil {
		enabled = *spec.Frontend.Form.Enabled
	}

	columns := spec.Frontend.Form.Columns
	if columns == 0 {
		columns = 2
	}

	view := FrontendFormView{
		Enabled:     enabled,
		Columns:     columns,
		CreateTitle: strings.TrimSpace(spec.Frontend.Form.CreateTitle),
		EditTitle:   strings.TrimSpace(spec.Frontend.Form.EditTitle),
		CopyTitle:   strings.TrimSpace(spec.Frontend.Form.CopyTitle),
	}
	if view.CreateTitle == "" {
		view.CreateTitle = frontendTitle(spec, object)
	}
	if view.EditTitle == "" {
		view.EditTitle = frontendTitle(spec, object)
	}
	if view.CopyTitle == "" {
		view.CopyTitle = frontendTitle(spec, object)
	}

	fieldByName := frontendFieldMap(object.FrontendFields)
	formSpecs := spec.Frontend.Form.Fields
	if len(formSpecs) == 0 {
		formSpecs = make([]FrontendFormFieldSpec, 0, len(object.FrontendCreateFields))
		for _, field := range object.FrontendCreateFields {
			formSpecs = append(formSpecs, FrontendFormFieldSpec{Field: field.TSName})
		}
	}

	imports := map[string]string{}
	for _, formSpec := range formSpecs {
		field, exists := fieldByName[formSpec.Field]
		if !exists {
			continue
		}
		component, componentImport := frontendFormComponent(field, formSpec)
		if componentImport != "" {
			imports[component] = componentImport
		}
		readOnly := field.SrvCalc || field.ReadOnly || field.ServerGenerated || field.AutoIncrement
		if formSpec.ReadOnly != nil {
			readOnly = *formSpec.ReadOnly
		}
		disableExpression := "false"
		if readOnly {
			disableExpression = "true"
		} else if field.PrimaryKey {
			disableExpression = `props.mode !== "create"`
		}
		columnSpan := formSpec.ColumnSpan
		if columnSpan == 0 {
			columnSpan = 1
		}
		view.Fields = append(view.Fields, FrontendFormFieldView{
			Field:             field,
			Component:         component,
			Label:             frontendFieldLabel(formSpec.Label, field),
			DefaultLiteral:    frontendDefaultLiteral(formSpec.Default, field),
			ReadOnly:          readOnly,
			Hidden:            formSpec.Hidden,
			Autofocus:         formSpec.Autofocus,
			ColumnSpan:        columnSpan,
			ID:                object.Camel + PascalCase(field.TSName),
			SubmitExpression:  frontendSubmitExpression(field),
			DisableExpression: disableExpression,
			CustomComponent:   isCustomFrontendComponent(formSpec.Component),
		})
		if field.Nullable && isTextLikeType(field.Type) {
			view.NeedsNullableText = true
		}
		if isDateLikeType(field.Type) {
			view.NeedsFormatDate = true
		}
	}

	componentNames := make([]string, 0, len(imports))
	for name := range imports {
		componentNames = append(componentNames, name)
	}
	sort.Strings(componentNames)
	for _, name := range componentNames {
		view.Imports = append(view.Imports, FrontendComponentImportView{
			Name: name,
			From: imports[name],
		})
	}
	return view
}

func buildFrontendList(spec ObjectSpec, object ObjectView) FrontendListView {
	enabled := spec.Frontend.Scaffold && object.CRUD.List
	if spec.Frontend.List.Enabled != nil {
		enabled = *spec.Frontend.List.Enabled
	}
	pageSize := spec.Frontend.List.PageSize
	if pageSize == 0 {
		pageSize = 30
	}
	editMode := strings.TrimSpace(spec.Frontend.List.EditMode)
	if editMode == "" {
		editMode = "page"
	}

	view := FrontendListView{
		Enabled:         enabled,
		PageSize:        pageSize,
		EditMode:        editMode,
		Inline:          editMode == "inline",
		CanInlineCreate: editMode == "inline" && object.CRUD.Create,
	}
	fieldByName := frontendFieldMap(object.FrontendListFields)
	updateFields := make(map[string]struct{}, len(object.FrontendUpdateFields))
	for _, field := range object.FrontendUpdateFields {
		updateFields[field.TSName] = struct{}{}
	}
	columnSpecs := spec.Frontend.List.Columns
	if len(columnSpecs) == 0 {
		columnSpecs = make([]FrontendListColumnSpec, 0, len(object.FrontendListFields))
		for _, field := range object.FrontendListFields {
			columnSpecs = append(columnSpecs, FrontendListColumnSpec{Field: field.TSName})
		}
	}

	for _, columnSpec := range columnSpecs {
		field, exists := fieldByName[columnSpec.Field]
		if !exists {
			continue
		}
		sortable := true
		if columnSpec.Sortable != nil {
			sortable = *columnSpec.Sortable
		}
		_, writable := updateFields[field.TSName]
		editable := view.Inline && writable && supportsGeneratedInlineEditor(field)
		if columnSpec.Editable != nil {
			editable = *columnSpec.Editable
		}
		dataType, align, format := frontendColumnPresentation(field)
		if strings.TrimSpace(columnSpec.DataType) != "" {
			dataType = strings.TrimSpace(columnSpec.DataType)
		}
		if strings.TrimSpace(columnSpec.Align) != "" {
			align = strings.TrimSpace(columnSpec.Align)
		}
		width := strings.TrimSpace(columnSpec.Width)
		if width == "" {
			width = frontendColumnWidth(field)
		}
		view.Columns = append(view.Columns, FrontendListColumnView{
			Field:    field,
			Label:    frontendFieldLabel(columnSpec.Label, field),
			Width:    width,
			Sortable: sortable,
			Editable: editable,
			DataType: dataType,
			Align:    align,
			Format:   format,
		})
		if format == "formatDate" {
			view.NeedsFormatDate = true
		}
	}
	if view.CanInlineCreate {
		listFields := make(map[string]FieldView, len(object.FrontendListFields))
		for _, field := range object.FrontendListFields {
			listFields[field.TSName] = field
			view.CreateRowFields = append(view.CreateRowFields, FrontendInlineCreateFieldView{
				Field:          field,
				DefaultLiteral: frontendInlineDraftLiteral(field),
			})
		}
		for _, field := range object.FrontendCreateFields {
			expression := frontendInlineDraftLiteral(field)
			if _, exists := listFields[field.TSName]; exists {
				expression = "row." + field.TSName
			}
			view.CreateModelFields = append(view.CreateModelFields, FrontendInlineCreateModelFieldView{
				Field:      field,
				Expression: expression,
			})
		}
	}
	return view
}

func supportsGeneratedInlineEditor(field FieldView) bool {
	switch field.Type {
	case "string", "text", "enum", "password", "time", "int", "bigint", "float", "numeric", "bool":
		return true
	default:
		return false
	}
}

func hasSafeInlineCreateDefault(field FieldView) bool {
	return field.Nullable || strings.TrimSpace(field.Default) != ""
}

func frontendInlineDraftLiteral(field FieldView) string {
	defaultValue := strings.TrimSpace(field.Default)

	switch field.Type {
	case "bool":
		switch strings.ToLower(defaultValue) {
		case "true":
			return "true"
		case "false":
			return "false"
		}
		if field.Nullable {
			return "null"
		}
		return "false"
	case "int", "bigint", "float", "numeric":
		if defaultValue != "" {
			if _, err := strconv.ParseFloat(defaultValue, 64); err == nil {
				return defaultValue
			}
		}
		if field.Nullable {
			return "null"
		}
		return "0"
	case "json", "jsonb":
		if field.Nullable {
			return "null"
		}
		return "{}"
	case "date", "datetime", "timestamptz":
		if field.Nullable {
			return "null"
		}
		if strings.Contains(strings.ToLower(defaultValue), "now") || strings.Contains(strings.ToLower(defaultValue), "current_") {
			return "new Date()"
		}
		return "new Date(0)"
	default:
		if field.Nullable {
			return "null"
		}
		return `""`
	}
}

func frontendFieldMap(fields []FieldView) map[string]FieldView {
	result := make(map[string]FieldView, len(fields)*2)
	for _, field := range fields {
		result[field.Name] = field
		result[field.TSName] = field
	}
	return result
}

func isBuiltInFrontendComponent(component string) bool {
	switch strings.ToLower(strings.TrimSpace(component)) {
	case "", "text", "inputtext", "textarea", "number", "inputnumber", "checkbox", "bool", "boolean", "date", "datetime":
		return true
	default:
		return false
	}
}

func isCustomFrontendComponent(component string) bool {
	return strings.TrimSpace(component) != "" && !isBuiltInFrontendComponent(component)
}

func frontendFormComponent(field FieldView, spec FrontendFormFieldSpec) (string, string) {
	component := strings.TrimSpace(spec.Component)
	componentImport := strings.TrimSpace(spec.ComponentImport)
	if component != "" {
		switch strings.ToLower(component) {
		case "text", "inputtext":
			return "InputText", "primevue/inputtext"
		case "textarea":
			return "Textarea", "primevue/textarea"
		case "number", "inputnumber":
			return "InputNumber", "primevue/inputnumber"
		case "checkbox", "bool", "boolean":
			return "Checkbox", "primevue/checkbox"
		case "date", "datetime":
			return "InputText", "primevue/inputtext"
		default:
			return component, componentImport
		}
	}

	switch field.Type {
	case "int", "bigint", "float", "numeric":
		return "InputNumber", "primevue/inputnumber"
	case "bool":
		return "Checkbox", "primevue/checkbox"
	case "text":
		return "Textarea", "primevue/textarea"
	case "date", "datetime", "timestamptz":
		return "InputText", "primevue/inputtext"
	default:
		return "InputText", "primevue/inputtext"
	}
}

func frontendFieldLabel(explicit string, field FieldView) string {
	if strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit)
	}
	return HumanName(field.TSName)
}

func frontendDefaultLiteral(explicit any, field FieldView) string {
	if explicit != nil {
		return tsLiteral(explicit)
	}
	if field.Nullable {
		return "null"
	}
	if field.ServerGenerated && isDateLikeType(field.Type) {
		return "undefined"
	}
	switch field.Type {
	case "int", "bigint", "float", "numeric":
		return "0"
	case "bool":
		return "false"
	case "json", "jsonb":
		return "{}"
	case "date", "datetime", "timestamptz":
		return "undefined"
	default:
		return `""`
	}
}

func frontendSubmitExpression(field FieldView) string {
	name := "form.value." + field.TSName
	switch field.Type {
	case "string", "text", "enum", "password", "time":
		if field.Nullable {
			return "nullableText(" + name + ")"
		}
		return name + `?.trim() ?? ""`
	case "int", "bigint", "float", "numeric":
		if field.Nullable {
			return name + " ?? null"
		}
		return name + " ?? 0"
	case "bool":
		return name + " ?? false"
	case "json", "jsonb":
		if field.Nullable {
			return name + " ?? null"
		}
		return name + " ?? {}"
	default:
		return name
	}
}

func frontendColumnPresentation(field FieldView) (string, string, string) {
	switch field.Type {
	case "int", "bigint", "float", "numeric":
		return "number", "right", ""
	case "bool":
		return "boolean", "", ""
	case "date", "datetime", "timestamptz":
		return "date", "", "formatDate"
	default:
		return "", "", ""
	}
}

func frontendColumnWidth(field FieldView) string {
	switch field.Type {
	case "int", "bigint", "float", "numeric":
		return "9rem"
	case "bool":
		return "8rem"
	case "date", "datetime", "timestamptz":
		return "14rem"
	case "text":
		return "30rem"
	default:
		return "16rem"
	}
}

func tsLiteral(value any) string {
	switch typed := value.(type) {
	case string:
		return strconv.Quote(typed)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case nil:
		return "undefined"
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return "undefined"
		}
		return string(encoded)
	}
}

func commonSchemasForFields(fields []FieldView) []string {
	used := make(map[string]struct{})
	for _, field := range fields {
		for _, expression := range []string{field.ValibotDTO, field.ValibotModel} {
			for _, name := range commonSchemaNames {
				if containsTypeScriptIdentifier(expression, name) {
					used[name] = struct{}{}
				}
			}
		}
	}
	result := make([]string, 0, len(used))
	for _, name := range commonSchemaNames {
		if _, exists := used[name]; exists {
			result = append(result, name)
		}
	}
	return result
}

func containsTypeScriptIdentifier(expression string, name string) bool {
	for start := 0; ; {
		index := strings.Index(expression[start:], name)
		if index < 0 {
			return false
		}
		index += start
		beforeOK := index == 0 || !isTypeScriptIdentifierPart(rune(expression[index-1]))
		afterIndex := index + len(name)
		afterOK := afterIndex == len(expression) || !isTypeScriptIdentifierPart(rune(expression[afterIndex]))
		if beforeOK && afterOK {
			return true
		}
		start = index + len(name)
	}
}

func isTypeScriptIdentifierPart(char rune) bool {
	return char == '_' || char == '$' || char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9'
}

func validateFrontendIdentifiers(object ObjectView) error {
	seen := make(map[string]struct{})
	for _, field := range append(append([]FieldView(nil), object.FrontendFields...), object.FrontendListFields...) {
		if _, exists := seen[field.TSName]; exists {
			continue
		}
		seen[field.TSName] = struct{}{}
		if !isTypeScriptIdentifier(field.TSName) {
			return fmt.Errorf("frontend JSON name %q for field %s is not a valid TypeScript property identifier", field.TSName, field.Name)
		}
	}
	for _, key := range object.FrontendKeys {
		if !isTypeScriptIdentifier(key.TSName) {
			return fmt.Errorf("frontend JSON name %q for key %s is not a valid TypeScript property identifier", key.TSName, key.Name)
		}
	}
	return nil
}

func isTypeScriptIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index, char := range value {
		if index == 0 {
			if char == '_' || char == '$' || char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' {
				continue
			}
			return false
		}
		if isTypeScriptIdentifierPart(char) {
			continue
		}
		return false
	}
	return true
}

func validateFrontendView(object ObjectView) error {
	if !object.Frontend.Scaffold {
		return nil
	}
	if object.Frontend.Form.Enabled && len(object.Frontend.Form.Fields) == 0 {
		return fmt.Errorf("frontend form is enabled but no JSON-visible fields are available")
	}
	if object.Frontend.Form.Enabled && !(object.CRUD.Create && object.CRUD.Detail && object.CRUD.Update) {
		return fmt.Errorf("generated frontend forms currently require create, detail, and update CRUD operations")
	}
	if object.Frontend.List.Enabled && len(object.Frontend.List.Columns) == 0 {
		return fmt.Errorf("frontend list is enabled but no JSON-visible list fields are available")
	}
	if object.Frontend.List.Enabled && object.CompositeKey {
		return fmt.Errorf("generated CollectionGrid pages require a single key; composite-key APIs are supported, but their list pages must remain hand-written")
	}
	if object.Frontend.List.Enabled {
		listFields := make(map[string]struct{}, len(object.FrontendListFields))
		for _, field := range object.FrontendListFields {
			listFields[field.TSName] = struct{}{}
		}
		for _, key := range object.FrontendKeys {
			if _, exists := listFields[key.TSName]; !exists {
				return fmt.Errorf("generated frontend list projection must include key field %s", key.TSName)
			}
		}
	}
	if object.Frontend.List.Enabled && object.Frontend.List.Inline {
		if !object.CRUD.Update {
			return fmt.Errorf("frontend inline list editing requires crud.update: true")
		}

		updateFields := make(map[string]struct{}, len(object.FrontendUpdateFields))
		for _, field := range object.FrontendUpdateFields {
			updateFields[field.TSName] = struct{}{}
		}
		editableFields := make(map[string]struct{})
		for _, column := range object.Frontend.List.Columns {
			if !column.Editable {
				continue
			}
			if _, exists := updateFields[column.Field.TSName]; !exists {
				return fmt.Errorf("frontend inline list column %s is editable but is not a writable update field", column.Field.TSName)
			}
			if !supportsGeneratedInlineEditor(column.Field) {
				return fmt.Errorf("frontend inline list column %s uses field type %s, which requires a custom inline editor not yet supported by generated grids", column.Field.TSName, column.Field.Type)
			}
			editableFields[column.Field.TSName] = struct{}{}
		}
		if len(editableFields) == 0 {
			return fmt.Errorf("frontend inline list editing requires at least one editable column")
		}

		if object.CRUD.Create {
			for _, field := range object.FrontendCreateFields {
				if _, exists := editableFields[field.TSName]; exists {
					continue
				}
				if !hasSafeInlineCreateDefault(field) {
					return fmt.Errorf("frontend inline creation requires writable create field %s to be editable, nullable, or have a default", field.TSName)
				}
			}
		}
	}
	if object.Frontend.Form.Enabled {
		formFields := make(map[string]struct{}, len(object.Frontend.Form.Fields))
		for _, field := range object.Frontend.Form.Fields {
			formFields[field.Field.TSName] = struct{}{}
			if (field.Field.Type == "json" || field.Field.Type == "jsonb") && !field.CustomComponent {
				return fmt.Errorf("generated frontend form field %s requires a custom JSON editor component", field.Field.TSName)
			}
			if isDateLikeType(field.Field.Type) && !field.ReadOnly && !field.CustomComponent {
				return fmt.Errorf("generated writable date form field %s requires a custom date component", field.Field.TSName)
			}
		}
		for _, field := range object.FrontendCreateFields {
			if _, exists := formFields[field.TSName]; !exists {
				return fmt.Errorf("generated frontend form must include writable create field %s", field.TSName)
			}
		}
	}
	return nil
}
