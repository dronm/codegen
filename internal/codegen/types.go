package codegen

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type ObjectView struct {
	SourceFile           string
	Name                 string
	Camel                string
	Snake                string
	SnakePlural          string
	Kebab                string
	Human                string
	HumanPlural          string
	Comment              string
	FileBase             string
	TableSchema          string
	TableName            string
	TableComment         string
	Relation             string
	ListModelName        string
	ListCamel            string
	ListFileBase         string
	ListRelation         string
	ListComment          string
	HasCustomList        bool
	Route                string
	ItemRoute            string
	PermissionPrefix     string
	ServiceName          string
	SessionRequired      bool
	CRUDNotifications    bool
	PermissionsEnabled   bool
	GoModule             string
	NeedsTimeImport      bool
	NeedsModelbindImport bool
	CompositeKey         bool
	Keys                 []KeyView
	Fields               []FieldView
	ListFields           []FieldView
	CreateFields         []FieldView
	UpdateFields         []FieldView
	FrontendFields       []FieldView
	FrontendListFields   []FieldView
	FrontendCreateFields []FieldView
	FrontendUpdateFields []FieldView
	FrontendKeys         []KeyView
	Frontend             FrontendView
	CommonSchemas        []string
	ListCommonSchemas    []string
	CRUD                 CRUDSpec
	GeneratedServiceCRUD CRUDSpec
	ManualServiceCRUD    CRUDSpec
	PermissionRows       []PermissionView
	GrantRoles           []string
	ApplicationRoute     ApplicationRouteView
	Menu                 MenuView
	Migration            MigrationView
	Test                 TestView
	ServiceImports       ServiceImportView
	HTTPImports          HTTPImportView
}

type KeyView struct {
	Name          string
	Pascal        string
	Camel         string
	PathName      string
	JSONName      string
	TSName        string
	Type          string
	GoType        string
	TSType        string
	StructTag     string
	IsString      bool
	IsNumeric     bool
	AutoIncrement bool
	FormatVerb    string
}

type FieldView struct {
	Name            string
	Type            string
	JSONName        string
	JSONEnabled     bool
	TSName          string
	TSOptional      bool
	Pascal          string
	GoType          string
	TSType          string
	TSDTOType       string
	ValibotDTO      string
	ValibotModel    string
	SQLType         string
	SQLLine         string
	StructTag       string
	Comment         string
	PrimaryKey      bool
	Required        bool
	DBRequired      bool
	Nullable        bool
	AutoIncrement   bool
	ServerGenerated bool
	SrvCalc         bool
	ReadOnly        bool
	DateType        string
	Enum            string
	MaxLen          string
	Default         string
	ModelTransform  string
	CreateField     bool
	UpdateField     bool
}

type FrontendView struct {
	Scaffold          bool
	Title             string
	TypeImports       []FrontendImportView
	SchemaImports     []FrontendImportView
	ListTypeImports   []FrontendImportView
	ListSchemaImports []FrontendImportView
	Form              FrontendFormView
	List              FrontendListView
	Routes            FrontendRoutesView
	LocaleFields      []FrontendLocaleFieldView
}

type FrontendLocaleFieldView struct {
	Name  string
	Label string
}

type FrontendImportView struct {
	From     string
	Names    []string
	TypeOnly bool
}

type FrontendFormView struct {
	Enabled           bool
	Columns           int
	CreateTitle       string
	EditTitle         string
	CopyTitle         string
	NeedsNullableText bool
	NeedsFormatDate   bool
	Fields            []FrontendFormFieldView
	Imports           []FrontendComponentImportView
}

type FrontendFormFieldView struct {
	Field             FieldView
	Component         string
	Label             string
	DefaultLiteral    string
	ReadOnly          bool
	Hidden            bool
	Autofocus         bool
	ColumnSpan        int
	ID                string
	SubmitExpression  string
	DisableExpression string
	CustomComponent   bool
}

type FrontendComponentImportView struct {
	Name string
	From string
}

type FrontendListView struct {
	Enabled           bool
	PageSize          int
	EditMode          string
	Inline            bool
	CanInlineCreate   bool
	NeedsFormatDate   bool
	Columns           []FrontendListColumnView
	CreateRowFields   []FrontendInlineCreateFieldView
	CreateModelFields []FrontendInlineCreateModelFieldView
}

