package codegen

import (
	"fmt"
	"go/token"
	"strconv"
	"strings"
	"unicode"
)

type ObjectSpec struct {
	SourceFile        string               `yaml:"-"`
	Name              string               `yaml:"name"`
	Comment           string               `yaml:"comment"`
	HumanName         string               `yaml:"humanName"`
	HumanNamePlural   string               `yaml:"humanNamePlural"`
	Table             TableSpec            `yaml:"table"`
	List              *ListSpec            `yaml:"list"`
	Route             string               `yaml:"route"`
	PermissionPrefix  string               `yaml:"permissionPrefix"`
	ServiceName       string               `yaml:"serviceName"`
	SessionRequired   *bool                `yaml:"sessionRequired"`
	CRUDNotifications *bool                `yaml:"crudNotifications"`
	Keys              []KeySpec            `yaml:"keys"`
	Fields            []FieldSpec          `yaml:"fields"`
	CRUD              CRUDSpec             `yaml:"crud"`
	Permissions       PermissionSpec       `yaml:"permissions"`
	ApplicationRoute  ApplicationRouteSpec `yaml:"applicationRoute"`
	Menu              MenuSpec             `yaml:"menu"`
	Migration         MigrationSpec        `yaml:"migration"`
	Test              TestSpec             `yaml:"test"`
	Frontend          FrontendSpec         `yaml:"frontend"`
}

type TableSpec struct {
	Schema  string   `yaml:"schema"`
	Name    string   `yaml:"name"`
	Checks  []string `yaml:"checks"`
	Comment string   `yaml:"comment"`
}

type ListSpec struct {
	Model   string      `yaml:"model"`
	Table   TableSpec   `yaml:"table"`
	Fields  []FieldSpec `yaml:"fields"`
	Comment string      `yaml:"comment"`
}

type KeySpec struct {
	Name     string `yaml:"name"`
	PathName string `yaml:"pathName"`
	Type     string `yaml:"type"`
	GoType   string `yaml:"goType"`
}

type FieldSpec struct {
	Name            string         `yaml:"name"`
	Type            string         `yaml:"type"`
	SQLType         string         `yaml:"sqlType"`
	GoType          string         `yaml:"goType"`
	TSType          string         `yaml:"tsType"`
	TSDTOType       string         `yaml:"tsDtoType"`
	Valibot         string         `yaml:"valibot"`
	ValibotDTO      string         `yaml:"valibotDto"`
	ValibotModel    string         `yaml:"valibotModel"`
	Comment         string         `yaml:"comment"`
	PrimaryKey      bool           `yaml:"primaryKey"`
	Required        bool           `yaml:"required"`
	DBRequired      bool           `yaml:"dbRequired"`
	Nullable        bool           `yaml:"nullable"`
	AutoIncrement   bool           `yaml:"autoIncrement"`
	ServerGenerated bool           `yaml:"serverGenerated"`
	SrvCalc         bool           `yaml:"srvCalc"`
	ReadOnly        bool           `yaml:"readOnly"`
	Default         string         `yaml:"default"`
	Length          int            `yaml:"length"`
	JSON            *bool          `yaml:"json"`
	JSONName        string         `yaml:"jsonName"`
	JSONOmitEmpty   bool           `yaml:"jsonOmitEmpty"`
	Filter          string         `yaml:"filter"`
	Enum            string         `yaml:"enum"`
	MaxLen          ScalarString   `yaml:"maxLen"`
	Unique          bool           `yaml:"unique"`
	Check           string         `yaml:"check"`
	References      *ReferenceSpec `yaml:"references"`

	// Legacy schema fields kept for compatibility with the first generator version.
	Alias    string       `yaml:"alias"`
	DateType string       `yaml:"dateType"`
	Agg      string       `yaml:"agg"`
	Max      ScalarString `yaml:"max"`
	Min      ScalarString `yaml:"min"`
	Fix      ScalarString `yaml:"fix"`
	RegExp   string       `yaml:"regExp"`
}

