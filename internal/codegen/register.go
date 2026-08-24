package codegen

import (
	"bytes"
	"errors"
	"fmt"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const registerRuntimeSchema = "public"

var registerNumericTypePattern = regexp.MustCompile(`^numeric(?:\(([1-9][0-9]*),[[:space:]]*([0-9]+)\))?$`)
var registerVarcharTypePattern = regexp.MustCompile(`^(?:varchar|character varying)\(([1-9][0-9]*)\)$`)

type RegisterSpec struct {
	SourceFile string                  `yaml:"-"`
	Name       string                  `yaml:"name"`
	Comment    string                  `yaml:"comment"`
	Schema     string                  `yaml:"schema"`
	TableName  string                  `yaml:"tableName"`
	Kind       string                  `yaml:"kind"`
	Period     string                  `yaml:"period"`
	Dimensions []RegisterDimensionSpec `yaml:"dimensions"`
	Resources  []RegisterResourceSpec  `yaml:"resources"`
	Migration  RegisterMigrationSpec   `yaml:"migration"`
}

type RegisterDimensionSpec struct {
	Name       string         `yaml:"name"`
	Type       string         `yaml:"type"`
	SQLType    string         `yaml:"sqlType"`
	Comment    string         `yaml:"comment"`
	References *ReferenceSpec `yaml:"references"`
}

type RegisterResourceSpec struct {
	Name    string `yaml:"name"`
	Type    string `yaml:"type"`
	SQLType string `yaml:"sqlType"`
	Comment string `yaml:"comment"`
}

type RegisterMigrationSpec struct {
	Enabled *bool  `yaml:"enabled"`
	Name    string `yaml:"name"`
}

type RegisterView struct {
	SourceFile       string
	Name             string
	Snake            string
	FileBase         string
	Comment          string
	Schema           string
	RuntimeSchema    string
	RuntimeVersion   int
	ActionRelation   string
	PeriodRelation   string
	CurrentRelation  string
	Dimensions       []RegisterFieldView
	Resources        []RegisterFieldView
	MigrationEnabled bool
	MigrationName    string
}

type RegisterFieldView struct {
	Name             string
	Pascal           string
	FilterName       string
	FilterPascal     string
	SQLType          string
	FunctionSQLType  string
	ArraySQLType     string
	GoType           string
	GoSliceType      string
	Comment          string
	ReferenceClause  string
}

type RegisterRuntimeView struct {
	Version          int
	Schema           string
	BusinessTimezone string
	MigrationName    string
}

type RegistersView struct {
	Registers []RegisterView
}

type registerValueOwner struct {
	name   string
	source string
}

func LoadRegisters(schemaDir string) ([]RegisterSpec, error) {
	entries, err := os.ReadDir(schemaDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read register schema dir %s: %w", schemaDir, err)
	}

	registers := make([]RegisterSpec, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		path := filepath.Join(schemaDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}

		var register RegisterSpec
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		decoder.KnownFields(true)
		if err := decoder.Decode(&register); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		var extra any
		if err := decoder.Decode(&extra); err == nil {
			return nil, fmt.Errorf("parse %s: multiple YAML documents are not supported", path)
		} else if !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		register.SourceFile = path
		if err := register.Validate(); err != nil {
			return nil, fmt.Errorf("validate %s: %w", path, err)
		}
		registers = append(registers, register)
	}

	sort.Slice(registers, func(i int, j int) bool {
		return registers[i].Name < registers[j].Name
	})
	return registers, nil
}

func (s RegisterSpec) Validate() error {
	name := strings.TrimSpace(s.Name)
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if !token.IsIdentifier(PascalCase(name)) {
		return fmt.Errorf("name %q does not produce a valid Go identifier", s.Name)
	}

	schema := strings.TrimSpace(s.Schema)
	if schema != "" && !isSQLIdentifier(schema) {
		return fmt.Errorf("schema %q is not a safe SQL identifier", s.Schema)
	}
	tableName := strings.TrimSpace(s.TableName)
	if tableName == "" {
		tableName = SnakeCase(name)
	}
	if !isSQLIdentifier(tableName) {
		return fmt.Errorf("tableName %q is not a safe SQL identifier", tableName)
	}

	kind := strings.ToLower(strings.TrimSpace(s.Kind))
	if kind != "" && kind != "accumulation" {
		return fmt.Errorf("kind %q is unsupported; version 1 supports accumulation registers", s.Kind)
	}
	period := strings.ToLower(strings.TrimSpace(s.Period))
	if period != "" && period != "month" {
		return fmt.Errorf("period %q is unsupported; version 1 supports month", s.Period)
	}
	if len(s.Dimensions) == 0 {
		return fmt.Errorf("at least one dimension is required")
	}
	if len(s.Resources) == 0 {
		return fmt.Errorf("at least one resource is required")
	}

	reserved := map[string]struct{}{
		"id": {}, "effective_at": {}, "created_at": {}, "recorder_type": {},
		"recorder_id": {}, "period_start": {},
	}
	seen := make(map[string]string, len(s.Dimensions)+len(s.Resources))
	seenGoNames := make(map[string]string, len(s.Dimensions)+len(s.Resources))
	for _, dimension := range s.Dimensions {
		if err := validateRegisterFieldName(dimension.Name, "dimension", reserved, seen); err != nil {
			return err
		}
		if err := validateRegisterGoFieldName(dimension.Name, "dimension", seenGoNames); err != nil {
			return err
		}
		if _, _, _, err := registerDimensionTypes(dimension); err != nil {
			return fmt.Errorf("dimension %s: %w", dimension.Name, err)
		}
		if err := validateRegisterReference(dimension.Name, dimension.References); err != nil {
			return err
		}
	}
	for _, resource := range s.Resources {
		if err := validateRegisterFieldName(resource.Name, "resource", reserved, seen); err != nil {
			return err
		}
		if err := validateRegisterGoFieldName(resource.Name, "resource", seenGoNames); err != nil {
			return err
		}
		if _, _, _, err := registerResourceTypes(resource); err != nil {
			return fmt.Errorf("resource %s: %w", resource.Name, err)
		}
	}

	migrationName := strings.TrimSpace(s.Migration.Name)
	if migrationName != "" && !isMigrationName(migrationName) {
		return fmt.Errorf("migration.name %q is not a safe migration name", migrationName)
	}
	return nil
}

func validateRegisterGoFieldName(name string, kind string, seen map[string]string) error {
	goName := PascalCase(name)
	if previous, exists := seen[goName]; exists {
		return fmt.Errorf("%s %q and %s generate the same Go field %s", kind, name, previous, goName)
	}
	seen[goName] = kind + " " + name
	return nil
}

func validateRegisterFieldName(
	name string,
	kind string,
	reserved map[string]struct{},
	seen map[string]string,
) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("%s name is required", kind)
	}
	if !isSQLIdentifier(name) {
		return fmt.Errorf("%s %q is not a safe SQL identifier", kind, name)
	}
	if !token.IsIdentifier(PascalCase(name)) {
		return fmt.Errorf("%s %q does not produce a valid Go identifier", kind, name)
	}
	if _, exists := reserved[name]; exists {
		return fmt.Errorf("%s name %q is reserved by the register runtime", kind, name)
	}
	if previous, exists := seen[name]; exists {
		return fmt.Errorf("%s %q duplicates %s", kind, name, previous)
	}
	seen[name] = kind
	return nil
}