type FrontendListColumnView struct {
	Field    FieldView
	Label    string
	Width    string
	Sortable bool
	Editable bool
	DataType string
	Align    string
	Format   string
}

type FrontendInlineCreateFieldView struct {
	Field          FieldView
	DefaultLiteral string
}

type FrontendInlineCreateModelFieldView struct {
	Field      FieldView
	Expression string
}

type FrontendRoutesView struct {
	ListName   string
	CreateName string
	EditName   string
	ListPath   string
	CreatePath string
	EditPath   string
}

type PermissionView struct {
	Action      string
	Code        string
	Description string
}

type ApplicationRouteView struct {
	Enabled       bool
	Name          string
	Path          string
	Description   string
	Section       string
	Icon          *string
	MenuAvailable bool
}

type MenuView struct {
	Enabled         bool
	Role            string
	Caption         string
	Icon            *string
	SortOrder       int
	IsActive        bool
	HasParent       bool
	ParentCaption   string
	ParentIcon      *string
	ParentSortOrder int
	CreateParent    bool
}

type IndexView struct {
	Name       string
	ColumnsSQL string
	Unique     bool
	Where      string
	Method     string
}

type MigrationView struct {
	Enabled             bool
	Name                string
	TableConstraints    []string
	Indexes             []IndexView
	UpdatedAtTrigger    string
	UpdatedAtFunction   string
	HasUpdatedAtTrigger bool
}

type ServiceImportView struct {
	Context   bool
	Errors    bool
	Fmt       bool
	Modelbind bool
	Models    bool
	Types     bool
	Session   bool
	AppErrors bool
	Webapp    bool
	WebModels bool
}

type HTTPImportView struct {
	Fmt       bool
	HTTP      bool
	Strconv   bool
	Modelbind bool
	Models    bool
	Webapp    bool
}

type TestView struct {
	Enabled           bool
	CreateBodyLines   []string
	UpdateBodyLines   []string
	CheckUpdatedField string
	CheckUpdatedValue string
	NeedsFmtImport    bool
	NeedsURLImport    bool
}

type RoutesView struct {
	Objects []ObjectView
}

