package codegen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func enumString(value string) *string { return &value }

func enumObjectSpec() ObjectSpec {
	spec := ObjectSpec{
		Name: "MaterialStatus", Table: TableSpec{Schema: "public", Name: "material_statuses"},
		Route: "/material-statuses", Keys: []KeySpec{{Name: "id", Type: "int"}},
		Enums: []EnumSpec{{
			Name: "material_status_type", GoType: "MaterialStatusType", SQLType: "public.material_status_types", FrontendName: "MaterialStatusType",
			Values: []EnumValueSpec{{Value: "at_work", Label: "В работе"}, {Value: "on_maintenance", Label: "На обслуживании"}},
		}},
		Fields: []FieldSpec{
			{Name: "id", Type: "int", PrimaryKey: true, AutoIncrement: true, ServerGenerated: true},
			{Name: "created_at", Type: "timestamptz", Required: true, Default: "now()"},
			{Name: "material_id", Type: "int", Required: true},
			{Name: "status", Type: "enum", Enum: "material_status_type", Required: true, FrontendDefault: enumString("at_work")},
			{Name: "is_active", Type: "bool", Required: true, Default: "true"},
		},
		CRUD:             CRUDSpec{Create: true, List: true, Detail: true, Update: true, Delete: true},
		ApplicationRoute: ApplicationRouteSpec{Enabled: true, Name: "materialStatuses", Path: "/material-statuses", Description: "Статусы материалов", Section: "Inventory"},
		Frontend: FrontendSpec{Scaffold: true, Title: "Статусы материалов", List: FrontendListSpec{EditMode: "inline", Columns: []FrontendListColumnSpec{
			{Field: "id", Label: "ID"}, {Field: "created_at", Label: "Дата"},
			{Field: "material_id", Label: "Материал", Reference: &FrontendListReferenceSpec{Name: "materialReference", Import: "@/references/inventoryReferences", Field: "material"}, SortField: "material->>'descr'"},
			{Field: "status", Label: "Статус"}, {Field: "is_active", Label: "Активна"},
		}}},
	}
	spec.List = &ListSpec{
		Model: "MaterialStatusList", Table: TableSpec{Schema: "public", Name: "material_statuses_list"},
		Fields: []FieldSpec{
			{Name: "id", Type: "int", PrimaryKey: true},
			{Name: "created_at", Type: "timestamptz", Required: true, Default: "now()"},
			{Name: "material_id", Type: "int", Required: true},
			{Name: "material", Type: "jsonb", GoType: "*Ref", Required: true},
			{Name: "status", Type: "enum", Enum: "material_status_type", Required: true},
			{Name: "is_active", Type: "bool", Required: true, Default: "true"},
		},
	}
	spec.Detail = &ListSpec{Model: "MaterialStatusDetail", Table: TableSpec{Schema: "public", Name: "material_statuses_detail"}, Fields: append([]FieldSpec(nil), spec.List.Fields...)}
	return spec
}

func enumConfig(t *testing.T) Config {
	t.Helper()
	cfg := testConfig(t)
	cfg.ServerRoot = t.TempDir()
	cfg.FrontendRoot = filepath.Join(cfg.ServerRoot, "front")
	cfg.SchemaDir = filepath.Join(cfg.ServerRoot, "schema")
	cfg.FrontendEnabled = true
	cfg.APITestEnabled = false
	cfg.MigrationsEnabled = false
	cfg.RegistersEnabled = false
	cfg.AllowManualCollisions = false
	return cfg
}