type FrontendSpec struct {
	Scaffold          bool                 `yaml:"scaffold"`
	Title             string               `yaml:"title"`
	TypeImports       []FrontendImportSpec `yaml:"typeImports"`
	SchemaImports     []FrontendImportSpec `yaml:"schemaImports"`
	ListTypeImports   []FrontendImportSpec `yaml:"listTypeImports"`
	ListSchemaImports []FrontendImportSpec `yaml:"listSchemaImports"`
	Form              FrontendFormSpec     `yaml:"form"`
	List              FrontendListSpec     `yaml:"list"`
	Routes            FrontendRoutesSpec   `yaml:"routes"`
}

type FrontendImportSpec struct {
	From     string   `yaml:"from"`
	Names    []string `yaml:"names"`
	TypeOnly bool     `yaml:"typeOnly"`
}

type FrontendFormSpec struct {
	Enabled     *bool                   `yaml:"enabled"`
	Columns     int                     `yaml:"columns"`
	CreateTitle string                  `yaml:"createTitle"`
	EditTitle   string                  `yaml:"editTitle"`
	CopyTitle   string                  `yaml:"copyTitle"`
	Fields      []FrontendFormFieldSpec `yaml:"fields"`
}

type FrontendFormFieldSpec struct {
	Field           string `yaml:"field"`
	Component       string `yaml:"component"`
	ComponentImport string `yaml:"componentImport"`
	Label           string `yaml:"label"`
	Default         any    `yaml:"default"`
	ReadOnly        *bool  `yaml:"readOnly"`
	Hidden          bool   `yaml:"hidden"`
	Autofocus       bool   `yaml:"autofocus"`
	ColumnSpan      int    `yaml:"columnSpan"`
}

type FrontendListSpec struct {
	Enabled  *bool                    `yaml:"enabled"`
	PageSize int                      `yaml:"pageSize"`
	EditMode string                   `yaml:"editMode"`
	Columns  []FrontendListColumnSpec `yaml:"columns"`
}

type FrontendListColumnSpec struct {
	Field    string `yaml:"field"`
	Label    string `yaml:"label"`
	Width    string `yaml:"width"`
	Sortable *bool  `yaml:"sortable"`
	Editable *bool  `yaml:"editable"`
	DataType string `yaml:"dataType"`
	Align    string `yaml:"align"`
}

type FrontendRoutesSpec struct {
	ListName   string `yaml:"listName"`
	CreateName string `yaml:"createName"`
	EditName   string `yaml:"editName"`
}

type ReferenceSpec struct {
	Schema   string `yaml:"schema"`
	Table    string `yaml:"table"`
	Column   string `yaml:"column"`
	OnDelete string `yaml:"onDelete"`
	OnUpdate string `yaml:"onUpdate"`
}

type IndexSpec struct {
	Name    string   `yaml:"name"`
	Columns []string `yaml:"columns"`
	Unique  bool     `yaml:"unique"`
	Where   string   `yaml:"where"`
	Method  string   `yaml:"method"`
}

type MigrationSpec struct {
	Enabled          *bool       `yaml:"enabled"`
	Name             string      `yaml:"name"`
	Indexes          []IndexSpec `yaml:"indexes"`
	UpdatedAtTrigger string      `yaml:"updatedAtTrigger"`
}

type PermissionSpec struct {
	Enabled      *bool             `yaml:"enabled"`
	GrantRoles   []string          `yaml:"grantRoles"`
	Descriptions map[string]string `yaml:"descriptions"`
}

type ApplicationRouteSpec struct {
	Enabled       bool    `yaml:"enabled"`
	Name          string  `yaml:"name"`
	Path          string  `yaml:"path"`
	Description   string  `yaml:"description"`
	Section       string  `yaml:"section"`
	Icon          *string `yaml:"icon"`
	MenuAvailable *bool   `yaml:"menuAvailable"`
}