func BuildObjectView(cfg Config, spec ObjectSpec) (ObjectView, error) {
	if err := spec.Validate(); err != nil {
		return ObjectView{}, err
	}

	name := PascalCase(spec.Name)
	tableSchema := schemaOrPublic(spec.Table.Schema)

	serviceName := strings.TrimSpace(spec.ServiceName)
	if serviceName == "" {
		serviceName = name
	}

	permissionPrefix := strings.TrimSpace(spec.PermissionPrefix)
	if permissionPrefix == "" {
		permissionPrefix = CamelCase(name)
	}

	human := strings.TrimSpace(spec.HumanName)
	if human == "" {
		human = HumanName(name)
	}
	humanPlural := strings.TrimSpace(spec.HumanNamePlural)
	if humanPlural == "" {
		humanPlural = pluralHumanName(human)
	}

	comment := oneLineText(spec.Comment)
	if comment == "" {
		comment = name + " is an editable database-backed application model."
	}

	view := ObjectView{
		SourceFile:         spec.SourceFile,
		Name:               name,
		Camel:              CamelCase(name),
		Snake:              SnakeCase(name),
		SnakePlural:        PluralSnake(name),
		Kebab:              KebabCase(name),
		Human:              human,
		HumanPlural:        humanPlural,
		Comment:            comment,
		FileBase:           CamelCase(name),
		TableSchema:        tableSchema,
		TableName:          spec.Table.Name,
		TableComment:       strings.TrimSpace(spec.Table.Comment),
		Relation:           relationName(tableSchema, spec.Table.Name),
		Route:              spec.Route,
		PermissionPrefix:   permissionPrefix,
		ServiceName:        serviceName,
		SessionRequired:    boolOrDefault(spec.SessionRequired, true),
		CRUDNotifications:  boolOrDefault(spec.CRUDNotifications, true),
		PermissionsEnabled: boolOrDefault(spec.Permissions.Enabled, true),
		CRUD:               spec.CRUD,
		ManualServiceCRUD:  manualServiceCRUD(spec.Service.ManualMethods),
		GoModule:           cfg.GoModule,
		CompositeKey:       len(spec.Keys) > 1,
	}
	view.GeneratedServiceCRUD = generatedServiceCRUD(view.CRUD, view.ManualServiceCRUD)

	fieldSpecs := make(map[string]FieldSpec, len(spec.Fields))
	primaryKeyCount := 0
	for _, field := range spec.Fields {
		fieldSpecs[field.Name] = field
		if field.PrimaryKey {
			primaryKeyCount++
		}
	}

	for _, key := range spec.Keys {
		field := fieldSpecs[key.Name]
		keyView, err := BuildKeyView(cfg, key, field)
		if err != nil {
			return ObjectView{}, err
		}
		view.Keys = append(view.Keys, keyView)
		view.FrontendKeys = append(view.FrontendKeys, keyView)
	}
	view.ItemRoute = itemRoute(spec.Route, view.Keys)

	for _, field := range spec.Fields {
		fieldView, err := BuildFieldView(cfg, field, primaryKeyCount == 1)
		if err != nil {
			return ObjectView{}, err
		}
		view.Fields = append(view.Fields, fieldView)
		if fieldView.JSONEnabled {
			view.FrontendFields = append(view.FrontendFields, fieldView)
		}
		if fieldView.CreateField {
			view.CreateFields = append(view.CreateFields, fieldView)
			if fieldView.JSONEnabled {
				view.FrontendCreateFields = append(view.FrontendCreateFields, fieldView)
			}
		}
		if fieldView.UpdateField {
			view.UpdateFields = append(view.UpdateFields, fieldView)
			if fieldView.JSONEnabled {
				view.FrontendUpdateFields = append(view.FrontendUpdateFields, fieldView)
			}
		}
		if strings.Contains(fieldView.GoType, "time.Time") {
			view.NeedsTimeImport = true
		}
	}
	if spec.CRUD.Update && len(view.UpdateFields) == 0 {
		return ObjectView{}, fmt.Errorf("update is enabled but %s has no writable non-key fields", view.Name)
	}

	view.ListModelName = view.Name
	view.ListCamel = view.Camel
	view.ListFileBase = view.FileBase
	view.ListRelation = view.Relation
	view.ListComment = view.Comment
	view.ListFields = view.Fields
	view.FrontendListFields = view.FrontendFields
	if spec.List != nil {
		view.HasCustomList = true
		view.ListModelName = PascalCase(spec.List.Model)
		view.ListCamel = CamelCase(view.ListModelName)
		view.ListFileBase = CamelCase(view.ListModelName)
		view.ListRelation = relationName(schemaOrPublic(spec.List.Table.Schema), spec.List.Table.Name)
		view.ListComment = oneLineText(spec.List.Comment)
		if view.ListComment == "" {
			view.ListComment = view.ListModelName + " is the collection projection for " + view.HumanPlural + "."
		}
		view.ListFields = nil
		view.FrontendListFields = nil
		for _, field := range spec.List.Fields {
			fieldView, err := BuildFieldView(cfg, field, false)
			if err != nil {
				return ObjectView{}, fmt.Errorf("list field: %w", err)
			}
			view.ListFields = append(view.ListFields, fieldView)
			if fieldView.JSONEnabled {
				view.FrontendListFields = append(view.FrontendListFields, fieldView)
			}
			if strings.Contains(fieldView.GoType, "time.Time") {
				view.NeedsTimeImport = true
			}
		}
	}

	if cfg.FrontendEnabled {
		if err := validateFrontendIdentifiers(view); err != nil {
			return ObjectView{}, err
		}
	}

	view.NeedsModelbindImport = spec.CRUD.Update
	view.PermissionRows = buildPermissionRows(view, spec.Permissions)
	view.GrantRoles = append([]string(nil), spec.Permissions.GrantRoles...)
	if len(view.GrantRoles) == 0 && view.PermissionsEnabled && len(view.PermissionRows) > 0 {
		view.GrantRoles = []string{"admin"}
	}
	view.ApplicationRoute = buildApplicationRouteView(spec.ApplicationRoute, spec.Menu)
	view.Menu = buildMenuView(spec.Menu)
	view.Migration = buildMigrationView(view, spec)
	view.Test = BuildTestView(spec.Test, view.Keys)
	view.ServiceImports = buildServiceImports(view)
	view.HTTPImports = buildHTTPImports(view)
	view.Frontend = buildFrontendView(spec, view)
	view.CommonSchemas = commonSchemasForFields(view.FrontendFields)
	view.ListCommonSchemas = commonSchemasForFields(view.FrontendListFields)
	if err := validateFrontendView(view); err != nil {
		return ObjectView{}, err
	}

	return view, nil
}