func TestEnumDefinitionValidation(t *testing.T) {
	cases := []struct {
		name string
		edit func(*ObjectSpec)
		want string
	}{
		{"name", func(s *ObjectSpec) { s.Enums[0].Name = "" }, "enum name is required"},
		{"duplicate names", func(s *ObjectSpec) { s.Enums = append(s.Enums, s.Enums[0]) }, "duplicate enum name"},
		{"go type", func(s *ObjectSpec) { s.Enums[0].GoType = "" }, "goType is required"},
		{"frontend name", func(s *ObjectSpec) { s.Enums[0].FrontendName = "" }, "frontendName is required"},
		{"invalid frontend name", func(s *ObjectSpec) { s.Enums[0].FrontendName = "../../Type" }, "TypeScript type identifier"},
		{"no values", func(s *ObjectSpec) { s.Enums[0].Values = nil }, "has no values"},
		{"empty value", func(s *ObjectSpec) { s.Enums[0].Values[0].Value = "" }, "empty value"},
		{"duplicate value", func(s *ObjectSpec) { s.Enums[0].Values[1].Value = s.Enums[0].Values[0].Value }, "duplicate value"},
		{"unknown enum", func(s *ObjectSpec) { s.Fields[3].Enum = "missing" }, "unknown enum"},
		{"missing enum", func(s *ObjectSpec) { s.Fields[3].Enum = "" }, "requires an enum registry name"},
		{"conflicting go type", func(s *ObjectSpec) { s.Fields[3].GoType = "WrongType" }, "goType"},
		{"conflicting sql type", func(s *ObjectSpec) { s.Fields[3].SQLType = "public.wrong_types" }, "sqlType"},
		{"invalid default", func(s *ObjectSpec) { s.Fields[3].FrontendDefault = enumString("unknown") }, "default \"unknown\""},
		{"empty default", func(s *ObjectSpec) { s.Fields[3].FrontendDefault = enumString("") }, "default \"\""},
		{"missing inline default", func(s *ObjectSpec) { s.Fields[3].FrontendDefault = nil }, "requires an explicit valid frontendDefault"},
		{"nonliteral inline default", func(s *ObjectSpec) { s.Fields[3].FrontendDefault = nil; s.Fields[3].Default = "compute_status()" }, "requires an explicit valid frontendDefault"},
		{"string downgrade", func(s *ObjectSpec) { s.Fields[3].TSType = "string" }, "conflicts"},
		{"generic schema downgrade", func(s *ObjectSpec) { s.Fields[3].Valibot = "RequiredStringSchema" }, "custom validation must use"},
		{"enum column downgrade", func(s *ObjectSpec) { s.Frontend.List.Columns[3].DataType = "string" }, "must use dataType enum"},
		{"enum type without enum", func(s *ObjectSpec) { s.Frontend.List.Columns[0].DataType = "enum" }, "requires a field with a defined enum"},
		{"locale namespace collision", func(s *ObjectSpec) { s.Enums[0].FrontendName = "MaterialStatus" }, "conflicts with model"},
		{"generated DTO collision", func(s *ObjectSpec) { s.Enums[0].FrontendName = "MaterialStatusDTO" }, "conflicts with model"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := enumObjectSpec()
			tc.edit(&spec)
			_, err := BuildObjectView(enumConfig(t), spec)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
		})
	}
}

func TestEnumInferenceAndBackendCompatibility(t *testing.T) {
	cfg := enumConfig(t)
	spec := enumObjectSpec()
	view, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatal(err)
	}
	for _, fields := range [][]FieldView{view.Fields, view.ListFields, view.DetailFields} {
		for _, field := range fields {
			if field.Name != "status" {
				continue
			}
			if field.GoType != "MaterialStatusType" || field.SQLType != "public.material_status_types" || !strings.Contains(field.StructTag, `enum:"material_status_type"`) {
				t.Fatalf("backend enum metadata lost: %+v", field)
			}
		}
	}
	// Explicit, consistent metadata is accepted.
	spec.Fields[3].GoType = "MaterialStatusType"
	spec.Fields[3].SQLType = "public.material_status_types"
	if _, err := BuildObjectView(cfg, spec); err != nil {
		t.Fatal(err)
	}

	legacy := ObjectSpec{
		Name: "LegacyRole", Table: TableSpec{Name: "legacy_roles"}, Route: "/legacy-roles",
		Keys:   []KeySpec{{Name: "id", Type: "int"}},
		Fields: []FieldSpec{{Name: "id", Type: "int", PrimaryKey: true}, {Name: "role_id", Type: "enum", GoType: "RoleID", Enum: "role_id"}},
		CRUD:   CRUDSpec{Detail: true},
	}
	cfg.FrontendEnabled = false
	oldView, err := BuildObjectView(cfg, legacy)
	if err != nil {
		t.Fatal(err)
	}
	if oldView.Fields[1].GoType != "RoleID" || !strings.Contains(oldView.Fields[1].StructTag, `enum:"role_id"`) {
		t.Fatal("legacy backend enum changed")
	}
	cfg.FrontendEnabled = true
	if _, err := BuildObjectView(cfg, legacy); err == nil || !strings.Contains(err.Error(), "no enum values are defined") {
		t.Fatalf("legacy frontend enum must fail clearly: %v", err)
	}
}