func validateRegisterReference(name string, reference *ReferenceSpec) error {
	if reference == nil {
		return nil
	}
	if strings.TrimSpace(reference.Table) == "" || !isSQLIdentifier(reference.Table) {
		return fmt.Errorf("dimension %s: references.table %q is not a safe SQL identifier", name, reference.Table)
	}
	if schema := strings.TrimSpace(reference.Schema); schema != "" && !isSQLIdentifier(schema) {
		return fmt.Errorf("dimension %s: references.schema %q is not a safe SQL identifier", name, schema)
	}
	if column := strings.TrimSpace(reference.Column); column != "" && !isSQLIdentifier(column) {
		return fmt.Errorf("dimension %s: references.column %q is not a safe SQL identifier", name, column)
	}
	if err := validateReferenceAction(reference.OnDelete); err != nil {
		return fmt.Errorf("dimension %s: onDelete: %w", name, err)
	}
	if err := validateReferenceAction(reference.OnUpdate); err != nil {
		return fmt.Errorf("dimension %s: onUpdate: %w", name, err)
	}
	for _, item := range []struct {
		label  string
		action string
	}{
		{label: "onDelete", action: reference.OnDelete},
		{label: "onUpdate", action: reference.OnUpdate},
	} {
		switch strings.ToUpper(strings.TrimSpace(item.action)) {
		case "SET NULL", "SET DEFAULT":
			return fmt.Errorf("dimension %s: %s %s is incompatible with non-null version 1 dimensions", name, item.label, item.action)
		}
	}
	return nil
}