func BuildKeyView(cfg Config, key KeySpec, field FieldSpec) (KeyView, error) {
	keyField := field
	if strings.TrimSpace(key.Type) != "" {
		keyField.Type = key.Type
	}
	if strings.TrimSpace(key.GoType) != "" {
		keyField.GoType = key.GoType
	}
	keyField.Nullable = false

	goType, err := goTypeFor(keyField)
	if err != nil {
		return KeyView{}, fmt.Errorf("key %s: %w", key.Name, err)
	}
	fieldType := field
	fieldType.Nullable = false
	fieldGoType, err := goTypeFor(fieldType)
	if err != nil {
		return KeyView{}, fmt.Errorf("key field %s: %w", key.Name, err)
	}
	if goType != fieldGoType {
		return KeyView{}, fmt.Errorf("key %s Go type %s does not match field Go type %s", key.Name, goType, fieldGoType)
	}

	pathName := strings.TrimSpace(key.PathName)
	if pathName == "" {
		pathName = key.Name
	}

	jsonName := field.JSONName
	if jsonName == "" {
		jsonName = field.Name
	}

	view := KeyView{
		Name:          key.Name,
		Pascal:        PascalCase(key.Name),
		Camel:         CamelCase(key.Name),
		PathName:      pathName,
		JSONName:      jsonName,
		TSName:        jsonName,
		Type:          keyField.Type,
		GoType:        goType,
		TSType:        tsTypeFor(keyField),
		AutoIncrement: field.AutoIncrement,
		StructTag: structTagLiteral([]tagPart{
			{Name: cfg.GoJSONTagName, Value: jsonName},
			{Name: "primaryKey", Value: "true"},
			{Name: "required", Value: "true"},
		}),
	}

	switch keyField.Type {
	case "string", "text", "enum", "password":
		view.IsString = true
		view.FormatVerb = "%s"
	case "int", "bigint":
		view.IsNumeric = true
		view.FormatVerb = "%d"
	default:
		if goType == "string" {
			view.IsString = true
			view.FormatVerb = "%s"
		} else if goType == "int" || goType == "int64" {
			view.IsNumeric = true
			view.FormatVerb = "%d"
		} else {
			return KeyView{}, fmt.Errorf("key %s: unsupported path key Go type %q; use string, int, or int64", key.Name, goType)
		}
	}

	return view, nil
}

func BuildFieldView(cfg Config, field FieldSpec, inlinePrimaryKey bool) (FieldView, error) {
	goType, err := goTypeFor(field)
	if err != nil {
		return FieldView{}, fmt.Errorf("field %s: %w", field.Name, err)
	}

	tsType := field.TSType
	if tsType == "" {
		tsType = tsTypeFor(field)
	}

	tsDTOType := field.TSDTOType
	if tsDTOType == "" {
		tsDTOType = tsDTOTypeFor(field)
	}

	sqlType := field.SQLType
	if sqlType == "" {
		sqlType = sqlTypeFor(field)
	}

	dateType := field.DateType
	if dateType == "" {
		dateType = dateTypeFor(field.Type)
	}

	srvCalc := field.SrvCalc || field.ServerGenerated || field.AutoIncrement || field.ReadOnly
	maxLen := field.MaxLen.String()
	if maxLen == "" && isStringLikeType(field.Type) {
		maxLen = field.Max.String()
	}

	jsonEnabled := field.JSON == nil || *field.JSON
	jsonName := field.JSONName
	if jsonName == "" {
		jsonName = field.Name
	}

	view := FieldView{
		Name:            field.Name,
		Type:            field.Type,
		JSONName:        jsonName,
		JSONEnabled:     jsonEnabled,
		TSName:          jsonName,
		TSOptional:      field.JSONOmitEmpty,
		Pascal:          PascalCase(field.Name),
		GoType:          goType,
		TSType:          tsType,
		TSDTOType:       tsDTOType,
		ValibotDTO:      valibotFor(field, true),
		ValibotModel:    valibotFor(field, false),
		SQLType:         sqlType,
		StructTag:       structTagForField(cfg, field, srvCalc, maxLen),
		Comment:         oneLineText(field.Comment),
		PrimaryKey:      field.PrimaryKey,
		Required:        field.Required,
		DBRequired:      field.DBRequired,
		Nullable:        field.Nullable,
		AutoIncrement:   field.AutoIncrement,
		ServerGenerated: field.ServerGenerated,
		SrvCalc:         srvCalc,
		ReadOnly:        field.ReadOnly,
		DateType:        dateType,
		Enum:            field.Enum,
		MaxLen:          maxLen,
		Default:         field.Default,
		CreateField:     !srvCalc,
		UpdateField:     !srvCalc && !field.PrimaryKey,
	}

	if isDateLikeType(field.Type) {
		if field.Nullable {
			view.ModelTransform = fmt.Sprintf("%s: parsedDTO.%s === null ? null : new Date(parsedDTO.%s)", jsonName, jsonName, jsonName)
		} else {
			view.ModelTransform = fmt.Sprintf("%s: new Date(parsedDTO.%s)", jsonName, jsonName)
		}
	}

	view.SQLLine = sqlLineFor(field, sqlType, inlinePrimaryKey)

	return view, nil
}