func TestEnumSharedDefinitions(t *testing.T) {
	first := enumObjectSpec()
	first.SourceFile = "first.yaml"
	second := enumObjectSpec()
	second.Name, second.SourceFile = "SecondStatus", "second.yaml"
	second.Enums = nil
	resolved, err := resolveProjectEnums([]ObjectSpec{first, second})
	if err != nil {
		t.Fatal(err)
	}
	if resolved[1].Fields[3].GoType != "MaterialStatusType" {
		t.Fatal("cross-file enum did not resolve")
	}
	second.Enums = first.Enums
	if _, err := resolveProjectEnums([]ObjectSpec{first, second}); err != nil {
		t.Fatal("identical shared definitions must be accepted:", err)
	}
	second.Enums = append([]EnumSpec(nil), first.Enums...)
	second.Enums[0].Values = append([]EnumValueSpec(nil), first.Enums[0].Values...)
	second.Enums[0].Values[0].Label = "Different label"
	if _, err := resolveProjectEnums([]ObjectSpec{first, second}); err == nil || !strings.Contains(err.Error(), "conflicting definition") {
		t.Fatalf("expected label conflict, got %v", err)
	}
}

func TestEnumDefaultsAndOptionalFields(t *testing.T) {
	for _, tc := range []struct {
		name, sql, expected string
		explicit            *string
		nullable, optional  bool
	}{
		{name: "frontend", explicit: enumString("on_maintenance"), expected: `"on_maintenance"`},
		{name: "sql literal", sql: "'at_work'", expected: `"at_work"`},
		{name: "sql cast", sql: "'at_work'::public.material_status_types", expected: `"at_work"`},
		{name: "frontend overrides sql", explicit: enumString("on_maintenance"), sql: "'at_work'", expected: `"on_maintenance"`},
		{name: "nullable", nullable: true, expected: "null"},
		{name: "optional", optional: true, expected: "undefined"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec := enumObjectSpec()
			spec.Fields[3].FrontendDefault, spec.Fields[3].Default = tc.explicit, tc.sql
			spec.Fields[3].Required, spec.Fields[3].Nullable, spec.Fields[3].JSONOmitEmpty = !tc.nullable && !tc.optional, tc.nullable, tc.optional
			for _, projection := range []*ListSpec{spec.List, spec.Detail} {
				projection.Fields[4].Required, projection.Fields[4].Nullable, projection.Fields[4].JSONOmitEmpty = spec.Fields[3].Required, tc.nullable, tc.optional
			}
			view, err := BuildObjectView(enumConfig(t), spec)
			if err != nil {
				t.Fatal(err)
			}
			for _, field := range view.Frontend.List.CreateRowFields {
				if field.Field.Name == "status" && field.DefaultLiteral != tc.expected {
					t.Fatalf("default: %q, want %q", field.DefaultLiteral, tc.expected)
				}
			}
			if tc.nullable {
				if view.Fields[3].TSType != "MaterialStatusType | null" || view.Fields[3].ValibotModel != "v.nullable(MaterialStatusTypeSchema)" {
					t.Fatal("nullable enum contract lost")
				}
			}
		})
	}
}