type MenuSpec struct {
	Enabled   bool            `yaml:"enabled"`
	Role      string          `yaml:"role"`
	Caption   string          `yaml:"caption"`
	Icon      *string         `yaml:"icon"`
	SortOrder int             `yaml:"sortOrder"`
	Parent    *MenuParentSpec `yaml:"parent"`
	IsActive  *bool           `yaml:"isActive"`
}

type MenuParentSpec struct {
	Caption   string  `yaml:"caption"`
	Icon      *string `yaml:"icon"`
	SortOrder int     `yaml:"sortOrder"`
	Create    *bool   `yaml:"create"`
}

type ScalarString string

func (v *ScalarString) UnmarshalYAML(unmarshal func(any) error) error {
	var raw any
	if err := unmarshal(&raw); err != nil {
		return err
	}

	switch value := raw.(type) {
	case nil:
		*v = ""
	case string:
		*v = ScalarString(value)
	case int:
		*v = ScalarString(fmt.Sprintf("%d", value))
	case int64:
		*v = ScalarString(fmt.Sprintf("%d", value))
	case float64:
		*v = ScalarString(fmt.Sprintf("%v", value))
	case bool:
		if value {
			*v = "true"
		} else {
			*v = "false"
		}
	default:
		return fmt.Errorf("unsupported scalar value %T", raw)
	}

	return nil
}

func (v ScalarString) String() string {
	return string(v)
}

type CRUDSpec struct {
	Create bool `yaml:"create"`
	List   bool `yaml:"list"`
	Detail bool `yaml:"detail"`
	Update bool `yaml:"update"`
	Delete bool `yaml:"delete"`
}

type TestSpec struct {
	Enabled           bool           `yaml:"enabled"`
	CreateBody        map[string]any `yaml:"createBody"`
	UpdateBody        map[string]any `yaml:"updateBody"`
	CheckUpdatedField string         `yaml:"checkUpdatedField"`
	CheckUpdatedValue string         `yaml:"checkUpdatedValue"`
}