func goTypeFor(field FieldSpec) (string, error) {
	if strings.TrimSpace(field.GoType) != "" {
		goType := strings.TrimSpace(field.GoType)
		if field.Nullable && !strings.HasPrefix(goType, "*") && !strings.HasPrefix(goType, "[]") && !strings.HasPrefix(goType, "map[") && goType != "any" {
			return "*" + goType, nil
		}
		return goType, nil
	}

	base := ""
	switch field.Type {
	case "int":
		base = "int"
	case "bigint":
		base = "int64"
	case "float", "numeric":
		base = "float64"
	case "string", "text", "enum", "password":
		base = "string"
	case "bool":
		base = "bool"
	case "json", "jsonb":
		base = "map[string]any"
	case "date", "time", "datetime", "timestamptz":
		base = "time.Time"
	default:
		return "", fmt.Errorf("unsupported type %q", field.Type)
	}

	if field.Nullable {
		return "*" + base, nil
	}

	return base, nil
}

func tsTypeFor(field FieldSpec) string {
	base := "unknown"
	switch field.Type {
	case "int", "bigint", "float", "numeric":
		base = "number"
	case "string", "text", "enum", "password":
		base = "string"
	case "bool":
		base = "boolean"
	case "json", "jsonb":
		base = "Record<string, unknown>"
	case "date", "datetime", "timestamptz":
		base = "Date"
	case "time":
		base = "string"
	}

	if field.Nullable {
		return base + " | null"
	}

	return base
}

func tsDTOTypeFor(field FieldSpec) string {
	base := "unknown"
	switch field.Type {
	case "int", "bigint", "float", "numeric":
		base = "number"
	case "string", "text", "enum", "password":
		base = "string"
	case "bool":
		base = "boolean"
	case "json", "jsonb":
		base = "Record<string, unknown>"
	case "date", "datetime", "timestamptz", "time":
		base = "string"
	}

	if field.Nullable {
		return base + " | null"
	}

	return base
}

func sqlTypeFor(field FieldSpec) string {
	switch field.Type {
	case "int":
		return "integer"
	case "bigint":
		return "bigint"
	case "float":
		return "double precision"
	case "numeric":
		return "numeric"
	case "string", "password", "enum":
		if field.Fix.String() != "" {
			return "character(" + field.Fix.String() + ")"
		}
		maxLen := field.MaxLen.String()
		if maxLen == "" {
			maxLen = field.Max.String()
		}
		if maxLen != "" {
			return "varchar(" + maxLen + ")"
		}
		if field.Length > 0 {
			return fmt.Sprintf("varchar(%d)", field.Length)
		}
		return "text"
	case "text":
		return "text"
	case "bool":
		return "boolean"
	case "json":
		return "json"
	case "jsonb":
		return "jsonb"
	case "date":
		return "date"
	case "time":
		return "time"
	case "datetime":
		return "timestamp without time zone"
	case "timestamptz":
		return "timestamp with time zone"
	default:
		return "text"
	}
}

func valibotFor(field FieldSpec, dto bool) string {
	if dto && strings.TrimSpace(field.ValibotDTO) != "" {
		return strings.TrimSpace(field.ValibotDTO)
	}
	if !dto && strings.TrimSpace(field.ValibotModel) != "" {
		return strings.TrimSpace(field.ValibotModel)
	}
	if field.Valibot != "" {
		return field.Valibot
	}

	base := validatorFor(field)
	if dto && isDateLikeType(field.Type) {
		base = "DateStringSchema"
	}
	if !dto && isDateLikeType(field.Type) {
		base = "v.date()"
	}
	if field.Type == "time" {
		base = "StringSchema"
		if field.Required {
			base = "RequiredStringSchema"
		}
	}

	if field.Nullable {
		return "v.nullable(" + base + ")"
	}

	return base
}