func TestEnumFrontendRendering(t *testing.T) {
	cfg := enumConfig(t)
	spec := enumObjectSpec()
	// A second field verifies import and schema-instance deduplication.
	spec.Fields = append(spec.Fields, FieldSpec{Name: "previous_status", Type: "enum", Enum: "material_status_type", Nullable: true})
	view, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatal(err)
	}
	renderer := NewRenderer(cfg)
	if err := renderer.RenderFrontendEnums([]ObjectView{view}); err != nil {
		t.Fatal(err)
	}
	if err := renderer.RenderObject(view); err != nil {
		t.Fatal(err)
	}
	if err := renderer.RenderFrontendLocale([]ObjectView{view}); err != nil {
		t.Fatal(err)
	}
	checks := map[string][]string{
		"src/types/enums/materialStatusType.gen.ts":       {`MATERIAL_STATUS_TYPE_VALUES = [`, `"at_work",`, `"on_maintenance",`, `] as const;`, `(typeof MATERIAL_STATUS_TYPE_VALUES)[number]`, `value: MaterialStatusType;`},
		"src/schemas/enums/materialStatusType.gen.ts":     {`v.picklist(`, `MATERIAL_STATUS_TYPE_VALUES,`, `t("validation.required")`, `createMaterialStatusTypeSchemas(defaultTranslate)`},
		"src/types/materialStatus.gen.ts":                 {`MaterialStatusType,`, `status: MaterialStatusType;`, `previous_status: MaterialStatusType | null;`, `MaterialStatusNew = Pick<`, `MaterialStatusUpd = Partial<Pick<`},
		"src/types/materialStatusList.gen.ts":             {`status: MaterialStatusType;`},
		"src/types/materialStatusDetail.gen.ts":           {`status: MaterialStatusType;`},
		"src/schemas/materialStatus.gen.ts":               {`createMaterialStatusTypeSchemas(t)`, `status: MaterialStatusTypeSchema`, `v.nullable(MaterialStatusTypeSchema)`},
		"src/schemas/materialStatusList.gen.ts":           {`createMaterialStatusTypeSchemas(t)`, `status: MaterialStatusTypeSchema`},
		"src/schemas/materialStatusDetail.gen.ts":         {`createMaterialStatusTypeSchemas(t)`, `status: MaterialStatusTypeSchema`},
		"src/collections/materialStatus.gen.ts":           {`dataType: "enum"`, `enumOptions: MATERIAL_STATUS_TYPE_VALUES.map((value) => ({`, "labelKey: `MaterialStatusType.${value}`", `"showClear": false`, `status: "at_work"`, `dataType: "datetime"`, `reference: materialReference`, `sortField: "material->>'descr'"`},
		"src/views/materialStatus/MaterialStatusList.vue": {`CollectionListPage`},
		"src/locales/ru.gen.json":                         {`"MaterialStatusType":`, `"at_work": "В работе"`, `"on_maintenance": "На обслуживании"`, `"material_id": "Материал"`},
	}
	for name, expected := range checks {
		content := readTestFile(t, filepath.Join(cfg.FrontendRoot, name))
		for _, value := range expected {
			if !strings.Contains(content, value) {
				t.Errorf("%s missing %q\n%s", name, value, content)
			}
		}
		if strings.Contains(content, `status: ""`) || strings.Contains(content, `status: string`) {
			t.Errorf("%s retained untyped enum fallback", name)
		}
	}
	for _, name := range []string{"materialStatus", "materialStatusList", "materialStatusDetail"} {
		schema := readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/schemas/"+name+".gen.ts"))
		if strings.Contains(schema, "RequiredStringSchema") || strings.Count(schema, "createMaterialStatusTypeSchemas(t)") != 1 {
			t.Fatalf("enum schema imports/instances not deduplicated: %s", name)
		}
		types := readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/types/"+name+".gen.ts"))
		if strings.Count(types, `from "@/types/enums/materialStatusType.gen"`) != 1 {
			t.Fatal("duplicate enum type import", name)
		}
	}
	var locale map[string]any
	if err := json.Unmarshal([]byte(readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/locales/ru.gen.json"))), &locale); err != nil {
		t.Fatal(err)
	}
	goModel := readTestFile(t, filepath.Join(cfg.ServerRoot, "internal/models/materialStatus.gen.go"))
	if strings.Contains(goModel, "type MaterialStatusType ") || strings.Contains(goModel, "RegisterModelbindEnums") {
		t.Fatal("application-owned backend enum declarations were generated")
	}

	cfg.Check = true
	if err := NewRenderer(cfg).RenderFrontendEnums([]ObjectView{view}); err != nil {
		t.Fatal(err)
	}
	if err := NewRenderer(cfg).RenderObject(view); err != nil {
		t.Fatal(err)
	}
	if err := NewRenderer(cfg).RenderFrontendLocale([]ObjectView{view}); err != nil {
		t.Fatal(err)
	}
}