func (s ObjectSpec) Validate() error {
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if !token.IsIdentifier(PascalCase(s.Name)) {
		return fmt.Errorf("name %q does not produce a valid Go identifier", s.Name)
	}
	if strings.TrimSpace(s.Table.Name) == "" {
		return fmt.Errorf("table.name is required")
	}
	if err := validateRelation(s.Table, "table"); err != nil {
		return err
	}
	if strings.TrimSpace(s.Route) == "" {
		return fmt.Errorf("route is required")
	}
	if err := validateRoutePath(s.Route, false); err != nil {
		return fmt.Errorf("route: %w", err)
	}
	if prefix := strings.TrimSpace(s.PermissionPrefix); prefix != "" && !token.IsIdentifier(prefix) {
		return fmt.Errorf("permissionPrefix %q must be a valid identifier", prefix)
	}
	if serviceName := strings.TrimSpace(s.ServiceName); strings.ContainsAny(serviceName, "\r\n\t") {
		return fmt.Errorf("serviceName must not contain control whitespace")
	}
	if len(s.Keys) == 0 {
		return fmt.Errorf("at least one key is required")
	}
	if len(s.Fields) == 0 {
		return fmt.Errorf("at least one field is required")
	}
	if !(s.CRUD.Create || s.CRUD.List || s.CRUD.Detail || s.CRUD.Update || s.CRUD.Delete) {
		return fmt.Errorf("at least one CRUD operation is required")
	}

	fieldNames := make(map[string]FieldSpec, len(s.Fields))
	fieldGoNames := make(map[string]string, len(s.Fields))
	fieldJSONNames := make(map[string]string, len(s.Fields))
	primaryFields := make(map[string]struct{})
	for _, field := range s.Fields {
		if err := validateField(field); err != nil {
			return err
		}
		if _, exists := fieldNames[field.Name]; exists {
			return fmt.Errorf("duplicate field %s", field.Name)
		}
		goName := PascalCase(field.Name)
		if previous, exists := fieldGoNames[goName]; exists {
			return fmt.Errorf("fields %s and %s generate the same Go field %s", previous, field.Name, goName)
		}
		fieldGoNames[goName] = field.Name
		if field.JSON == nil || *field.JSON {
			jsonName := strings.TrimSpace(field.JSONName)
			if jsonName == "" {
				jsonName = field.Name
			}
			if previous, exists := fieldJSONNames[jsonName]; exists {
				return fmt.Errorf("fields %s and %s use the same JSON name %s", previous, field.Name, jsonName)
			}
			fieldJSONNames[jsonName] = field.Name
		}
		fieldNames[field.Name] = field
		if field.PrimaryKey {
			primaryFields[field.Name] = struct{}{}
		}
	}
	for _, field := range s.Fields {
		jsonHidden := field.JSON != nil && !*field.JSON
		writable := !(field.ServerGenerated || field.SrvCalc || field.ReadOnly || field.AutoIncrement)
		if jsonHidden && writable && (s.CRUD.Create || (s.CRUD.Update && !field.PrimaryKey)) {
			return fmt.Errorf("field %s is writable but json is false; generic CRUD binders cannot populate it", field.Name)
		}
	}
	if len(primaryFields) == 0 {
		return fmt.Errorf("at least one field must have primaryKey: true")
	}
	if len(s.Keys) != len(primaryFields) {
		return fmt.Errorf("keys must contain every primary-key field: got %d keys for %d primary fields", len(s.Keys), len(primaryFields))
	}

	pathNames := make(map[string]struct{}, len(s.Keys))
	keyNames := make(map[string]struct{}, len(s.Keys))
	for _, key := range s.Keys {
		if strings.TrimSpace(key.Name) == "" {
			return fmt.Errorf("key name is required")
		}
		if _, exists := keyNames[key.Name]; exists {
			return fmt.Errorf("duplicate key %s", key.Name)
		}
		keyNames[key.Name] = struct{}{}
		field, ok := fieldNames[key.Name]
		if !ok {
			return fmt.Errorf("key %s does not match any field", key.Name)
		}
		if !field.PrimaryKey {
			return fmt.Errorf("key %s field must have primaryKey: true", key.Name)
		}
		pathName := strings.TrimSpace(key.PathName)
		if pathName == "" {
			pathName = key.Name
		}
		if !isPathIdentifier(pathName) {
			return fmt.Errorf("key %s: invalid pathName %q", key.Name, pathName)
		}
		if _, exists := pathNames[pathName]; exists {
			return fmt.Errorf("duplicate key pathName %s", pathName)
		}
		pathNames[pathName] = struct{}{}
	}
	for primaryName := range primaryFields {
		if _, exists := keyNames[primaryName]; !exists {
			return fmt.Errorf("primary-key field %s is missing from keys", primaryName)
		}
	}

	if s.List != nil {
		if strings.TrimSpace(s.List.Model) == "" {
			return fmt.Errorf("list.model is required")
		}
		if !token.IsIdentifier(PascalCase(s.List.Model)) {
			return fmt.Errorf("list.model %q does not produce a valid Go identifier", s.List.Model)
		}
		if PascalCase(s.List.Model) == PascalCase(s.Name) {
			return fmt.Errorf("list.model must differ from the base model name")
		}
		if strings.TrimSpace(s.List.Table.Name) == "" {
			return fmt.Errorf("list.table.name is required")
		}
		if err := validateRelation(s.List.Table, "list.table"); err != nil {
			return err
		}
		if len(s.List.Fields) == 0 {
			return fmt.Errorf("list.fields must not be empty")
		}
		seen := make(map[string]struct{}, len(s.List.Fields))
		seenGo := make(map[string]string, len(s.List.Fields))
		seenJSON := make(map[string]string, len(s.List.Fields))
		for _, field := range s.List.Fields {
			if err := validateField(field); err != nil {
				return fmt.Errorf("list: %w", err)
			}
			if _, exists := seen[field.Name]; exists {
				return fmt.Errorf("list: duplicate field %s", field.Name)
			}
			goName := PascalCase(field.Name)
			if previous, exists := seenGo[goName]; exists {
				return fmt.Errorf("list fields %s and %s generate the same Go field %s", previous, field.Name, goName)
			}
			if field.JSON == nil || *field.JSON {
				jsonName := strings.TrimSpace(field.JSONName)
				if jsonName == "" {
					jsonName = field.Name
				}
				if previous, exists := seenJSON[jsonName]; exists {
					return fmt.Errorf("list fields %s and %s use the same JSON name %s", previous, field.Name, jsonName)
				}
				seenJSON[jsonName] = field.Name
			}
			seen[field.Name] = struct{}{}
			seenGo[goName] = field.Name
		}
	}

	for _, index := range s.Migration.Indexes {
		if strings.TrimSpace(index.Name) == "" {
			return fmt.Errorf("migration index name is required")
		}
		if !isSQLIdentifier(index.Name) {
			return fmt.Errorf("migration index name %q is not a safe SQL identifier", index.Name)
		}
		if len(index.Columns) == 0 {
			return fmt.Errorf("migration index %s must contain columns", index.Name)
		}
		if err := validateIndexMethod(index.Method); err != nil {
			return fmt.Errorf("migration index %s: %w", index.Name, err)
		}
	}
	if name := strings.TrimSpace(s.Migration.Name); name != "" && !isMigrationName(name) {
		return fmt.Errorf("migration.name %q may contain only letters, digits, and underscores", name)
	}
	if column := strings.TrimSpace(s.Migration.UpdatedAtTrigger); column != "" {
		if !isSQLIdentifier(column) {
			return fmt.Errorf("migration.updatedAtTrigger %q is not a safe SQL identifier", column)
		}
		if _, exists := fieldNames[column]; !exists {
			return fmt.Errorf("migration.updatedAtTrigger field %s does not exist", column)
		}
	}

	if s.ApplicationRoute.Enabled {
		if strings.TrimSpace(s.ApplicationRoute.Name) == "" {
			return fmt.Errorf("applicationRoute.name is required when enabled")
		}
		if strings.TrimSpace(s.ApplicationRoute.Path) == "" {
			return fmt.Errorf("applicationRoute.path is required when enabled")
		}
		if !token.IsIdentifier(strings.TrimSpace(s.ApplicationRoute.Name)) {
			return fmt.Errorf("applicationRoute.name %q must be a valid identifier", s.ApplicationRoute.Name)
		}
		if err := validateRoutePath(strings.TrimSpace(s.ApplicationRoute.Path), true); err != nil {
			return fmt.Errorf("applicationRoute.path: %w", err)
		}
		if strings.TrimSpace(s.ApplicationRoute.Description) == "" {
			return fmt.Errorf("applicationRoute.description is required when enabled")
		}
		if strings.TrimSpace(s.ApplicationRoute.Section) == "" {
			return fmt.Errorf("applicationRoute.section is required when enabled")
		}
	}
	allowedPermissionActions := map[string]struct{}{
		"create": {}, "list": {}, "detail": {}, "update": {}, "delete": {},
	}
	for action := range s.Permissions.Descriptions {
		if _, exists := allowedPermissionActions[action]; !exists {
			return fmt.Errorf("permissions.descriptions contains unsupported action %q", action)
		}
	}
	for _, role := range s.Permissions.GrantRoles {
		if strings.TrimSpace(role) == "" {
			return fmt.Errorf("permissions.grantRoles must not contain empty values")
		}
	}

	if s.Test.Enabled && !(s.CRUD.Create && s.CRUD.List && s.CRUD.Detail && s.CRUD.Update && s.CRUD.Delete) {
		return fmt.Errorf("test.enabled requires full CRUD")
	}

	if s.Menu.Enabled {
		if !s.ApplicationRoute.Enabled {
			return fmt.Errorf("menu requires applicationRoute.enabled: true")
		}
		if strings.TrimSpace(s.Menu.Caption) == "" {
			return fmt.Errorf("menu.caption is required when menu is enabled")
		}
		if s.Menu.Parent != nil && strings.TrimSpace(s.Menu.Parent.Caption) == "" {
			return fmt.Errorf("menu.parent.caption is required")
		}
	}
	if err := validateFrontendSpec(s, fieldNames); err != nil {
		return err
	}

	return nil
}