func validatorFor(field FieldSpec) string {
	switch field.Type {
	case "int", "bigint":
		if field.PrimaryKey || field.References != nil || field.Name == "id" || strings.HasSuffix(field.Name, "_id") {
			return "IdSchema"
		}
		return "IntSchema"
	case "float", "numeric":
		return "NumberSchema"
	case "bool":
		return "v.boolean()"
	case "json", "jsonb":
		return "AttrsSchema"
	case "date", "datetime", "timestamptz":
		return "DateStringSchema"
	case "time":
		if field.Required {
			return "RequiredStringSchema"
		}
		return "StringSchema"
	case "text":
		if field.Required {
			return "RequiredTextSchema"
		}
		return "TextSchema"
	case "string", "enum", "password":
		if field.Required {
			return "RequiredStringSchema"
		}
		return "StringSchema"
	default:
		return "v.unknown()"
	}
}

func sqlLineFor(field FieldSpec, sqlType string, inlinePrimaryKey bool) string {
	parts := []string{field.Name, sqlType}
	if field.AutoIncrement {
		parts = append(parts, "GENERATED BY DEFAULT AS IDENTITY")
	}
	if field.Required || field.DBRequired || field.PrimaryKey {
		parts = append(parts, "NOT NULL")
	}
	if field.Default != "" {
		parts = append(parts, "DEFAULT", field.Default)
	}
	if field.Unique {
		parts = append(parts, "UNIQUE")
	}
	if field.References != nil {
		refSchema := schemaOrPublic(field.References.Schema)
		refColumn := strings.TrimSpace(field.References.Column)
		if refColumn == "" {
			refColumn = "id"
		}
		parts = append(parts, "REFERENCES", relationName(refSchema, field.References.Table)+"("+refColumn+")")
		if action := strings.ToUpper(strings.TrimSpace(field.References.OnDelete)); action != "" {
			parts = append(parts, "ON DELETE", action)
		}
		if action := strings.ToUpper(strings.TrimSpace(field.References.OnUpdate)); action != "" {
			parts = append(parts, "ON UPDATE", action)
		}
	}
	if strings.TrimSpace(field.Check) != "" {
		parts = append(parts, "CHECK ("+strings.TrimSpace(field.Check)+")")
	}
	if field.PrimaryKey && inlinePrimaryKey {
		parts = append(parts, "PRIMARY KEY")
	}

	return strings.Join(parts, " ")
}

type tagPart struct {
	Name  string
	Value string
}

func structTagForField(cfg Config, field FieldSpec, srvCalc bool, maxLen string) string {
	parts := make([]tagPart, 0, 8)
	jsonEnabled := field.JSON == nil || *field.JSON
	if jsonEnabled {
		jsonName := field.JSONName
		if jsonName == "" {
			jsonName = field.Name
		}
		if field.JSONOmitEmpty {
			jsonName += ",omitempty"
		}
		parts = append(parts, tagPart{Name: cfg.GoJSONTagName, Value: jsonName})
	} else {
		parts = append(parts, tagPart{Name: cfg.GoJSONTagName, Value: "-"})
	}
	if field.Filter != "" {
		parts = append(parts, tagPart{Name: cfg.GoFilterTagName, Value: field.Filter})
	}
	if field.PrimaryKey {
		parts = append(parts, tagPart{Name: "primaryKey", Value: "true"})
	}
	if field.Required {
		parts = append(parts, tagPart{Name: "required", Value: "true"})
	}
	if srvCalc {
		parts = append(parts, tagPart{Name: "srvCalc", Value: "true"})
	}
	if field.Enum != "" {
		parts = append(parts, tagPart{Name: "enum", Value: field.Enum})
	}
	if maxLen != "" {
		parts = append(parts, tagPart{Name: "maxLen", Value: maxLen})
	}

	return structTagLiteral(parts)
}

func structTagLiteral(parts []tagPart) string {
	if len(parts) == 0 {
		return ""
	}

	items := make([]string, 0, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part.Name)
		if name == "" {
			continue
		}
		items = append(items, name+":"+strconv.Quote(sanitizeStructTagValue(part.Value)))
	}
	if len(items) == 0 {
		return ""
	}

	return "`" + strings.Join(items, " ") + "`"
}