func TestEnumColumnOverridesAndNullableClear(t *testing.T) {
	spec := enumObjectSpec()
	spec.Fields[3].Required, spec.Fields[3].Nullable = false, true
	for _, p := range []*ListSpec{spec.List, spec.Detail} {
		p.Fields[4].Required, p.Fields[4].Nullable = false, true
	}
	column := &spec.Frontend.List.Columns[3]
	column.Width, column.Format, column.SearchField, column.SearchDataType = "25rem", "(value) => String(value ?? '')", "status::text", "string"
	column.Searchable = new(bool)
	column.SearchOperations = []string{"contains", "exact"}
	column.EditorProps = map[string]any{"filter": true}
	cfg := enumConfig(t)
	view, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := NewRenderer(cfg).RenderObject(view); err != nil {
		t.Fatal(err)
	}
	content := readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/collections/materialStatus.gen.ts"))
	for _, part := range []string{`"showClear": true`, `"filter": true`, `width: "25rem"`, `searchable: false`, `searchField: "status::text"`, `searchDataType: "string"`, `searchOperations: ["contains", "exact"]`, `format: (value) => String(value ?? '')`} {
		if !strings.Contains(content, part) {
			t.Errorf("missing override %q", part)
		}
	}
	column.EditorProps["showClear"] = false
	view, err = BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatal(err)
	}
	props := view.Frontend.List.Columns[3].EditorProps
	if props[len(props)-1].Literal != "false" {
		t.Fatalf("explicit showClear was not preserved: %+v", props)
	}
}

func TestEnumFormDraftAndSelect(t *testing.T) {
	spec := enumObjectSpec()
	spec.Fields = []FieldSpec{spec.Fields[0], spec.Fields[3]}
	spec.Fields[1].FrontendDefault = nil
	spec.List, spec.Detail = nil, nil
	spec.Frontend.List = FrontendListSpec{}
	cfg := enumConfig(t)
	view, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatal(err)
	}
	if view.Frontend.Form.Fields[0].DefaultLiteral != "undefined" {
		t.Fatal("non-inline form must allow an unselected draft")
	}
	if err := NewRenderer(cfg).RenderObject(view); err != nil {
		t.Fatal(err)
	}
	form := readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/components/materialStatus/MaterialStatusForm.vue"))
	for _, part := range []string{`import Select from "primevue/select"`, `optionValue="value"`, `MATERIAL_STATUS_TYPE_VALUES.map`, `submit: [model: MaterialStatusFormModel]`} {
		if !strings.Contains(form, part) {
			t.Errorf("enum form missing %q", part)
		}
	}
	if strings.Contains(form, `status?.trim()`) || strings.Contains(form, `status: ""`) {
		t.Fatal("enum form retained a string coercion")
	}
}