func validateFrontendSpec(spec ObjectSpec, fields map[string]FieldSpec) error {
	frontend := spec.Frontend
	if frontend.Form.Columns < 0 || frontend.Form.Columns > 4 {
		return fmt.Errorf("frontend.form.columns must be between 1 and 4")
	}
	if frontend.List.PageSize < 0 {
		return fmt.Errorf("frontend.list.pageSize must not be negative")
	}
	switch strings.TrimSpace(frontend.List.EditMode) {
	case "", "page", "inline":
	default:
		return fmt.Errorf("frontend.list.editMode must be page or inline")
	}
	for _, imports := range [][]FrontendImportSpec{
		frontend.TypeImports,
		frontend.SchemaImports,
		frontend.ListTypeImports,
		frontend.ListSchemaImports,
	} {
		for _, item := range imports {
			if strings.TrimSpace(item.From) == "" {
				return fmt.Errorf("frontend import from is required")
			}
			if len(item.Names) == 0 {
				return fmt.Errorf("frontend import %s must contain names", item.From)
			}
			for _, name := range item.Names {
				if !token.IsIdentifier(strings.TrimSpace(name)) {
					return fmt.Errorf("frontend import name %q must be a valid identifier", name)
				}
			}
		}
	}

	frontendNames := make(map[string]FieldSpec, len(fields)*2)
	for _, field := range fields {
		if field.JSON != nil && !*field.JSON {
			continue
		}
		jsonName := strings.TrimSpace(field.JSONName)
		if jsonName == "" {
			jsonName = field.Name
		}
		frontendNames[field.Name] = field
		frontendNames[jsonName] = field
	}
	seenFormFields := make(map[string]struct{}, len(frontend.Form.Fields))
	for _, item := range frontend.Form.Fields {
		name := strings.TrimSpace(item.Field)
		if name == "" {
			return fmt.Errorf("frontend.form.fields[].field is required")
		}
		if _, exists := frontendNames[name]; !exists {
			return fmt.Errorf("frontend form field %s does not match a JSON-visible model field", name)
		}
		if _, exists := seenFormFields[name]; exists {
			return fmt.Errorf("duplicate frontend form field %s", name)
		}
		seenFormFields[name] = struct{}{}
		if item.ColumnSpan < 0 || item.ColumnSpan > 4 {
			return fmt.Errorf("frontend form field %s columnSpan must be between 1 and 4", name)
		}
		if strings.TrimSpace(item.ComponentImport) != "" && strings.TrimSpace(item.Component) == "" {
			return fmt.Errorf("frontend form field %s componentImport requires component", name)
		}
		if isCustomFrontendComponent(item.Component) && strings.TrimSpace(item.ComponentImport) == "" {
			return fmt.Errorf("frontend form field %s custom component %s requires componentImport", name, item.Component)
		}
	}

	listFields := fields
	if spec.List != nil {
		listFields = make(map[string]FieldSpec, len(spec.List.Fields))
		for _, field := range spec.List.Fields {
			listFields[field.Name] = field
		}
	}
	frontendListNames := make(map[string]struct{}, len(listFields)*2)
	for _, field := range listFields {
		if field.JSON != nil && !*field.JSON {
			continue
		}
		jsonName := strings.TrimSpace(field.JSONName)
		if jsonName == "" {
			jsonName = field.Name
		}
		frontendListNames[field.Name] = struct{}{}
		frontendListNames[jsonName] = struct{}{}
	}
	seenColumns := make(map[string]struct{}, len(frontend.List.Columns))
	for _, item := range frontend.List.Columns {
		name := strings.TrimSpace(item.Field)
		if name == "" {
			return fmt.Errorf("frontend.list.columns[].field is required")
		}
		if _, exists := frontendListNames[name]; !exists {
			return fmt.Errorf("frontend list column %s does not match a JSON-visible list field", name)
		}
		if _, exists := seenColumns[name]; exists {
			return fmt.Errorf("duplicate frontend list column %s", name)
		}
		seenColumns[name] = struct{}{}
		switch strings.TrimSpace(item.DataType) {
		case "", "string", "number", "boolean", "date":
		default:
			return fmt.Errorf("frontend list column %s has unsupported dataType %q", name, item.DataType)
		}
		switch strings.TrimSpace(item.Align) {
		case "", "left", "center", "right":
		default:
			return fmt.Errorf("frontend list column %s has unsupported align %q", name, item.Align)
		}
	}
	for label, name := range map[string]string{
		"listName":   frontend.Routes.ListName,
		"createName": frontend.Routes.CreateName,
		"editName":   frontend.Routes.EditName,
	} {
		if strings.TrimSpace(name) != "" && !token.IsIdentifier(strings.TrimSpace(name)) {
			return fmt.Errorf("frontend.routes.%s %q must be a valid identifier", label, name)
		}
	}
	if frontend.Scaffold && !spec.ApplicationRoute.Enabled {
		return fmt.Errorf("frontend.scaffold requires applicationRoute.enabled: true so generated pages participate in route synchronization")
	}
	return nil
}