func sanitizeStructTagValue(value string) string {
	return strings.ReplaceAll(value, "`", "'")
}

func dateTypeFor(fieldType string) string {
	switch fieldType {
	case "date":
		return "date"
	case "time":
		return "time"
	case "datetime":
		return "datetime"
	case "timestamptz":
		return "datetime_tz"
	default:
		return ""
	}
}

func isTextLikeType(fieldType string) bool {
	switch fieldType {
	case "string", "text", "enum", "password", "time":
		return true
	default:
		return false
	}
}

func isDateLikeType(fieldType string) bool {
	switch fieldType {
	case "date", "datetime", "timestamptz":
		return true
	default:
		return false
	}
}

func isStringLikeType(fieldType string) bool {
	switch fieldType {
	case "string", "text", "enum", "password":
		return true
	default:
		return false
	}
}

func BuildTestView(spec TestSpec, keys []KeyView) TestView {
	return TestView{
		Enabled:           spec.Enabled,
		CreateBodyLines:   mapToGoLiteralLines(spec.CreateBody),
		UpdateBodyLines:   mapToGoLiteralLines(spec.UpdateBody),
		CheckUpdatedField: spec.CheckUpdatedField,
		CheckUpdatedValue: spec.CheckUpdatedValue,
		NeedsFmtImport:    len(keys) > 0,
		NeedsURLImport:    len(keys) > 0,
	}
}

func mapToGoLiteralLines(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("%q: %s,", key, goLiteral(values[key])))
	}

	return lines
}