func TestEnumOwnershipCleanupAndCheck(t *testing.T) {
	cfg := enumConfig(t)
	view, err := BuildObjectView(cfg, enumObjectSpec())
	if err != nil {
		t.Fatal(err)
	}
	r := NewRenderer(cfg)
	if err := r.RenderFrontendEnums([]ObjectView{view, view}); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(cfg.FrontendRoot, "src/types/enums/materialStatusType.gen.ts")
	manual := filepath.Join(cfg.FrontendRoot, "src/types/enums/manual.ts")
	if err := os.WriteFile(manual, []byte("export type Manual = string;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	check := cfg
	check.Check = true
	if err := NewRenderer(check).RenderFrontendEnums(nil); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("check did not detect removed enum: %v", err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatal("check deleted a file")
	}
	if err := os.WriteFile(file, []byte("// developer-owned\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := r.RenderFrontendEnums(nil); err == nil || !strings.Contains(err.Error(), "developer-owned") {
		t.Fatalf("must protect manual file: %v", err)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := r.RenderFrontendEnums([]ObjectView{view}); err != nil {
		t.Fatal(err)
	}
	if err := r.RenderFrontendEnums(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("stale enum was not removed: %v", err)
	}
	if _, err := os.Stat(manual); err != nil {
		t.Fatal("manual source deleted")
	}
	if err := NewRenderer(check).RenderFrontendEnums(nil); err != nil {
		t.Fatal(err)
	}
	for _, unsafe := range []string{"../manual.ts", "/src/types/enums/x.gen.ts", "src/types/enums/../../x.gen.ts", "src/types/enums/x.ts"} {
		if safeEnumArtifactPath(unsafe) {
			t.Errorf("accepted unsafe manifest entry %q", unsafe)
		}
	}
}

func TestEnumImportDeduplication(t *testing.T) {
	spec := enumObjectSpec()
	spec.Frontend.TypeImports = []FrontendImportSpec{{From: "@/types/enums/materialStatusType.gen", Names: []string{"MaterialStatusType"}, TypeOnly: true}}
	view, err := BuildObjectView(enumConfig(t), spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Frontend.TypeImports) != 1 || !reflect.DeepEqual(view.Frontend.TypeImports[0].Names, []string{"MaterialStatusType"}) {
		t.Fatal("manual/auto enum imports not deduplicated")
	}
	spec.Frontend.TypeImports[0].From = "@/wrong"
	if _, err := BuildObjectView(enumConfig(t), spec); err == nil || !strings.Contains(err.Error(), "duplicate frontend import") {
		t.Fatalf("expected import conflict, got %v", err)
	}
}

func TestEnumDependencyRequirement(t *testing.T) {
	cfg := enumConfig(t)
	view, err := BuildObjectView(cfg, enumObjectSpec())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cfg.FrontendRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(cfg.FrontendRoot, "package.json")
	for _, version := range []string{"^0.1.10", "0.1.10", "~0.1.11", ">=0.1.10", "^0.2.0", "^0.1.9", "*", "latest", "0.1.10-beta.1"} {
		content, _ := json.Marshal(map[string]any{"dependencies": map[string]string{"@katren/vue-collection-lib": version}})
		if err := os.WriteFile(file, content, 0o644); err != nil {
			t.Fatal(err)
		}
		err := validateEnumCollectionDependency(cfg.FrontendRoot, []ObjectView{view})
		if (err == nil) != enumLibraryRangeCompatible(version) {
			t.Fatalf("version %s: %v", version, err)
		}
		after, _ := os.ReadFile(file)
		if string(after) != string(content) {
			t.Fatal("package.json was modified")
		}
	}
	if err := validateEnumCollectionDependency("missing", nil); err != nil {
		t.Fatal("non-enum project must not need a package manifest", err)
	}
}

func TestLoadObjectsParsesFrontendEnums(t *testing.T) {
	cfg := enumConfig(t)
	if err := os.MkdirAll(cfg.SchemaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "examples", "material_status_enums.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.SchemaDir, "materialStatus.yaml"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	objects, err := LoadObjects(cfg.SchemaDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(objects) != 1 || len(objects[0].Enums) != 1 || objects[0].Fields[3].GoType != "MaterialStatusType" {
		t.Fatal("enum definition was not parsed/inferred")
	}
	if err := os.MkdirAll(cfg.FrontendRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.FrontendRoot, "package.json"), []byte(`{"dependencies":{"@katren/vue-collection-lib":"^0.1.10"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Validate(cfg); err != nil {
		t.Fatal(err)
	}
	if err := Generate(cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Check = true
	if err := Generate(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestEnumProjectionCannotDowngradeToText(t *testing.T) {
	for _, location := range []string{"list", "detail"} {
		t.Run(location, func(t *testing.T) {
			spec := enumObjectSpec()
			projection := spec.List
			if location == "detail" {
				projection = spec.Detail
			}
			projection.Fields[4].Type = "text"
			projection.Fields[4].Enum = ""
			if _, err := BuildObjectView(enumConfig(t), spec); err == nil || !strings.Contains(err.Error(), "must preserve the base field's enum") {
				t.Fatalf("expected projection enum mismatch, got %v", err)
			}
		})
	}
}

func TestEnumUnusedColumnDoesNotImportValues(t *testing.T) {
	spec := enumObjectSpec()
	spec.Frontend.List.Columns = append(spec.Frontend.List.Columns[:3], spec.Frontend.List.Columns[4:]...)
	cfg := enumConfig(t)
	view, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatal(err)
	}
	content, err := NewRenderer(cfg).executeTemplate("vue/collection.ts.tmpl", view)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "MATERIAL_STATUS_TYPE_VALUES") {
		t.Fatal("hidden enum column emitted an unused values import")
	}
	if !strings.Contains(string(content), `status: "at_work"`) {
		t.Fatal("hidden enum field lost its required create default")
	}
}

func TestEnumEscapingAndFormComponentValidation(t *testing.T) {
	spec := enumObjectSpec()
	spec.Enums[0].Values = []EnumValueSpec{{Value: "value\x00\a\"'", Label: "Label\a\n\""}}
	spec.Fields[3].FrontendDefault = enumString(spec.Enums[0].Values[0].Value)
	cfg := enumConfig(t)
	view, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := NewRenderer(cfg).RenderFrontendLocale([]ObjectView{view}); err != nil {
		t.Fatal(err)
	}
	content := readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/locales/ru.gen.json"))
	if !json.Valid([]byte(content)) {
		t.Fatalf("enum labels are not JSON escaped: %s", content)
	}
	enumContent, err := NewRenderer(cfg).executeTemplate("vue/enumTypes.ts.tmpl", view.Frontend.Enums[0])
	if err != nil || !strings.Contains(string(enumContent), `\u0000\u0007`) {
		t.Fatalf("enum values not safely escaped for TypeScript: %s (%v)", enumContent, err)
	}

	spec = enumObjectSpec()
	spec.Frontend.Form.Fields = []FrontendFormFieldSpec{{Field: "status", Component: "text"}}
	if _, err := BuildObjectView(cfg, spec); err == nil || !strings.Contains(err.Error(), "requires a Select") {
		t.Fatalf("expected unsafe string form editor error: %v", err)
	}
}

func TestEnumCheckDetectsMissingAndChangedFiles(t *testing.T) {
	cfg := enumConfig(t)
	view, err := BuildObjectView(cfg, enumObjectSpec())
	if err != nil {
		t.Fatal(err)
	}
	r := NewRenderer(cfg)
	if err := r.RenderFrontendEnums([]ObjectView{view}); err != nil {
		t.Fatal(err)
	}
	check := cfg
	check.Check = true
	target := filepath.Join(cfg.FrontendRoot, "src/schemas/enums/materialStatusType.gen.ts")
	original := readTestFile(t, target)
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := NewRenderer(check).RenderFrontendEnums([]ObjectView{view}); err == nil {
		t.Fatal("check did not detect missing enum schema")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("check created the missing file")
	}
	if err := os.WriteFile(target, []byte(original+"\n// stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := NewRenderer(check).RenderFrontendEnums([]ObjectView{view}); err == nil {
		t.Fatal("check did not detect modified managed enum schema")
	}
	if err := r.RenderFrontendEnums([]ObjectView{view}); err != nil {
		t.Fatal(err)
	}
	if err := NewRenderer(check).RenderFrontendEnums([]ObjectView{view}); err != nil {
		t.Fatal(err)
	}
}

func TestEnumLocalLibraryDependency(t *testing.T) {
	cfg := enumConfig(t)
	view, err := BuildObjectView(cfg, enumObjectSpec())
	if err != nil {
		t.Fatal(err)
	}
	library := filepath.Join(cfg.ServerRoot, "collection-lib")
	if err := os.MkdirAll(library, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cfg.FrontendRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.FrontendRoot, "package.json"), []byte(`{"dependencies":{"@katren/vue-collection-lib":"file:../collection-lib"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"0.1.9", "0.1.10"} {
		content, _ := json.Marshal(map[string]string{"version": version})
		if err := os.WriteFile(filepath.Join(library, "package.json"), content, 0o644); err != nil {
			t.Fatal(err)
		}
		err := validateEnumCollectionDependency(cfg.FrontendRoot, []ObjectView{view})
		if (err == nil) != (version == "0.1.10") {
			t.Fatalf("local version %s: %v", version, err)
		}
	}
}

func TestNonEnumProjectDoesNotCreateEnumArtifacts(t *testing.T) {
	cfg := enumConfig(t)
	spec := enumObjectSpec()
	spec.Enums = nil
	spec.Fields[3].Type, spec.Fields[3].Enum, spec.Fields[3].FrontendDefault = "text", "", nil
	for _, p := range []*ListSpec{spec.List, spec.Detail} {
		p.Fields[4].Type, p.Fields[4].Enum = "text", ""
	}
	view, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := NewRenderer(cfg).RenderFrontendEnums([]ObjectView{view}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cfg.FrontendRoot); !os.IsNotExist(err) {
		t.Fatal("non-enum project unexpectedly received enum output or manifest")
	}
	if len(view.Frontend.Enums) != 0 || len(view.Frontend.List.Enums) != 0 {
		t.Fatal("non-enum fields unexpectedly acquired enum metadata")
	}
}

func TestEnumNaturalKeyFrontendContract(t *testing.T) {
	spec := enumObjectSpec()
	spec.Fields = []FieldSpec{spec.Fields[3], {Name: "name", Type: "text", Required: true}}
	spec.Fields[0].PrimaryKey = true
	spec.Fields[0].FrontendDefault = nil
	spec.Keys = []KeySpec{{Name: "status", Type: "enum"}}
	spec.List, spec.Detail = nil, nil
	spec.Frontend.List = FrontendListSpec{}
	cfg := enumConfig(t)
	view, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := NewRenderer(cfg).RenderObject(view); err != nil {
		t.Fatal(err)
	}
	api := readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/api/materialStatus.gen.ts"))
	if !strings.Contains(api, `getKeyValue: (key: MaterialStatusKey): MaterialStatusKey["status"]`) {
		t.Fatal("enum key annotation requires an unimported type")
	}
	page := readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/views/materialStatus/MaterialStatusEditPage.vue"))
	if !strings.Contains(page, "v.parse(schemas.MaterialStatusKeySchema") || strings.Contains(page, "${detail.status}_COPY") {
		t.Fatal("enum route/copy keys were not validated/reset")
	}
}