func validateField(field FieldSpec) error {
	if strings.TrimSpace(field.Name) == "" {
		return fmt.Errorf("field name is required")
	}
	if !isSQLIdentifier(field.Name) {
		return fmt.Errorf("field %q is not a safe SQL identifier", field.Name)
	}
	if !token.IsIdentifier(PascalCase(field.Name)) {
		return fmt.Errorf("field %q does not produce a valid Go identifier", field.Name)
	}
	if strings.TrimSpace(field.Type) == "" && strings.TrimSpace(field.GoType) == "" {
		return fmt.Errorf("field %s: type or goType is required", field.Name)
	}
	if field.JSON != nil && !*field.JSON && strings.TrimSpace(field.JSONName) != "" {
		return fmt.Errorf("field %s: jsonName cannot be set when json is false", field.Name)
	}
	if field.JSON != nil && !*field.JSON && field.JSONOmitEmpty {
		return fmt.Errorf("field %s: jsonOmitEmpty cannot be set when json is false", field.Name)
	}
	if jsonName := strings.TrimSpace(field.JSONName); jsonName == "-" || strings.ContainsAny(jsonName, ",\r\n\t") {
		return fmt.Errorf("field %s: invalid jsonName %q", field.Name, field.JSONName)
	}
	if field.PrimaryKey && field.JSON != nil && !*field.JSON {
		return fmt.Errorf("field %s: primary key must be JSON-visible", field.Name)
	}
	if strings.TrimSpace(field.DateType) != "" && !isAllowedDateType(field.DateType) {
		return fmt.Errorf("field %s: unsupported dateType %q", field.Name, field.DateType)
	}
	if field.Nullable && field.Required {
		return fmt.Errorf("field %s: nullable and required cannot both be true", field.Name)
	}
	if field.Nullable && field.PrimaryKey {
		return fmt.Errorf("field %s: primary key cannot be nullable", field.Name)
	}
	if field.Length < 0 {
		return fmt.Errorf("field %s: length must not be negative", field.Name)
	}
	for label, value := range map[string]string{
		"maxLen": field.MaxLen.String(),
		"max":    field.Max.String(),
		"fix":    field.Fix.String(),
	} {
		if strings.TrimSpace(value) == "" {
			continue
		}
		number, err := strconv.Atoi(value)
		if err != nil || number <= 0 {
			return fmt.Errorf("field %s: %s must be a positive integer", field.Name, label)
		}
	}
	if field.AutoIncrement && !field.PrimaryKey {
		return fmt.Errorf("field %s: autoIncrement requires primaryKey", field.Name)
	}
	if field.References != nil {
		if strings.TrimSpace(field.References.Table) == "" {
			return fmt.Errorf("field %s: references.table is required", field.Name)
		}
		if !isSQLIdentifier(field.References.Table) {
			return fmt.Errorf("field %s: references.table %q is not a safe SQL identifier", field.Name, field.References.Table)
		}
		if schema := strings.TrimSpace(field.References.Schema); schema != "" && !isSQLIdentifier(schema) {
			return fmt.Errorf("field %s: references.schema %q is not a safe SQL identifier", field.Name, schema)
		}
		if column := strings.TrimSpace(field.References.Column); column != "" && !isSQLIdentifier(column) {
			return fmt.Errorf("field %s: references.column %q is not a safe SQL identifier", field.Name, column)
		}
		if strings.TrimSpace(field.References.Column) == "" {
			field.References.Column = "id"
		}
		if err := validateReferenceAction(field.References.OnDelete); err != nil {
			return fmt.Errorf("field %s: onDelete: %w", field.Name, err)
		}
		if err := validateReferenceAction(field.References.OnUpdate); err != nil {
			return fmt.Errorf("field %s: onUpdate: %w", field.Name, err)
		}
	}

	return nil
}