func BuildRegisterView(spec RegisterSpec) (RegisterView, error) {
	if err := spec.Validate(); err != nil {
		return RegisterView{}, err
	}
	name := PascalCase(spec.Name)
	snake := strings.TrimSpace(spec.TableName)
	if snake == "" {
		snake = SnakeCase(spec.Name)
	}
	schema := schemaOrPublic(spec.Schema)
	comment := oneLineText(spec.Comment)
	if comment == "" {
		comment = name + " accumulation register."
	}
	migrationName := strings.TrimSpace(spec.Migration.Name)
	if migrationName == "" {
		migrationName = "create_" + snake + "_register"
	}

	view := RegisterView{
		SourceFile:       spec.SourceFile,
		Name:             name,
		Snake:            snake,
		FileBase:         CamelCase(name),
		Comment:          comment,
		Schema:           schema,
		RuntimeSchema:    registerRuntimeSchema,
		ActionRelation:   relationName(schema, "ra_"+snake),
		PeriodRelation:   relationName(schema, "rg_"+snake+"_period"),
		CurrentRelation:  relationName(schema, "rg_"+snake+"_current"),
		MigrationEnabled: boolOrDefault(spec.Migration.Enabled, true),
		MigrationName:    migrationName,
	}

	for _, dimension := range spec.Dimensions {
		sqlType, functionSQLType, goType, err := registerDimensionTypes(dimension)
		if err != nil {
			return RegisterView{}, fmt.Errorf("dimension %s: %w", dimension.Name, err)
		}
		filterName, filterPascal := registerFilterNames(dimension.Name)
		view.Dimensions = append(view.Dimensions, RegisterFieldView{
			Name:            dimension.Name,
			Pascal:          PascalCase(dimension.Name),
			FilterName:      filterName,
			FilterPascal:    filterPascal,
			SQLType:         sqlType,
			FunctionSQLType: functionSQLType,
			ArraySQLType:    functionSQLType + "[]",
			GoType:          goType,
			GoSliceType:     "[]" + goType,
			Comment:         oneLineText(dimension.Comment),
			ReferenceClause: registerReferenceClause(dimension.References),
		})
	}
	for _, resource := range spec.Resources {
		sqlType, functionSQLType, goType, err := registerResourceTypes(resource)
		if err != nil {
			return RegisterView{}, fmt.Errorf("resource %s: %w", resource.Name, err)
		}
		view.Resources = append(view.Resources, RegisterFieldView{
			Name:             resource.Name,
			Pascal:           PascalCase(resource.Name),
			SQLType:          sqlType,
			FunctionSQLType:  functionSQLType,
			GoType:           goType,
			Comment:          oneLineText(resource.Comment),
		})
	}
	return view, nil
}

func BuildRegisterRuntimeView(cfg Config) RegisterRuntimeView {
	return RegisterRuntimeView{
		Version:          cfg.RegisterRuntimeVersion,
		Schema:           registerRuntimeSchema,
		BusinessTimezone: cfg.RegisterBusinessTZ,
		MigrationName:    fmt.Sprintf("register_common_v%d", cfg.RegisterRuntimeVersion),
	}
}

func registerDimensionTypes(spec RegisterDimensionSpec) (string, string, string, error) {
	typeName := strings.ToLower(strings.TrimSpace(spec.Type))
	sqlType := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(spec.SQLType)), " "))
	switch typeName {
	case "int":
		if sqlType != "" && sqlType != "integer" {
			return "", "", "", fmt.Errorf("sqlType must be integer for type int")
		}
		return "integer", "integer", "int", nil
	case "bigint":
		if sqlType != "" && sqlType != "bigint" {
			return "", "", "", fmt.Errorf("sqlType must be bigint for type bigint")
		}
		return "bigint", "bigint", "int64", nil
	case "string":
		if sqlType == "" || sqlType == "text" {
			return "text", "text", "string", nil
		}
		matches := registerVarcharTypePattern.FindStringSubmatch(sqlType)
		if len(matches) == 2 {
			return "varchar(" + matches[1] + ")", "character varying", "string", nil
		}
		return "", "", "", fmt.Errorf("sqlType must be text or varchar(length) for type string")
	default:
		return "", "", "", fmt.Errorf("unsupported type %q; dimensions support int, bigint, and string", spec.Type)
	}
}