func goLiteral(value any) string {
	switch v := value.(type) {
	case string:
		if strings.Contains(v, "TestID(") || strings.Contains(v, "os.Getpid()") || strings.Contains(v, "fmt.Sprintf(") {
			return v
		}
		return fmt.Sprintf("%q", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%v", v)
	case nil:
		return "nil"
	default:
		return fmt.Sprintf("%#v", v)
	}
}

func buildPermissionRows(view ObjectView, spec PermissionSpec) []PermissionView {
	if !view.PermissionsEnabled {
		return nil
	}

	descriptions := map[string]string{
		"create": "Create " + view.Human,
		"list":   "List " + view.HumanPlural,
		"detail": "View " + view.Human,
		"update": "Update " + view.Human,
		"delete": "Delete " + view.Human,
	}
	for action, description := range spec.Descriptions {
		descriptions[action] = strings.TrimSpace(description)
	}

	actions := []struct {
		name    string
		enabled bool
	}{
		{name: "create", enabled: view.CRUD.Create},
		{name: "list", enabled: view.CRUD.List},
		{name: "detail", enabled: view.CRUD.Detail},
		{name: "update", enabled: view.CRUD.Update},
		{name: "delete", enabled: view.CRUD.Delete},
	}

	rows := make([]PermissionView, 0, len(actions))
	for _, action := range actions {
		if !action.enabled {
			continue
		}
		rows = append(rows, PermissionView{
			Action:      action.name,
			Code:        view.PermissionPrefix + "." + action.name,
			Description: descriptions[action.name],
		})
	}

	return rows
}

func buildApplicationRouteView(spec ApplicationRouteSpec, menu MenuSpec) ApplicationRouteView {
	return ApplicationRouteView{
		Enabled:       spec.Enabled,
		Name:          strings.TrimSpace(spec.Name),
		Path:          strings.TrimSpace(spec.Path),
		Description:   strings.TrimSpace(spec.Description),
		Section:       strings.TrimSpace(spec.Section),
		Icon:          spec.Icon,
		MenuAvailable: boolOrDefault(spec.MenuAvailable, menu.Enabled),
	}
}

func buildMenuView(spec MenuSpec) MenuView {
	view := MenuView{
		Enabled:   spec.Enabled,
		Role:      strings.TrimSpace(spec.Role),
		Caption:   strings.TrimSpace(spec.Caption),
		Icon:      spec.Icon,
		SortOrder: spec.SortOrder,
		IsActive:  boolOrDefault(spec.IsActive, true),
	}
	if view.Role == "" {
		view.Role = "admin"
	}
	if spec.Parent != nil {
		view.HasParent = true
		view.ParentCaption = strings.TrimSpace(spec.Parent.Caption)
		view.ParentIcon = spec.Parent.Icon
		view.ParentSortOrder = spec.Parent.SortOrder
		view.CreateParent = boolOrDefault(spec.Parent.Create, true)
	}
	return view
}

func buildMigrationView(view ObjectView, spec ObjectSpec) MigrationView {
	migrationName := strings.TrimSpace(spec.Migration.Name)
	if migrationName == "" {
		migrationName = "create_" + view.TableName
	}
	result := MigrationView{
		Enabled: boolOrDefault(spec.Migration.Enabled, true),
		Name:    migrationName,
	}

	if len(view.Keys) > 1 {
		columns := make([]string, 0, len(view.Keys))
		for _, key := range view.Keys {
			columns = append(columns, key.Name)
		}
		result.TableConstraints = append(result.TableConstraints, "PRIMARY KEY ("+strings.Join(columns, ", ")+")")
	}
	for _, check := range spec.Table.Checks {
		check = strings.TrimSpace(check)
		if check != "" {
			result.TableConstraints = append(result.TableConstraints, "CHECK ("+check+")")
		}
	}
	for _, index := range spec.Migration.Indexes {
		method := strings.TrimSpace(index.Method)
		if method == "" {
			method = "btree"
		}
		result.Indexes = append(result.Indexes, IndexView{
			Name:       index.Name,
			ColumnsSQL: strings.Join(index.Columns, ", "),
			Unique:     index.Unique,
			Where:      strings.TrimSpace(index.Where),
			Method:     method,
		})
	}
	result.UpdatedAtTrigger = strings.TrimSpace(spec.Migration.UpdatedAtTrigger)
	if result.UpdatedAtTrigger != "" {
		result.HasUpdatedAtTrigger = true
		result.UpdatedAtFunction = view.TableName + "_set_updated_at"
	}

	return result
}

func buildServiceImports(view ObjectView) ServiceImportView {
	crud := view.GeneratedServiceCRUD
	anyCRUD := crud.Create || crud.List || crud.Detail || crud.Update || crud.Delete
	return ServiceImportView{
		Context:   anyCRUD,
		Errors:    crud.Detail,
		Fmt:       anyCRUD,
		Modelbind: crud.Create || crud.List,
		Models:    anyCRUD,
		Types:     crud.Delete,
		Session:   true,
		AppErrors: view.SessionRequired,
		Webapp:    true,
		WebModels: crud.List || crud.Update || crud.Delete,
	}
}

func generatedServiceCRUD(crud CRUDSpec, manual CRUDSpec) CRUDSpec {
	return CRUDSpec{
		Create: crud.Create && !manual.Create,
		List:   crud.List && !manual.List,
		Detail: crud.Detail && !manual.Detail,
		Update: crud.Update && !manual.Update,
		Delete: crud.Delete && !manual.Delete,
	}
}

func buildHTTPImports(view ObjectView) HTTPImportView {
	compositeUsesBinder := view.CompositeKey && (view.CRUD.Detail || view.CRUD.Update || view.CRUD.Delete)
	hasNumericCompositeKey := false
	for _, key := range view.Keys {
		if key.IsNumeric {
			hasNumericCompositeKey = true
		}
	}
	return HTTPImportView{
		Fmt:       compositeUsesBinder,
		HTTP:      view.CRUD.Create || compositeUsesBinder,
		Strconv:   compositeUsesBinder && hasNumericCompositeKey,
		Modelbind: view.CompositeKey && view.CRUD.Update,
		Models:    view.CRUD.Create || view.CRUD.Update || (view.CompositeKey && (view.CRUD.Detail || view.CRUD.Delete)),
		Webapp:    true,
	}
}

func schemaOrPublic(schema string) string {
	schema = strings.TrimSpace(schema)
	if schema == "" {
		return "public"
	}
	return schema
}

func relationName(schema string, table string) string {
	return schema + "." + strings.TrimSpace(table)
}

func itemRoute(base string, keys []KeyView) string {
	result := strings.TrimRight(base, "/")
	for _, key := range keys {
		result += "/{" + key.PathName + "}"
	}
	return result
}

func pluralHumanName(value string) string {
	if strings.HasSuffix(value, "y") && len(value) > 1 {
		return strings.TrimSuffix(value, "y") + "ies"
	}
	if strings.HasSuffix(value, "s") {
		return value
	}
	return value + "s"
}

func oneLineText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