func validateRelation(table TableSpec, label string) error {
	if schema := strings.TrimSpace(table.Schema); schema != "" && !isSQLIdentifier(schema) {
		return fmt.Errorf("%s.schema %q is not a safe SQL identifier", label, schema)
	}
	if !isSQLIdentifier(strings.TrimSpace(table.Name)) {
		return fmt.Errorf("%s.name %q is not a safe SQL identifier", label, table.Name)
	}
	return nil
}

func isSQLIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		if index == 0 {
			if r != '_' && (r < 'a' || r > 'z') {
				return false
			}
			continue
		}
		if r != '_' && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func isPathIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		if index == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && r != '-' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func validateRoutePath(value string, allowParameters bool) error {
	if !strings.HasPrefix(value, "/") {
		return fmt.Errorf("must start with /")
	}
	if len(value) > 1 && strings.HasSuffix(value, "/") {
		return fmt.Errorf("must not end with /")
	}
	for _, r := range value {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return fmt.Errorf("must not contain whitespace or control characters")
		}
	}
	if !allowParameters && strings.ContainsAny(value, "{}:") {
		return fmt.Errorf("must be a collection route without path placeholders")
	}

	return nil
}

func isMigrationName(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func validateIndexMethod(value string) error {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "btree", "hash", "gist", "spgist", "gin", "brin":
		return nil
	default:
		return fmt.Errorf("unsupported index method %q", value)
	}
}

func validateReferenceAction(value string) error {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return nil
	}
	switch value {
	case "NO ACTION", "RESTRICT", "CASCADE", "SET NULL", "SET DEFAULT":
		return nil
	default:
		return fmt.Errorf("unsupported action %q", value)
	}
}

func isAllowedDateType(value string) bool {
	switch value {
	case "date", "time", "datetime", "datetime_tz":
		return true
	default:
		return false
	}
}

func boolOrDefault(value *bool, defaultValue bool) bool {
	if value == nil {
		return defaultValue
	}
	return *value
}