func registerResourceTypes(spec RegisterResourceSpec) (string, string, string, error) {
	typeName := strings.ToLower(strings.TrimSpace(spec.Type))
	sqlType := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(spec.SQLType)), " "))
	switch typeName {
	case "numeric":
		if sqlType == "" {
			sqlType = "numeric"
		}
		matches := registerNumericTypePattern.FindStringSubmatch(sqlType)
		if len(matches) == 0 {
			return "", "", "", fmt.Errorf("sqlType must be numeric or numeric(precision, scale)")
		}
		if len(matches) == 3 && matches[1] != "" {
			precision, _ := strconv.Atoi(matches[1])
			scale, _ := strconv.Atoi(matches[2])
			if precision > 1000 || scale > precision {
				return "", "", "", fmt.Errorf("numeric precision must be at most 1000 and scale must not exceed precision")
			}
			sqlType = fmt.Sprintf("numeric(%d, %d)", precision, scale)
		}
		return sqlType, "numeric", "float64", nil
	case "int":
		if sqlType != "" && sqlType != "integer" {
			return "", "", "", fmt.Errorf("sqlType must be integer for type int")
		}
		return "integer", "integer", "int", nil
	case "bigint":
		if sqlType != "" && sqlType != "bigint" {
			return "", "", "", fmt.Errorf("sqlType must be bigint for type bigint")
		}
		return "bigint", "bigint", "int64", nil
	default:
		return "", "", "", fmt.Errorf("unsupported type %q; resources support numeric, int, and bigint", spec.Type)
	}
}

func registerFilterNames(name string) (string, string) {
	switch {
	case strings.HasSuffix(name, "_id"):
		name = strings.TrimSuffix(name, "_id") + "_ids"
	case strings.HasSuffix(name, "_code"):
		name = strings.TrimSuffix(name, "_code") + "_codes"
	default:
		name += "_values"
	}
	return name, PascalCase(name)
}

func registerReferenceClause(reference *ReferenceSpec) string {
	if reference == nil {
		return ""
	}
	schema := schemaOrPublic(reference.Schema)
	column := strings.TrimSpace(reference.Column)
	if column == "" {
		column = "id"
	}
	parts := []string{"REFERENCES", relationName(schema, reference.Table) + "(" + column + ")"}
	if action := strings.ToUpper(strings.TrimSpace(reference.OnDelete)); action != "" {
		parts = append(parts, "ON DELETE", action)
	}
	if action := strings.ToUpper(strings.TrimSpace(reference.OnUpdate)); action != "" {
		parts = append(parts, "ON UPDATE", action)
	}
	return strings.Join(parts, " ")
}

func validateRegisterViews(objects []ObjectView, registers []RegisterView) error {
	relations := make(map[string]registerValueOwner)
	migrations := make(map[string]registerValueOwner)
	goNames := make(map[string]registerValueOwner)
	for _, object := range objects {
		relations[object.Relation] = registerValueOwner{name: object.Name, source: object.SourceFile}
		if object.HasCustomList {
			relations[object.ListRelation] = registerValueOwner{name: object.Name, source: object.SourceFile}
		}
		if object.Migration.Enabled {
			migrations[object.Migration.Name] = registerValueOwner{name: object.Name, source: object.SourceFile}
		}
	}
	if len(registers) > 0 {
		common := registerValueOwner{name: "register runtime", source: "embedded runtime v1"}
		commonRelation := relationName(registerRuntimeSchema, "register_settings")
		if previous, exists := relations[commonRelation]; exists {
			return duplicateRegisterValue("relation", commonRelation, previous, common)
		}
		relations[commonRelation] = common
		commonMigration := fmt.Sprintf("register_common_v%d", registers[0].RuntimeVersion)
		if previous, exists := migrations[commonMigration]; exists {
			return duplicateRegisterValue("migration name", commonMigration, previous, common)
		}
		migrations[commonMigration] = common
	}

	for _, register := range registers {
		current := registerValueOwner{name: register.Name, source: register.SourceFile}
		if register.FileBase == "registry" || register.Name == "All" {
			return fmt.Errorf("register %s (%s) collides with generated register repository infrastructure", register.Name, sourceLabel(register.SourceFile))
		}
		if previous, exists := goNames[register.Name]; exists {
			return duplicateRegisterValue("Go name", register.Name, previous, current)
		}
		goNames[register.Name] = current
		for _, relation := range []string{register.ActionRelation, register.PeriodRelation, register.CurrentRelation} {
			if previous, exists := relations[relation]; exists {
				return duplicateRegisterValue("relation", relation, previous, current)
			}
			relations[relation] = current
		}
		if register.MigrationEnabled {
			if previous, exists := migrations[register.MigrationName]; exists {
				return duplicateRegisterValue("migration name", register.MigrationName, previous, current)
			}
			migrations[register.MigrationName] = current
		}
	}
	return nil
}

func duplicateRegisterValue(kind string, value string, previous registerValueOwner, current registerValueOwner) error {
	return fmt.Errorf(
		"duplicate %s %q for %s (%s) and %s (%s)",
		kind,
		value,
		previous.name,
		sourceLabel(previous.source),
		current.name,
		sourceLabel(current.source),
	)
}
