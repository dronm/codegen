package codegen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoCaseUsesProjectInitialisms(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		pascal string
		camel  string
	}{
		"id":              {pascal: "ID", camel: "id"},
		"product_id":      {pascal: "ProductID", camel: "productID"},
		"stored_url":      {pascal: "StoredURL", camel: "storedURL"},
		"ProxyAI":         {pascal: "ProxyAI", camel: "proxyAI"},
		"product_agc":     {pascal: "ProductAgc", camel: "productAgc"},
		"http_api_url_id": {pascal: "HTTPAPIURLID", camel: "httpAPIURLID"},
	}

	for input, expected := range tests {
		if actual := PascalCase(input); actual != expected.pascal {
			t.Errorf("PascalCase(%q): got %q, want %q", input, actual, expected.pascal)
		}
		if actual := CamelCase(input); actual != expected.camel {
			t.Errorf("CamelCase(%q): got %q, want %q", input, actual, expected.camel)
		}
	}
}

func TestLoadObjectsRejectsUnknownSchemaFields(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content := `{
		"name":"Example",
		"table":{"name":"examples"},
		"route":"/examples",
		"keys":[{"name":"id","type":"int"}],
		"fields":[{"name":"id","type":"int","primaryKey":true}],
		"crud":{"detail":true},
		"unknownProperty":true
	}`
	if err := os.WriteFile(filepath.Join(dir, "example.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}

	_, err := LoadObjects(dir)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "unknown") {
		t.Fatalf("expected unknown-field error, got %v", err)
	}
}

func TestLoadObjectsSupportsManualServiceMethods(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content := `{
		"name":"Example",
		"table":{"name":"examples"},
		"route":"/examples",
		"keys":[{"name":"id","type":"int"}],
		"fields":[
			{"name":"id","type":"int","primaryKey":true},
			{"name":"name","type":"string"}
		],
		"crud":{"create":true,"update":true},
		"service":{"manualMethods":["create","update"]}
	}`
	if err := os.WriteFile(filepath.Join(dir, "example.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}

	objects, err := LoadObjects(dir)
	if err != nil {
		t.Fatalf("LoadObjects(): %v", err)
	}
	if len(objects) != 1 || len(objects[0].Service.ManualMethods) != 2 {
		t.Fatalf("manual service methods were not loaded: %+v", objects)
	}
}

func TestModulePathFromGoMod(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "go.mod")
	if err := os.WriteFile(path, []byte("module example.com/project\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}

	modulePath, err := modulePathFromGoMod(path)
	if err != nil {
		t.Fatalf("modulePathFromGoMod(): %v", err)
	}
	if modulePath != "example.com/project" {
		t.Fatalf("unexpected module path %q", modulePath)
	}
}

func TestCompositeSpecValidation(t *testing.T) {
	t.Parallel()

	if err := compositeObjectSpec().Validate(); err != nil {
		t.Fatalf("Validate(): %v", err)
	}
}

func TestSpecValidationSupportsManualServiceMethods(t *testing.T) {
	t.Parallel()

	spec := compositeObjectSpec()
	spec.Service.ManualMethods = []string{"Create", " update "}
	if err := spec.Validate(); err != nil {
		t.Fatalf("Validate(): %v", err)
	}

	view, err := BuildObjectView(testConfig(t), spec)
	if err != nil {
		t.Fatalf("BuildObjectView(): %v", err)
	}
	if !view.ManualServiceCRUD.Create || !view.ManualServiceCRUD.Update {
		t.Fatalf("manual service methods were not preserved: %+v", view.ManualServiceCRUD)
	}
	if view.GeneratedServiceCRUD.Create || view.GeneratedServiceCRUD.Update {
		t.Fatalf("manual service methods must not be generated: %+v", view.GeneratedServiceCRUD)
	}
	if !view.GeneratedServiceCRUD.List || !view.GeneratedServiceCRUD.Detail || !view.GeneratedServiceCRUD.Delete {
		t.Fatalf("unowned service methods must remain generated: %+v", view.GeneratedServiceCRUD)
	}
}

func TestSpecValidationRejectsInvalidManualServiceMethods(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		methods []string
		crud    CRUDSpec
		want    string
	}{
		{
			name:    "unsupported",
			methods: []string{"archive"},
			crud:    CRUDSpec{Create: true},
			want:    `unsupported method "archive"`,
		},
		{
			name:    "disabled",
			methods: []string{"update"},
			crud:    CRUDSpec{Create: true},
			want:    `method "update" requires crud.update: true`,
		},
		{
			name:    "duplicate",
			methods: []string{"create", "Create"},
			crud:    CRUDSpec{Create: true},
			want:    `duplicate method "create"`,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			spec := compositeObjectSpec()
			spec.CRUD = test.crud
			spec.Service.ManualMethods = test.methods
			err := spec.Validate()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected error containing %q, got %v", test.want, err)
			}
		})
	}
}

func TestSpecValidationRequiresEveryPrimaryKey(t *testing.T) {
	t.Parallel()

	spec := compositeObjectSpec()
	spec.Keys = spec.Keys[:1]

	err := spec.Validate()
	if err == nil || !strings.Contains(err.Error(), "keys must contain every primary-key field") {
		t.Fatalf("expected primary-key coverage error, got %v", err)
	}
}

func TestBuildObjectViewRejectsUpdateWithoutWritableFields(t *testing.T) {
	t.Parallel()

	spec := compositeObjectSpec()
	spec.Fields = spec.Fields[:2]
	spec.List = nil
	spec.CRUD = CRUDSpec{Update: true}

	_, err := BuildObjectView(testConfig(t), spec)
	if err == nil || !strings.Contains(err.Error(), "no writable non-key fields") {
		t.Fatalf("expected no-writable-fields error, got %v", err)
	}
}

func TestSpecValidationRejectsWritableJSONHiddenField(t *testing.T) {
	t.Parallel()

	spec := compositeObjectSpec()
	hidden := false
	spec.Fields[2].JSON = &hidden

	err := spec.Validate()
	if err == nil || !strings.Contains(err.Error(), "generic CRUD binders cannot populate it") {
		t.Fatalf("expected writable-hidden-field error, got %v", err)
	}
}

func TestBuildObjectViewRejectsKeyTypeMismatch(t *testing.T) {
	t.Parallel()

	spec := compositeObjectSpec()
	spec.Keys[1].Type = "int"

	_, err := BuildObjectView(testConfig(t), spec)
	if err == nil || !strings.Contains(err.Error(), "does not match field Go type") {
		t.Fatalf("expected key-type mismatch error, got %v", err)
	}
}

func TestBuildObjectViewUsesEffectiveKeyJSONName(t *testing.T) {
	t.Parallel()

	spec := compositeObjectSpec()
	spec.Fields[0].JSONName = "group"

	view, err := BuildObjectView(testConfig(t), spec)
	if err != nil {
		t.Fatalf("BuildObjectView(): %v", err)
	}
	if view.Keys[0].JSONName != "group" {
		t.Fatalf("unexpected key JSON name %q", view.Keys[0].JSONName)
	}
	if !strings.Contains(view.Keys[0].StructTag, `json:"group"`) {
		t.Fatalf("unexpected key struct tag %s", view.Keys[0].StructTag)
	}
}

func TestValidateObjectViewsRejectsDuplicateServiceName(t *testing.T) {
	t.Parallel()

	first, err := BuildObjectView(testConfig(t), compositeObjectSpec())
	if err != nil {
		t.Fatalf("BuildObjectView(first): %v", err)
	}
	secondSpec := compositeObjectSpec()
	secondSpec.Name = "OtherCatalogueItem"
	secondSpec.Table.Name = "other_catalogue_items"
	secondSpec.Route = "/other-catalogue-items"
	secondSpec.PermissionPrefix = "otherCatalogueItem"
	secondSpec.ServiceName = first.ServiceName
	secondSpec.List = nil
	second, err := BuildObjectView(testConfig(t), secondSpec)
	if err != nil {
		t.Fatalf("BuildObjectView(second): %v", err)
	}

	err = validateObjectViews([]ObjectView{first, second})
	if err == nil || !strings.Contains(err.Error(), "duplicate service name") {
		t.Fatalf("expected duplicate-service error, got %v", err)
	}
}

func TestValidateManualBackendCollisionsFindsRegistrations(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	servicePath := filepath.Join(root, "internal", "services", "legacy.go")
	routePath := filepath.Join(root, "internal", "httpapi", "legacy.go")
	modelPath := filepath.Join(root, "internal", "models", "legacy.go")
	for _, path := range []string{servicePath, routePath, modelPath} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll(): %v", err)
		}
	}
	if err := os.WriteFile(servicePath, []byte(`package services
import "github.com/dronm/webapp"
func registerLegacy() { webapp.MustRegisterService("CatalogueItem", nil, nil) }
func (s *CatalogueItemService) Create() {}
`), 0o644); err != nil {
		t.Fatalf("WriteFile(service): %v", err)
	}
	if err := os.WriteFile(routePath, []byte(`package httpapi
import "github.com/dronm/webapp"
func legacyRoutes(api *webapp.Group) {
	api.GET("/catalogue-items", webapp.WithName("catalogueItem.list"), webapp.WithPermission("catalogueItem.list"))
}
`), 0o644); err != nil {
		t.Fatalf("WriteFile(route): %v", err)
	}
	if err := os.WriteFile(modelPath, []byte("package models\ntype CatalogueItemKey struct{}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(model): %v", err)
	}

	cfg := testConfig(t)
	view, err := BuildObjectView(cfg, compositeObjectSpec())
	if err != nil {
		t.Fatalf("BuildObjectView(): %v", err)
	}
	err = validateManualBackendCollisions(root, []ObjectView{view})
	if err == nil {
		t.Fatal("expected manual backend collision")
	}
	for _, expected := range []string{
		"model type \"CatalogueItemKey\"",
		"registered service name \"CatalogueItem\"",
		"service method \"CatalogueItemService.Create\"",
		"HTTP route \"GET /catalogue-items\"",
		"route name \"catalogueItem.list\"",
		"permission name \"catalogueItem.list\"",
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("collision error missing %q: %v", expected, err)
		}
	}
}

func TestValidateManualBackendCollisionsRequiresOwnedServiceMethods(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := testConfig(t)
	spec := compositeObjectSpec()
	spec.Service.ManualMethods = []string{"create", "update"}
	view, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatalf("BuildObjectView(): %v", err)
	}

	err = validateManualBackendCollisions(root, []ObjectView{view})
	if err == nil || !strings.Contains(err.Error(), `manual service method "CatalogueItemService.Create"`) ||
		!strings.Contains(err.Error(), `manual service method "CatalogueItemService.Update"`) {
		t.Fatalf("expected missing manual-method error, got %v", err)
	}

	path := filepath.Join(root, "internal", "services", "catalogueItem_custom.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(): %v", err)
	}
	content := `package services
func (s *CatalogueItemService) Create() {}
func (s *CatalogueItemService) Update() {}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}
	if err := validateManualBackendCollisions(root, []ObjectView{view}); err != nil {
		t.Fatalf("manual service methods must satisfy ownership: %v", err)
	}
}

func TestValidateManualFrontendCollisionsFindsFilesRoutesAndLocale(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := map[string]string{
		filepath.Join(root, "src", "api", "warehouseZone.ts"): `export const warehouseZoneApi = {};\n`,
		filepath.Join(root, "src", "router", "routeManifest.ts"): `export const routeManifest = [
	{
		route: {
			path: "/warehouse-zones",
			name: "warehouseZones",
		},
	},
];
`,
		filepath.Join(root, "src", "locales", "ru.json"): `{"WarehouseZone":{"title":"Ручная зона"}}`,
	}
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll(): %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%s): %v", path, err)
		}
	}

	spec := ObjectSpec{
		Name:            "WarehouseZone",
		HumanName:       "warehouse zone",
		HumanNamePlural: "warehouse zones",
		Table:           TableSpec{Schema: "public", Name: "warehouse_zones"},
		Route:           "/warehouse-zones",
		Keys:            []KeySpec{{Name: "id", PathName: "id", Type: "int"}},
		Fields: []FieldSpec{
			{Name: "id", Type: "int", PrimaryKey: true, AutoIncrement: true, ServerGenerated: true},
			{Name: "name", Type: "string", Required: true},
		},
		CRUD: CRUDSpec{Create: true, List: true, Detail: true, Update: true, Delete: true},
		ApplicationRoute: ApplicationRouteSpec{
			Enabled:     true,
			Name:        "warehouseZones",
			Path:        "/warehouse-zones",
			Description: "Складские зоны",
			Section:     "Справочники",
		},
		Frontend:  FrontendSpec{Scaffold: true},
		Migration: MigrationSpec{Enabled: boolPointer(false)},
	}

	view, err := BuildObjectView(testConfig(t), spec)
	if err != nil {
		t.Fatalf("BuildObjectView(): %v", err)
	}
	err = validateManualFrontendCollisions(root, []ObjectView{view})
	if err == nil {
		t.Fatal("expected manual frontend collision")
	}
	for _, expected := range []string{
		"frontend file for WarehouseZone",
		`frontend route name "warehouseZones"`,
		`frontend route path "/warehouse-zones"`,
		`frontend locale key "WarehouseZone"`,
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("collision error missing %q: %v", expected, err)
		}
	}
}

func TestBuildObjectViewUsesCurrentModelTagsAndNaturalKeys(t *testing.T) {
	t.Parallel()

	spec := compositeObjectSpec()
	view, err := BuildObjectView(testConfig(t), spec)
	if err != nil {
		t.Fatalf("BuildObjectView(): %v", err)
	}

	if !view.SessionRequired {
		t.Fatal("sessionRequired must default to true")
	}
	if !view.CRUDNotifications {
		t.Fatal("crudNotifications must default to true")
	}
	if !view.CompositeKey {
		t.Fatal("expected composite key")
	}
	if view.ItemRoute != "/catalogue-items/{groupCode}/{itemCode}" {
		t.Fatalf("unexpected item route %q", view.ItemRoute)
	}
	if len(view.CreateFields) != 4 {
		t.Fatalf("expected natural keys in create fields, got %d", len(view.CreateFields))
	}
	if len(view.UpdateFields) != 2 {
		t.Fatalf("expected only non-key writable update fields, got %d", len(view.UpdateFields))
	}

	groupCode := view.Fields[0]
	if !strings.Contains(groupCode.StructTag, `json:"group_code"`) ||
		!strings.Contains(groupCode.StructTag, `primaryKey:"true"`) ||
		!strings.Contains(groupCode.StructTag, `required:"true"`) ||
		!strings.Contains(groupCode.StructTag, `maxLen:"50"`) {
		t.Fatalf("unexpected current model tags: %s", groupCode.StructTag)
	}
	if strings.Contains(groupCode.StructTag, "dbRequired") || strings.Contains(groupCode.StructTag, "dateType") {
		t.Fatalf("legacy tags leaked into model: %s", groupCode.StructTag)
	}
}

func TestUpdateOnlyServiceDoesNotImportModelbind(t *testing.T) {
	t.Parallel()

	spec := compositeObjectSpec()
	spec.CRUD = CRUDSpec{Update: true}

	view, err := BuildObjectView(testConfig(t), spec)
	if err != nil {
		t.Fatalf("BuildObjectView(): %v", err)
	}
	if view.ServiceImports.Modelbind {
		t.Fatal("update-only service must not import modelbind directly")
	}
}

func TestSingleKeyDetailHTTPDoesNotImportModels(t *testing.T) {
	t.Parallel()

	spec := compositeObjectSpec()
	spec.Name = "SingleDetail"
	spec.Table.Name = "single_details"
	spec.Route = "/single-details"
	spec.Keys = spec.Keys[:1]
	spec.Fields = spec.Fields[:1]
	spec.List = nil
	spec.CRUD = CRUDSpec{Detail: true}

	view, err := BuildObjectView(testConfig(t), spec)
	if err != nil {
		t.Fatalf("BuildObjectView(): %v", err)
	}
	if view.HTTPImports.Models {
		t.Fatal("single-key detail route must not import models")
	}
}

func TestRenderFrontendStringKeyUsesStringValue(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	cfg.ServerRoot = t.TempDir()
	cfg.FrontendRoot = filepath.Join(cfg.ServerRoot, "front")
	cfg.BackendEnabled = false
	cfg.FrontendEnabled = true
	cfg.APITestEnabled = false
	cfg.MigrationsEnabled = false

	spec := ObjectSpec{
		Name:            "CodeLookup",
		HumanName:       "code lookup",
		HumanNamePlural: "code lookups",
		Table:           TableSpec{Schema: "public", Name: "code_lookups"},
		Route:           "/code-lookups",
		Keys:            []KeySpec{{Name: "code", PathName: "code", Type: "string"}},
		Fields: []FieldSpec{
			{Name: "code", Type: "string", PrimaryKey: true, Required: true},
			{Name: "name", Type: "string", Required: true},
		},
		CRUD:      CRUDSpec{Create: true, List: true, Detail: true, Update: true, Delete: true},
		Migration: MigrationSpec{Enabled: boolPointer(false)},
	}

	view, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatalf("BuildObjectView(): %v", err)
	}
	if err := NewRenderer(cfg).RenderObject(view); err != nil {
		t.Fatalf("RenderObject(): %v", err)
	}

	api := readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/api/codeLookup.gen.ts"))
	if !strings.Contains(api, "getKeyValue: (key: CodeLookupKey): string") {
		t.Fatalf("string key type was not preserved\n%s", api)
	}
}

func TestFrontendSupportsCustomListProjection(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	cfg.ServerRoot = t.TempDir()
	cfg.FrontendRoot = filepath.Join(cfg.ServerRoot, "front")
	cfg.BackendEnabled = false
	cfg.FrontendEnabled = true
	cfg.APITestEnabled = false
	cfg.MigrationsEnabled = false

	view, err := BuildObjectView(cfg, compositeObjectSpec())
	if err != nil {
		t.Fatalf("BuildObjectView(): %v", err)
	}
	if err := NewRenderer(cfg).RenderObject(view); err != nil {
		t.Fatalf("RenderObject(): %v", err)
	}

	types := readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/types/catalogueItem.gen.ts"))
	listTypes := readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/types/catalogueItemList.gen.ts"))
	api := readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/api/catalogueItem.gen.ts"))
	listSchemas := readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/schemas/catalogueItemList.gen.ts"))
	if strings.Contains(types, "CatalogueItemListDTO") {
		t.Fatalf("detail types must not contain the separate list projection\n%s", types)
	}
	for _, expected := range []string{
		"export interface CatalogueItemListDTO",
		"export interface CatalogueItemList",
	} {
		if !strings.Contains(listTypes, expected) {
			t.Fatalf("list types missing %q\n%s", expected, listTypes)
		}
	}
	for _, expected := range []string{
		`from "@/types/catalogueItemList.gen"`,
		`from "@/schemas/catalogueItemList.gen"`,
		`serviceName: "CatalogueItem"`,
		"const detailPath = (key: CatalogueItemKey): string =>",
		"encodeURIComponent(String(key.group_code))",
		"encodeURIComponent(String(key.item_code))",
		"detail: async (",
		"update: async (",
		"delete: async (",
	} {
		if !strings.Contains(api, expected) {
			t.Fatalf("api missing %q\n%s", expected, api)
		}
	}
	if !strings.Contains(listSchemas, "const CatalogueItemListDTOSchema") || !strings.Contains(listSchemas, "export const catalogueItemListFromDTO") {
		t.Fatalf("custom list schemas were not generated\n%s", listSchemas)
	}
}

func TestFrontendSupportsCustomJSONMappingAndHiddenFields(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	cfg.ServerRoot = t.TempDir()
	cfg.FrontendRoot = filepath.Join(cfg.ServerRoot, "front")
	cfg.BackendEnabled = false
	cfg.FrontendEnabled = true
	cfg.APITestEnabled = false
	cfg.MigrationsEnabled = false

	hidden := false
	spec := compositeObjectSpec()
	spec.List = nil
	spec.Keys = spec.Keys[:1]
	spec.Fields = append([]FieldSpec(nil), spec.Fields[:2]...)
	spec.Fields[0].JSONName = "group"
	spec.Fields[1].JSON = &hidden
	spec.Fields[1].ServerGenerated = true
	spec.Fields[1].PrimaryKey = false
	spec.CRUD = CRUDSpec{Detail: true}

	view, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatalf("BuildObjectView(): %v", err)
	}
	if err := NewRenderer(cfg).RenderObject(view); err != nil {
		t.Fatalf("RenderObject(): %v", err)
	}

	types := readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/types/catalogueItem.gen.ts"))
	if !strings.Contains(types, "group: string;") {
		t.Fatalf("custom JSON name was not emitted\n%s", types)
	}
	if strings.Contains(types, "item_code") {
		t.Fatalf("hidden JSON field leaked into frontend types\n%s", types)
	}
}

func TestRenderFrontendScaffoldMatchesExpectedShape(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	cfg.ServerRoot = t.TempDir()
	cfg.FrontendRoot = filepath.Join(cfg.ServerRoot, "front")
	cfg.BackendEnabled = false
	cfg.FrontendEnabled = true
	cfg.APITestEnabled = false
	cfg.MigrationsEnabled = false
	cfg.RegistriesEnabled = true

	spec := ObjectSpec{
		Name:            "WarehouseZone",
		HumanName:       "warehouse zone",
		HumanNamePlural: "warehouse zones",
		Table:           TableSpec{Schema: "public", Name: "warehouse_zones"},
		Route:           "/warehouse-zones",
		Keys:            []KeySpec{{Name: "id", PathName: "id", Type: "int"}},
		Fields: []FieldSpec{
			{Name: "id", Type: "int", PrimaryKey: true, AutoIncrement: true, ServerGenerated: true},
			{Name: "name", Type: "string", Required: true},
			{Name: "priority", Type: "int", Required: true},
			{Name: "comment", Type: "text", Nullable: true},
			{Name: "is_active", Type: "bool", Required: true, Default: "true"},
			{Name: "created_at", Type: "timestamptz", ServerGenerated: true},
			{Name: "updated_at", Type: "timestamptz", ServerGenerated: true},
		},
		CRUD: CRUDSpec{Create: true, List: true, Detail: true, Update: true, Delete: true},
		ApplicationRoute: ApplicationRouteSpec{
			Enabled:     true,
			Name:        "warehouseZones",
			Path:        "/warehouse-zones",
			Description: "Складские зоны",
			Section:     "Справочники",
		},
		Frontend: FrontendSpec{
			Scaffold: true,
			Form: FrontendFormSpec{
				Fields: []FrontendFormFieldSpec{
					{Field: "name", Autofocus: true},
					{Field: "priority"},
					{Field: "comment"},
					{Field: "is_active", Default: true},
				},
			},
			List: FrontendListSpec{
				Columns: []FrontendListColumnSpec{
					{Field: "id"},
					{Field: "name"},
					{Field: "is_active"},
					{Field: "updated_at"},
				},
			},
		},
		Migration: MigrationSpec{Enabled: boolPointer(false)},
	}

	view, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatalf("BuildObjectView(): %v", err)
	}
	renderer := NewRenderer(cfg)
	if err := renderer.RenderObject(view); err != nil {
		t.Fatalf("RenderObject(): %v", err)
	}
	if err := renderer.RenderFrontendRoutes([]ObjectView{view}); err != nil {
		t.Fatalf("RenderFrontendRoutes(): %v", err)
	}
	if err := renderer.RenderFrontendLocale([]ObjectView{view}); err != nil {
		t.Fatalf("RenderFrontendLocale(): %v", err)
	}

	api := readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/api/warehouseZone.gen.ts"))
	for _, expected := range []string{
		`serviceName: "WarehouseZone"`,
		`getKeyValue: (key: WarehouseZoneKey): number`,
	} {
		if !strings.Contains(api, expected) {
			t.Fatalf("api missing %q\n%s", expected, api)
		}
	}

	schemas := readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/schemas/warehouseZone.gen.ts"))
	for _, expected := range []string{
		"DateStringSchema",
		"IdSchema",
		"IntSchema",
		"RequiredStringSchema",
		"TextSchema",
		"created_at: new Date(parsedDTO.created_at)",
	} {
		if !strings.Contains(schemas, expected) {
			t.Fatalf("schemas missing %q\n%s", expected, schemas)
		}
	}
	if strings.Contains(schemas, "\n\t\tStringSchema,\n") {
		t.Fatalf("schemas destructured an unused substring match for StringSchema\n%s", schemas)
	}

	formContract := readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/forms/warehouseZone.gen.ts"))
	for _, expected := range []string{
		`export type WarehouseZoneFormModel`,
		`export const createWarehouseZoneFormModel`,
		`export const warehouseZoneFormMutationFields`,
	} {
		if !strings.Contains(formContract, expected) {
			t.Fatalf("form contract missing %q\n%s", expected, formContract)
		}
	}

	form := readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/components/warehouseZone/WarehouseZoneForm.vue"))
	for _, expected := range []string{
		`<script setup lang="ts">`,
		`import Checkbox from "primevue/checkbox";`,
		`import InputNumber from "primevue/inputnumber";`,
		`CollectionForm`,
		`FormField`,
		`useCollectionFormModel`,
		`const nullableText =`,
		`const model: WarehouseZoneNew = {`,
		`comment: nullableText(form.value.comment)`,
	} {
		if !strings.Contains(form, expected) {
			t.Fatalf("form missing %q\n%s", expected, form)
		}
	}
	if strings.Contains(form, "const formatDate =") {
		t.Fatalf("form generated an unused date helper\n%s", form)
	}

	collection := readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/collections/warehouseZone.gen.ts"))
	for _, expected := range []string{
		"defineCollection",
		`const formatDate =`,
		`name: "warehouseZoneEdit"`,
		`stateKey: "warehouseZone-grid"`,
		`showCommandShortcuts: false`,
	} {
		if !strings.Contains(collection, expected) {
			t.Fatalf("collection missing %q\n%s", expected, collection)
		}
	}

	list := readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/views/warehouseZone/WarehouseZoneList.vue"))
	for _, expected := range []string{
		"CollectionListPage",
		`warehouseZoneCollection`,
	} {
		if !strings.Contains(list, expected) {
			t.Fatalf("list scaffold missing %q\n%s", expected, list)
		}
	}

	edit := readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/views/warehouseZone/WarehouseZoneEditPage.vue"))
	for _, expected := range []string{
		"useCollectionEditPage",
		"CollectionEditPage",
		`createRouteName: "warehouseZoneCreate"`,
		`listRoute: { name: "warehouseZones" }`,
		`fields: warehouseZoneFormMutationFields`,
	} {
		if !strings.Contains(edit, expected) {
			t.Fatalf("edit page missing %q\n%s", expected, edit)
		}
	}

	routes := readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/router/routeManifest.gen.ts"))
	for _, expected := range []string{
		`name: "warehouseZones"`,
		`name: "warehouseZoneCreate"`,
		`name: "warehouseZoneEdit"`,
		`section: "Справочники"`,
	} {
		if !strings.Contains(routes, expected) {
			t.Fatalf("route manifest missing %q\n%s", expected, routes)
		}
	}

	localeContent := readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/locales/ru.gen.json"))
	var locale map[string]any
	if err := json.Unmarshal([]byte(localeContent), &locale); err != nil {
		t.Fatalf("generated locale is not valid JSON: %v\n%s", err, localeContent)
	}
	if _, exists := locale["WarehouseZone"]; !exists {
		t.Fatalf("generated locale is missing WarehouseZone\n%s", localeContent)
	}

	manualFormPath := filepath.Join(cfg.FrontendRoot, "src/components/warehouseZone/WarehouseZoneForm.vue")
	manualForm := readTestFile(t, manualFormPath) + "\n<!-- manual customization -->\n"
	if err := os.WriteFile(manualFormPath, []byte(manualForm), 0o644); err != nil {
		t.Fatalf("write manual form customization: %v", err)
	}

	if err := renderer.RenderObject(view); err != nil {
		t.Fatalf("RenderObject() after manual customization: %v", err)
	}
	if preserved := readTestFile(t, manualFormPath); !strings.Contains(preserved, "manual customization") {
		t.Fatalf("RenderObject() overwrote the manual form scaffold\n%s", preserved)
	}

	checkCfg := cfg
	checkCfg.Check = true
	if err := NewRenderer(checkCfg).RenderObject(view); err != nil {
		t.Fatalf("check must accept a manually customized scaffold: %v", err)
	}
}

func TestRenderFrontendScaffoldMigratesLegacyGeneratedVueFiles(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	cfg.ServerRoot = t.TempDir()
	cfg.FrontendRoot = filepath.Join(cfg.ServerRoot, "front")
	cfg.BackendEnabled = false
	cfg.FrontendEnabled = true
	cfg.APITestEnabled = false
	cfg.MigrationsEnabled = false

	spec := ObjectSpec{
		Name:            "WarehouseZone",
		HumanName:       "warehouse zone",
		HumanNamePlural: "warehouse zones",
		Table:           TableSpec{Schema: "public", Name: "warehouse_zones"},
		Route:           "/warehouse-zones",
		Keys:            []KeySpec{{Name: "id", PathName: "id", Type: "int"}},
		Fields: []FieldSpec{
			{Name: "id", Type: "int", PrimaryKey: true, AutoIncrement: true, ServerGenerated: true},
			{Name: "name", Type: "string", Required: true},
		},
		CRUD:      CRUDSpec{Create: true, List: true, Detail: true, Update: true, Delete: true},
		Frontend:  FrontendSpec{Scaffold: true},
		Migration: MigrationSpec{Enabled: boolPointer(false)},
	}

	view, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatalf("BuildObjectView(): %v", err)
	}

	pairs := [][2]string{
		{
			filepath.Join(cfg.FrontendRoot, "src", "components", view.Camel, view.Name+"Form.gen.vue"),
			filepath.Join(cfg.FrontendRoot, "src", "components", view.Camel, view.Name+"Form.vue"),
		},
		{
			filepath.Join(cfg.FrontendRoot, "src", "views", view.Camel, view.Name+"EditPage.gen.vue"),
			filepath.Join(cfg.FrontendRoot, "src", "views", view.Camel, view.Name+"EditPage.vue"),
		},
		{
			filepath.Join(cfg.FrontendRoot, "src", "views", view.Camel, view.Name+"List.gen.vue"),
			filepath.Join(cfg.FrontendRoot, "src", "views", view.Camel, view.Name+"List.vue"),
		},
	}
	for _, pair := range pairs {
		legacyPath := pair[0]
		if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
			t.Fatalf("mkdir legacy scaffold dir: %v", err)
		}
		if err := os.WriteFile(legacyPath, []byte("<!-- customized legacy scaffold -->\n"), 0o644); err != nil {
			t.Fatalf("write legacy scaffold: %v", err)
		}
	}

	if err := NewRenderer(cfg).RenderObject(view); err != nil {
		t.Fatalf("RenderObject(): %v", err)
	}
	for _, pair := range pairs {
		legacyPath := pair[0]
		manualPath := pair[1]
		if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
			t.Fatalf("legacy generated scaffold still exists: %s", legacyPath)
		}
		content := readTestFile(t, manualPath)
		if !strings.Contains(content, "customized legacy scaffold") {
			t.Fatalf("legacy scaffold contents were not preserved in %s\n%s", manualPath, content)
		}
	}
}

func TestFrontendCustomListPageImportsSeparateProjection(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	cfg.ServerRoot = t.TempDir()
	cfg.FrontendRoot = filepath.Join(cfg.ServerRoot, "front")
	cfg.BackendEnabled = false
	cfg.FrontendEnabled = true
	cfg.APITestEnabled = false
	cfg.MigrationsEnabled = false

	spec := ObjectSpec{
		Name:            "WarehouseZone",
		HumanName:       "warehouse zone",
		HumanNamePlural: "warehouse zones",
		Table:           TableSpec{Schema: "public", Name: "warehouse_zones"},
		List: &ListSpec{
			Model: "WarehouseZoneList",
			Table: TableSpec{Schema: "public", Name: "warehouse_zones_list"},
			Fields: []FieldSpec{
				{Name: "id", Type: "int", PrimaryKey: true, ServerGenerated: true},
				{Name: "descr", Type: "text", Required: true},
				{Name: "is_active", Type: "bool", Required: true},
			},
		},
		Route: "/warehouse-zones",
		Keys:  []KeySpec{{Name: "id", PathName: "id", Type: "int"}},
		Fields: []FieldSpec{
			{Name: "id", Type: "int", PrimaryKey: true, AutoIncrement: true, ServerGenerated: true},
			{Name: "name", Type: "string", Required: true},
			{Name: "is_active", Type: "bool", Required: true, Default: "true"},
		},
		CRUD: CRUDSpec{Create: true, List: true, Detail: true, Update: true, Delete: true},
		ApplicationRoute: ApplicationRouteSpec{
			Enabled:     true,
			Name:        "warehouseZones",
			Path:        "/warehouse-zones",
			Description: "Складские зоны",
			Section:     "Справочники",
		},
		Frontend: FrontendSpec{
			Scaffold: true,
			List: FrontendListSpec{
				Columns: []FrontendListColumnSpec{
					{Field: "id"},
					{Field: "descr"},
					{Field: "is_active"},
				},
			},
		},
		Migration: MigrationSpec{Enabled: boolPointer(false)},
	}

	view, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatalf("BuildObjectView(): %v", err)
	}
	if err := NewRenderer(cfg).RenderObject(view); err != nil {
		t.Fatalf("RenderObject(): %v", err)
	}

	collection := readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/collections/warehouseZone.gen.ts"))
	if !strings.Contains(collection, `import type { WarehouseZoneList } from "@/types/warehouseZoneList.gen";`) {
		t.Fatalf("custom collection does not import its separate projection\n%s", collection)
	}
	if !strings.Contains(collection, `WarehouseZoneKey,`) {
		t.Fatalf("custom collection does not import the base key\n%s", collection)
	}
	if strings.Contains(collection, `WarehouseZoneList } from "@/types/warehouseZone.gen"`) {
		t.Fatalf("custom list projection was incorrectly imported from the base type module\n%s", collection)
	}
}

func TestFrontendInlineListGeneratesNativeInlineEditing(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	cfg.ServerRoot = t.TempDir()
	cfg.FrontendRoot = filepath.Join(cfg.ServerRoot, "front")
	cfg.BackendEnabled = false
	cfg.FrontendEnabled = true
	cfg.APITestEnabled = false
	cfg.MigrationsEnabled = false

	formEnabled := false
	spec := ObjectSpec{
		Name:            "Customer",
		HumanName:       "customer",
		HumanNamePlural: "customers",
		Table:           TableSpec{Schema: "public", Name: "customers"},
		Route:           "/customers",
		Keys:            []KeySpec{{Name: "id", PathName: "id", Type: "int"}},
		Fields: []FieldSpec{
			{Name: "id", Type: "int", PrimaryKey: true, AutoIncrement: true, ServerGenerated: true},
			{Name: "inn", Type: "string", Required: true, MaxLen: "12"},
			{Name: "kpp", Type: "string", Nullable: true, MaxLen: "10"},
			{Name: "name", Type: "text", Required: true},
			{Name: "ref_1c", Type: "jsonb", Nullable: true},
			{Name: "active", Type: "bool", Required: true, Default: "true"},
		},
		CRUD: CRUDSpec{Create: true, List: true, Detail: true, Update: true, Delete: true},
		ApplicationRoute: ApplicationRouteSpec{
			Enabled:     true,
			Name:        "customers",
			Path:        "/customers",
			Description: "Customers",
			Section:     "References",
		},
		Frontend: FrontendSpec{
			Scaffold: true,
			Form: FrontendFormSpec{
				Enabled: &formEnabled,
			},
			List: FrontendListSpec{
				EditMode: "inline",
				Columns: []FrontendListColumnSpec{
					{Field: "id"},
					{Field: "inn"},
					{Field: "kpp"},
					{Field: "name"},
					{Field: "ref_1c"},
					{Field: "active"},
				},
			},
		},
		Migration: MigrationSpec{Enabled: boolPointer(false)},
	}

	view, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatalf("BuildObjectView(): %v", err)
	}
	if err := NewRenderer(cfg).RenderObject(view); err != nil {
		t.Fatalf("RenderObject(): %v", err)
	}

	list := readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/collections/customer.gen.ts"))
	for _, expected := range []string{
		`editMode: "inline"`,
		`const createRow = (): ListModel => ({`,
		`const createModel = (row: ListModel): CreateModel => ({`,
		`id: 0,`,
		`inn: "",`,
		`kpp: null,`,
		`name: "",`,
		`ref_1c: null,`,
		`active: true,`,
		`field: "inn",`,
		`editable: true,`,
		`{ name: "create" },`,
		`{ name: "edit" },`,
		`createRow,`,
		`createModel,`,
	} {
		if !strings.Contains(list, expected) {
			t.Fatalf("inline list missing %q\n%s", expected, list)
		}
	}
	if strings.Contains(list, `{ name: "copy" }`) {
		t.Fatalf("inline list must not emit the page-copy command\n%s", list)
	}

	idColumnStart := strings.Index(list, `field: "id",`)
	innColumnStart := strings.Index(list, `field: "inn",`)
	if idColumnStart < 0 || innColumnStart < 0 || innColumnStart <= idColumnStart {
		t.Fatalf("could not locate id/inn columns\n%s", list)
	}
	if strings.Contains(list[idColumnStart:innColumnStart], "editable: true") {
		t.Fatalf("server-generated key must not be editable\n%s", list[idColumnStart:innColumnStart])
	}
	refColumnStart := strings.Index(list, `field: "ref_1c",`)
	activeColumnStart := strings.Index(list, `field: "active",`)
	if refColumnStart < 0 || activeColumnStart < 0 || activeColumnStart <= refColumnStart {
		t.Fatalf("could not locate ref_1c/active columns\n%s", list)
	}
	if strings.Contains(list[refColumnStart:activeColumnStart], "editable: true") {
		t.Fatalf("jsonb field must not use the generated scalar inline editor\n%s", list[refColumnStart:activeColumnStart])
	}
}

func TestFrontendInlineListRequiresCreateFieldsWithoutDefaultsToBeEditable(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	cfg.BackendEnabled = false
	cfg.FrontendEnabled = true
	cfg.APITestEnabled = false
	cfg.MigrationsEnabled = false

	formEnabled := false
	spec := ObjectSpec{
		Name:  "Customer",
		Table: TableSpec{Schema: "public", Name: "customers"},
		Route: "/customers",
		Keys:  []KeySpec{{Name: "id", Type: "int"}},
		Fields: []FieldSpec{
			{Name: "id", Type: "int", PrimaryKey: true, AutoIncrement: true, ServerGenerated: true},
			{Name: "name", Type: "string", Required: true},
			{Name: "active", Type: "bool", Required: true},
		},
		CRUD: CRUDSpec{Create: true, List: true, Detail: true, Update: true, Delete: true},
		ApplicationRoute: ApplicationRouteSpec{
			Enabled:     true,
			Name:        "customers",
			Path:        "/customers",
			Description: "Customers",
			Section:     "References",
		},
		Frontend: FrontendSpec{
			Scaffold: true,
			Form:     FrontendFormSpec{Enabled: &formEnabled},
			List: FrontendListSpec{
				EditMode: "inline",
				Columns: []FrontendListColumnSpec{
					{Field: "id"},
					{Field: "name"},
				},
			},
		},
	}

	_, err := BuildObjectView(cfg, spec)
	if err == nil || !strings.Contains(err.Error(), "inline creation requires writable create field active to be editable, nullable, or have a default") {
		t.Fatalf("expected missing inline create field error, got %v", err)
	}
}

func TestFrontendPartialCRUDDoesNotEnableUnsafeFormScaffold(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	cfg.BackendEnabled = false
	cfg.FrontendEnabled = true
	cfg.APITestEnabled = false
	cfg.MigrationsEnabled = false

	spec := ObjectSpec{
		Name:  "ImportRequest",
		Table: TableSpec{Schema: "public", Name: "import_requests"},
		Route: "/import-requests",
		Keys:  []KeySpec{{Name: "id", PathName: "id", Type: "int"}},
		Fields: []FieldSpec{
			{Name: "id", Type: "int", PrimaryKey: true, AutoIncrement: true, ServerGenerated: true},
			{Name: "source", Type: "string", Required: true},
		},
		CRUD: CRUDSpec{Create: true},
		ApplicationRoute: ApplicationRouteSpec{
			Enabled:     true,
			Name:        "importRequests",
			Path:        "/import-requests",
			Description: "Импорт",
			Section:     "Формы",
		},
		Frontend:  FrontendSpec{Scaffold: true},
		Migration: MigrationSpec{Enabled: boolPointer(false)},
	}

	view, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatalf("BuildObjectView(): %v", err)
	}
	if view.Frontend.Form.Enabled {
		t.Fatal("partial CRUD unexpectedly enabled a generated edit form")
	}
}

func TestRenderEmptyFrontendRouteRegistryHasNoUnusedHelper(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	cfg.ServerRoot = t.TempDir()
	cfg.FrontendRoot = filepath.Join(cfg.ServerRoot, "front")
	cfg.BackendEnabled = false
	cfg.FrontendEnabled = true
	cfg.APITestEnabled = false
	cfg.MigrationsEnabled = false
	cfg.RegistriesEnabled = true

	if err := NewRenderer(cfg).RenderFrontendRoutes(nil); err != nil {
		t.Fatalf("RenderFrontendRoutes(): %v", err)
	}

	routes := readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/router/routeManifest.gen.ts"))
	if strings.Contains(routes, "defineGeneratedRoute") {
		t.Fatalf("empty route registry contains an unused helper\n%s", routes)
	}
	if !strings.Contains(routes, "export const generatedRouteManifest: GeneratedRouteManifestEntry[] = [\n];") {
		t.Fatalf("empty route registry has an unexpected shape\n%s", routes)
	}
}

func TestFrontendReadOnlyAPIUsesNeverMutationModels(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	cfg.ServerRoot = t.TempDir()
	cfg.FrontendRoot = filepath.Join(cfg.ServerRoot, "front")
	cfg.BackendEnabled = false
	cfg.FrontendEnabled = true
	cfg.APITestEnabled = false
	cfg.MigrationsEnabled = false

	spec := ObjectSpec{
		Name:  "AuditEntry",
		Table: TableSpec{Schema: "public", Name: "audit_entries"},
		Route: "/audit-entries",
		Keys:  []KeySpec{{Name: "id", PathName: "id", Type: "int"}},
		Fields: []FieldSpec{
			{Name: "id", Type: "int", PrimaryKey: true, ServerGenerated: true},
			{Name: "message", Type: "text", Required: true},
		},
		CRUD:      CRUDSpec{List: true, Detail: true},
		Migration: MigrationSpec{Enabled: boolPointer(false)},
	}

	view, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatalf("BuildObjectView(): %v", err)
	}
	if err := NewRenderer(cfg).RenderObject(view); err != nil {
		t.Fatalf("RenderObject(): %v", err)
	}
	api := readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/api/auditEntry.gen.ts"))
	if !strings.Contains(api, "\tnever,\n\tnever\n>(") {
		t.Fatalf("read-only API did not use never mutation models\n%s", api)
	}
}

func TestFrontendRejectsUnsafeJSONPropertyName(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	cfg.FrontendEnabled = true
	spec := compositeObjectSpec()
	spec.List = nil
	spec.Keys = spec.Keys[:1]
	spec.Fields = spec.Fields[:1]
	spec.Fields[0].JSONName = "invalid-name"
	spec.CRUD = CRUDSpec{Detail: true}

	_, err := BuildObjectView(cfg, spec)
	if err == nil || !strings.Contains(err.Error(), "valid TypeScript property identifier") {
		t.Fatalf("expected unsafe frontend property error, got %v", err)
	}
}

func TestRenderCompositeBackend(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	cfg.ServerRoot = t.TempDir()
	cfg.MigrationsEnabled = false
	cfg.APITestEnabled = false
	cfg.FrontendEnabled = false

	view, err := BuildObjectView(cfg, compositeObjectSpec())
	if err != nil {
		t.Fatalf("BuildObjectView(): %v", err)
	}

	renderer := NewRenderer(cfg)
	if err := renderer.RenderObject(view); err != nil {
		t.Fatalf("RenderObject(): %v", err)
	}

	service := readTestFile(t, filepath.Join(cfg.ServerRoot, "internal/services/catalogueItem.gen.go"))
	for _, expected := range []string{
		"webapp.WithCRUDNotifications()",
		"key models.CatalogueItemKey",
		"input.Keys.GroupCode == \"\"",
		"[]types.DBModel{",
		"wmodels.CollectionResponse[*models.CatalogueItemList]",
	} {
		if !strings.Contains(service, expected) {
			t.Fatalf("service missing %q\n%s", expected, service)
		}
	}

	httpAPI := readTestFile(t, filepath.Join(cfg.ServerRoot, "internal/httpapi/catalogueItem.gen.go"))
	for _, expected := range []string{
		`"/catalogue-items/{groupCode}/{itemCode}"`,
		"func catalogueItemKeyFromRequest",
		"modelbind.DecodeRequestInput[*models.CatalogueItem]",
	} {
		if !strings.Contains(httpAPI, expected) {
			t.Fatalf("http api missing %q\n%s", expected, httpAPI)
		}
	}

	model := readTestFile(t, filepath.Join(cfg.ServerRoot, "internal/models/catalogueItem.gen.go"))
	if !strings.Contains(model, "type CatalogueItemList struct") || !strings.Contains(model, `const catalogueItemListRelation = "public.catalogue_items_list"`) {
		t.Fatalf("custom list model was not generated\n%s", model)
	}
}

func TestRenderManualServiceMethodsKeepsCRUDContract(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	cfg.ServerRoot = t.TempDir()
	cfg.MigrationsEnabled = false
	cfg.APITestEnabled = false
	cfg.FrontendEnabled = false

	spec := compositeObjectSpec()
	spec.Service.ManualMethods = []string{"create", "update"}
	view, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatalf("BuildObjectView(): %v", err)
	}
	if err := NewRenderer(cfg).RenderObject(view); err != nil {
		t.Fatalf("RenderObject(): %v", err)
	}

	service := readTestFile(t, filepath.Join(cfg.ServerRoot, "internal/services/catalogueItem.gen.go"))
	for _, unexpected := range []string{
		"func (s *CatalogueItemService) Create(",
		"func (s *CatalogueItemService) Update(",
	} {
		if strings.Contains(service, unexpected) {
			t.Fatalf("manual service method was generated: %q\n%s", unexpected, service)
		}
	}
	for _, expected := range []string{
		"func (s *CatalogueItemService) List(",
		"func (s *CatalogueItemService) Detail(",
		"func (s *CatalogueItemService) Delete(",
	} {
		if !strings.Contains(service, expected) {
			t.Fatalf("generated service method missing %q\n%s", expected, service)
		}
	}

	httpAPI := readTestFile(t, filepath.Join(cfg.ServerRoot, "internal/httpapi/catalogueItem.gen.go"))
	for _, expected := range []string{
		"api.POST(",
		"api.PATCH(",
		`webapp.WithService("CatalogueItem", "Create")`,
		`webapp.WithService("CatalogueItem", "Update")`,
	} {
		if !strings.Contains(httpAPI, expected) {
			t.Fatalf("manual service ownership changed the HTTP contract; missing %q\n%s", expected, httpAPI)
		}
	}

	actions := make(map[string]bool, len(view.PermissionRows))
	for _, permission := range view.PermissionRows {
		actions[permission.Action] = true
	}
	if !actions["create"] || !actions["update"] {
		t.Fatalf("manual service ownership removed CRUD permissions: %+v", view.PermissionRows)
	}
}

func TestRenderFullyManualServiceOmitsGeneratedMethodImports(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	cfg.ServerRoot = t.TempDir()
	cfg.MigrationsEnabled = false
	cfg.APITestEnabled = false
	cfg.FrontendEnabled = false

	spec := compositeObjectSpec()
	spec.Service.ManualMethods = []string{"create", "list", "detail", "update", "delete"}
	view, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatalf("BuildObjectView(): %v", err)
	}
	if err := NewRenderer(cfg).RenderObject(view); err != nil {
		t.Fatalf("RenderObject(): %v", err)
	}

	service := readTestFile(t, filepath.Join(cfg.ServerRoot, "internal/services/catalogueItem.gen.go"))
	for _, unexpected := range []string{
		`"context"`,
		`"errors"`,
		`"fmt"`,
		`"github.com/dronm/modelbind"`,
		`"github.com/dronm/modelbind/types"`,
		`"example.com/project/internal/models"`,
		`wmodels "github.com/dronm/webapp/models"`,
	} {
		if strings.Contains(service, unexpected) {
			t.Fatalf("fully manual service retained unused generated-method import %q\n%s", unexpected, service)
		}
	}
	if !strings.Contains(service, "func RegisterCatalogueItemService()") ||
		!strings.Contains(service, "func (s *CatalogueItemService) requireDB() error") {
		t.Fatalf("fully manual service lost generated infrastructure\n%s", service)
	}
}

func TestRenderCompositeNumericKeyBinder(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	cfg.ServerRoot = t.TempDir()
	cfg.MigrationsEnabled = false
	cfg.APITestEnabled = false
	cfg.FrontendEnabled = false

	spec := compositeObjectSpec()
	spec.Name = "NumericCatalogueItem"
	spec.Route = "/numeric-catalogue-items"
	spec.Table.Name = "numeric_catalogue_items"
	spec.List = nil
	spec.Keys[1].Type = "int"
	spec.Fields[1].Type = "int"

	view, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatalf("BuildObjectView(): %v", err)
	}
	if err := NewRenderer(cfg).RenderObject(view); err != nil {
		t.Fatalf("RenderObject(): %v", err)
	}

	httpAPI := readTestFile(t, filepath.Join(cfg.ServerRoot, "internal/httpapi/numericCatalogueItem.gen.go"))
	for _, expected := range []string{
		`"strconv"`,
		"strconv.Atoi",
		"key.ItemCode = itemCode",
	} {
		if !strings.Contains(httpAPI, expected) {
			t.Fatalf("http api missing %q\n%s", expected, httpAPI)
		}
	}
}

func TestRenderObjectRejectsManualBackendCollision(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	cfg.ServerRoot = t.TempDir()
	cfg.MigrationsEnabled = false
	cfg.APITestEnabled = false
	cfg.FrontendEnabled = false

	manualPath := filepath.Join(cfg.ServerRoot, "internal", "models", "catalogueItem.go")
	if err := os.MkdirAll(filepath.Dir(manualPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(): %v", err)
	}
	if err := os.WriteFile(manualPath, []byte("package models\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}

	view, err := BuildObjectView(cfg, compositeObjectSpec())
	if err != nil {
		t.Fatalf("BuildObjectView(): %v", err)
	}

	err = NewRenderer(cfg).RenderObject(view)
	if err == nil || !strings.Contains(err.Error(), "hand-written backend files already exist") {
		t.Fatalf("expected manual collision error, got %v", err)
	}
}

func TestRenderMigrationIncludesAccessAndMenuMetadata(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	cfg.ServerRoot = t.TempDir()
	cfg.MigrationsDir = filepath.Join(cfg.ServerRoot, "migrations")
	cfg.MigrationsEnabled = true
	cfg.MigrationCreateMode = "internal"
	cfg.MigrationSequenceWidth = 6
	cfg.APITestEnabled = false
	cfg.FrontendEnabled = false

	if err := os.MkdirAll(cfg.MigrationsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(): %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.MigrationsDir, "000107_previous.up.sql"), nil, 0o644); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}

	spec := compositeObjectSpec()
	spec.Table.Comment = "Catalogue values"
	spec.Fields[2].Comment = "Visible item name"
	spec.ApplicationRoute = ApplicationRouteSpec{
		Enabled:     true,
		Name:        "catalogueItems",
		Path:        "/catalogue-items",
		Description: "Элементы справочников",
		Section:     "Справочники",
	}
	spec.Menu = MenuSpec{
		Enabled:   true,
		Caption:   "Элементы справочников",
		SortOrder: 20,
		Parent: &MenuParentSpec{
			Caption:   "Справочники",
			SortOrder: 50,
		},
	}
	spec.Migration = MigrationSpec{
		UpdatedAtTrigger: "updated_at",
		Indexes: []IndexSpec{{
			Name:    "catalogue_items_name_idx",
			Columns: []string{"lower(name)"},
		}},
	}

	view, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatalf("BuildObjectView(): %v", err)
	}
	if err := NewRenderer(cfg).RenderMigration(view); err != nil {
		t.Fatalf("RenderMigration(): %v", err)
	}

	upPath := filepath.Join(cfg.MigrationsDir, "000108_create_catalogue_items.up.sql")
	up := readTestFile(t, upPath)
	for _, expected := range []string{
		"PRIMARY KEY (group_code, item_code)",
		"CREATE INDEX catalogue_items_name_idx",
		"catalogueItem.create",
		"INSERT INTO public.role_permissions",
		"INSERT INTO public.application_routes",
		"INSERT INTO public.main_menus",
		"UPDATE public.main_menus AS existing",
		"catalogue_items_set_updated_at",
		"COMMENT ON TABLE public.catalogue_items IS 'Catalogue values'",
		"COMMENT ON COLUMN public.catalogue_items.name IS 'Visible item name'",
	} {
		if !strings.Contains(up, expected) {
			t.Fatalf("migration missing %q\n%s", expected, up)
		}
	}
}

func TestRenderMigrationCheckComparesExistingSQL(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	cfg.ServerRoot = t.TempDir()
	cfg.MigrationsDir = filepath.Join(cfg.ServerRoot, "migrations")
	cfg.MigrationsEnabled = true
	cfg.MigrationsOverwrite = true
	cfg.MigrationCreateMode = "internal"
	cfg.MigrationSequenceWidth = 6
	cfg.APITestEnabled = false
	cfg.FrontendEnabled = false

	if err := os.MkdirAll(cfg.MigrationsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(): %v", err)
	}

	view, err := BuildObjectView(cfg, compositeObjectSpec())
	if err != nil {
		t.Fatalf("BuildObjectView(): %v", err)
	}
	renderer := NewRenderer(cfg)
	if err := renderer.RenderMigration(view); err != nil {
		t.Fatalf("RenderMigration(): %v", err)
	}

	pair, exists, err := renderer.findMigrationPair(view.Migration.Name)
	if err != nil || !exists {
		t.Fatalf("findMigrationPair(): exists=%v err=%v", exists, err)
	}
	if err := os.WriteFile(pair.UpPath, []byte("outdated\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}

	cfg.Check = true
	cfg.MigrationsOverwrite = false
	err = NewRenderer(cfg).RenderMigration(view)
	if err == nil || !strings.Contains(err.Error(), "is outdated") {
		t.Fatalf("expected migration check mismatch, got %v", err)
	}
}

func TestFindMigrationPairRejectsIncompletePair(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	cfg.MigrationsDir = t.TempDir()
	renderer := NewRenderer(cfg)
	upPath := filepath.Join(cfg.MigrationsDir, "000001_create_examples.up.sql")
	if err := os.WriteFile(upPath, nil, 0o644); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}

	_, _, err := renderer.findMigrationPair("create_examples")
	if err == nil || !strings.Contains(err.Error(), "incomplete migration") {
		t.Fatalf("expected incomplete migration error, got %v", err)
	}
}

func compositeObjectSpec() ObjectSpec {
	return ObjectSpec{
		Name:            "CatalogueItem",
		HumanName:       "catalogue item",
		HumanNamePlural: "catalogue items",
		Table: TableSpec{
			Schema: "public",
			Name:   "catalogue_items",
		},
		List: &ListSpec{
			Model: "CatalogueItemList",
			Table: TableSpec{Schema: "public", Name: "catalogue_items_list"},
			Fields: []FieldSpec{
				{Name: "group_code", Type: "string", PrimaryKey: true, Required: true},
				{Name: "item_code", Type: "string", PrimaryKey: true, Required: true},
				{Name: "name", Type: "string", Required: true},
			},
		},
		Route: "/catalogue-items",
		Keys: []KeySpec{
			{Name: "group_code", PathName: "groupCode", Type: "string"},
			{Name: "item_code", PathName: "itemCode", Type: "string"},
		},
		Fields: []FieldSpec{
			{Name: "group_code", Type: "string", PrimaryKey: true, Required: true, MaxLen: "50"},
			{Name: "item_code", Type: "string", PrimaryKey: true, Required: true, MaxLen: "50"},
			{Name: "name", Type: "string", Required: true, MaxLen: "250"},
			{Name: "sort_order", Type: "int", Required: true, Default: "0"},
			{Name: "updated_at", Type: "timestamptz", ServerGenerated: true, DBRequired: true, Default: "now()"},
		},
		CRUD: CRUDSpec{Create: true, List: true, Detail: true, Update: true, Delete: true},
		Permissions: PermissionSpec{
			GrantRoles: []string{"admin"},
		},
	}
}

func boolPointer(value bool) *bool {
	return &value
}

func testConfig(t *testing.T) Config {
	t.Helper()

	return Config{
		TemplateDir:            "",
		GoModule:               "example.com/project",
		BackendEnabled:         true,
		APITestEnabled:         true,
		RegistriesEnabled:      true,
		MigrationsEnabled:      false,
		MigrationCreateMode:    "internal",
		MigrationCreateExt:     "sql",
		MigrationSequenceWidth: 6,
		GoJSONTagName:          "json",
		GoFilterTagName:        "f",
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	return string(content)
}

func TestDefaultConfigUsesEmbeddedTemplates(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig(t.TempDir())
	if cfg.TemplateDir != "" {
		t.Fatalf("expected embedded templates by default, got template dir %q", cfg.TemplateDir)
	}
}

func TestConfigFileLoadsRegisterSettings(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "codegen.yaml")
	content := `
registers:
  enabled: false
  schemaDir: ./db/registers
  businessTimezone: Asia/Yekaterinburg
  runtimeVersion: 1
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}
	fileConfig, err := readConfigFile(path)
	if err != nil {
		t.Fatalf("readConfigFile(): %v", err)
	}
	cfg := DefaultConfig(t.TempDir())
	applyConfigFile(&cfg, fileConfig)
	if cfg.RegisterSchemaDir != "./db/registers" {
		t.Fatalf("unexpected register schema dir %q", cfg.RegisterSchemaDir)
	}
	if cfg.RegisterBusinessTZ != "Asia/Yekaterinburg" {
		t.Fatalf("unexpected register timezone %q", cfg.RegisterBusinessTZ)
	}
	if cfg.RegisterRuntimeVersion != 1 || cfg.RegistersEnabled {
		t.Fatalf("unexpected register configuration: %+v", cfg)
	}
}

func TestEmbeddedTemplatesAreAvailable(t *testing.T) {
	t.Parallel()

	renderer := NewRenderer(testConfig(t))
	content, label, err := renderer.readTemplate(filepath.Join("go", "model.go.tmpl"))
	if err != nil {
		t.Fatalf("readTemplate(): %v", err)
	}
	if !strings.Contains(label, "templates/go/model.go.tmpl") {
		t.Fatalf("unexpected embedded template label %q", label)
	}
	if !strings.Contains(string(content), "package models") {
		t.Fatalf("unexpected embedded model template content")
	}
	runtime, label, err := renderer.readTemplate(filepath.Join("register", "runtime", "v1", "bootstrap.up.sql.tmpl"))
	if err != nil {
		t.Fatalf("readTemplate(register runtime): %v", err)
	}
	if !strings.Contains(label, "templates/register/runtime/v1/bootstrap.up.sql.tmpl") {
		t.Fatalf("unexpected embedded register runtime label %q", label)
	}
	if !strings.Contains(string(runtime), "register_runtime_version") {
		t.Fatalf("unexpected embedded register runtime template content")
	}
}

func TestLoadRegistersSupportsAccumulationRegister(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content := `
name: Materials
comment: Material stock by construction site.
kind: accumulation
period: month
dimensions:
  - name: construction_site_id
    type: int
    references:
      table: construction_sites
      column: id
  - name: material_id
    type: int
    references:
      table: materials
      column: id
resources:
  - name: quant
    type: numeric
    sqlType: numeric(19, 4)
`
	if err := os.WriteFile(filepath.Join(dir, "materials.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}

	registers, err := LoadRegisters(dir)
	if err != nil {
		t.Fatalf("LoadRegisters(): %v", err)
	}
	if len(registers) != 1 {
		t.Fatalf("expected one register, got %d", len(registers))
	}
	view, err := BuildRegisterView(registers[0])
	if err != nil {
		t.Fatalf("BuildRegisterView(): %v", err)
	}
	if view.ActionRelation != "public.ra_materials" {
		t.Fatalf("unexpected action relation %q", view.ActionRelation)
	}
	if view.Resources[0].SQLType != "numeric(19, 4)" {
		t.Fatalf("unexpected resource SQL type %q", view.Resources[0].SQLType)
	}
	if view.Dimensions[0].FilterName != "construction_site_ids" {
		t.Fatalf("unexpected dimension filter %q", view.Dimensions[0].FilterName)
	}
}

func TestLoadRegistersAllowsMissingDefaultDirectory(t *testing.T) {
	t.Parallel()

	registers, err := LoadRegisters(filepath.Join(t.TempDir(), "registers"))
	if err != nil {
		t.Fatalf("LoadRegisters(): %v", err)
	}
	if len(registers) != 0 {
		t.Fatalf("expected no registers, got %d", len(registers))
	}
}

func TestRegisterValidationRejectsUnsupportedAndDuplicateFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		edit func(*RegisterSpec)
		want string
	}{
		{
			name: "period",
			edit: func(spec *RegisterSpec) { spec.Period = "day" },
			want: "supports month",
		},
		{
			name: "nullable-like-resource",
			edit: func(spec *RegisterSpec) { spec.Resources[0].Type = "string" },
			want: "resources support numeric, int, and bigint",
		},
		{
			name: "duplicate",
			edit: func(spec *RegisterSpec) { spec.Resources[0].Name = spec.Dimensions[0].Name },
			want: "duplicates dimension",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			spec := materialsRegisterSpec()
			test.edit(&spec)
			err := spec.Validate()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected error containing %q, got %v", test.want, err)
			}
		})
	}
}

func TestRenderRegistersCreatesRuntimeMigrationFirst(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := testConfig(t)
	cfg.ServerRoot = root
	cfg.MigrationsDir = filepath.Join(root, "migrations")
	cfg.MigrationsEnabled = true
	cfg.BackendEnabled = true
	cfg.RegistersEnabled = true
	cfg.RegisterBusinessTZ = "Asia/Yekaterinburg"
	cfg.RegisterRuntimeVersion = 1

	view, err := BuildRegisterView(materialsRegisterSpec())
	if err != nil {
		t.Fatalf("BuildRegisterView(): %v", err)
	}
	view.RuntimeVersion = cfg.RegisterRuntimeVersion
	if err := NewRenderer(cfg).RenderRegisters([]RegisterView{view}); err != nil {
		t.Fatalf("RenderRegisters(): %v", err)
	}

	common := readTestFile(t, filepath.Join(cfg.MigrationsDir, "000001_register_common_v1.up.sql"))
	for _, expected := range []string{
		"codegen:register-runtime version=1",
		"CREATE TABLE public.register_settings",
		"VALUES (1, 'Asia/Yekaterinburg')",
		"register_runtime_version()",
	} {
		if !strings.Contains(common, expected) {
			t.Fatalf("common register migration missing %q\n%s", expected, common)
		}
	}

	registerSQL := readTestFile(t, filepath.Join(cfg.MigrationsDir, "000002_create_materials_register.up.sql"))
	for _, expected := range []string{
		"CREATE TABLE public.ra_materials",
		"CREATE TABLE public.rg_materials_period",
		"CREATE TABLE public.rg_materials_current",
		"public.rg_materials_apply_delta",
		"public.rg_materials_balance",
		"public.rg_materials_summary",
		"action.effective_at >= in_from",
		"action.effective_at < in_to",
	} {
		if !strings.Contains(registerSQL, expected) {
			t.Fatalf("register migration missing %q\n%s", expected, registerSQL)
		}
	}

	registerGo := readTestFile(t, filepath.Join(root, "internal", "registers", "materials.gen.go"))
	for _, expected := range []string{
		"type MaterialsAction struct",
		"func AddMaterialsAction(",
		"func MaterialsBalanceAt(",
		"func SummarizeMaterials(",
	} {
		if !strings.Contains(registerGo, expected) {
			t.Fatalf("generated register Go helper missing %q\n%s", expected, registerGo)
		}
	}

	registryGo := readTestFile(t, filepath.Join(root, "internal", "registers", "registry.gen.go"))
	if !strings.Contains(registryGo, `namespace := "register:" + registerName + ":" + recorderType`) {
		t.Fatalf("register registry does not namespace recorder locks\n%s", registryGo)
	}
}

func TestRenderRegisterRuntimeCheckDetectsChanges(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := testConfig(t)
	cfg.MigrationsDir = filepath.Join(root, "migrations")
	cfg.MigrationsEnabled = true
	cfg.RegisterBusinessTZ = "UTC"
	cfg.RegisterRuntimeVersion = 1
	renderer := NewRenderer(cfg)
	runtime := BuildRegisterRuntimeView(cfg)
	if err := renderer.RenderRegisterRuntimeMigration(runtime); err != nil {
		t.Fatalf("RenderRegisterRuntimeMigration(): %v", err)
	}
	pair, exists, err := renderer.findMigrationPair(runtime.MigrationName)
	if err != nil || !exists {
		t.Fatalf("findMigrationPair(): exists=%v err=%v", exists, err)
	}
	if err := os.WriteFile(pair.UpPath, []byte("changed\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}

	cfg.Check = true
	err = NewRenderer(cfg).RenderRegisterRuntimeMigration(runtime)
	if err == nil || !strings.Contains(err.Error(), "is outdated") {
		t.Fatalf("expected outdated runtime migration error, got %v", err)
	}
}

func TestRenderRegisterSupportsStringDimensionAndMultipleResources(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := testConfig(t)
	cfg.ServerRoot = root
	cfg.MigrationsDir = filepath.Join(root, "migrations")
	cfg.MigrationsEnabled = true

	spec := RegisterSpec{
		Name:   "AccountTurnovers",
		Schema: "accounting",
		Dimensions: []RegisterDimensionSpec{
			{Name: "account_code", Type: "string", SQLType: "varchar(20)"},
		},
		Resources: []RegisterResourceSpec{
			{Name: "amount", Type: "numeric", SQLType: "numeric(15, 2)"},
			{Name: "entries", Type: "bigint"},
		},
	}
	view, err := BuildRegisterView(spec)
	if err != nil {
		t.Fatalf("BuildRegisterView(): %v", err)
	}
	view.RuntimeVersion = 1
	renderer := NewRenderer(cfg)
	if err := renderer.RenderRegisterGo(view); err != nil {
		t.Fatalf("RenderRegisterGo(): %v", err)
	}
	if err := renderer.RenderRegisterMigration(view); err != nil {
		t.Fatalf("RenderRegisterMigration(): %v", err)
	}

	generatedGo := readTestFile(t, filepath.Join(root, "internal", "registers", "accountTurnovers.gen.go"))
	for _, expected := range []string{
		"AccountCode string",
		"Amount float64",
		"Entries int64",
		"action.Amount == 0 && action.Entries == 0",
	} {
		if !strings.Contains(generatedGo, expected) {
			t.Fatalf("multi-resource Go output missing %q\n%s", expected, generatedGo)
		}
	}

	pair, exists, err := renderer.findMigrationPair(view.MigrationName)
	if err != nil || !exists {
		t.Fatalf("findMigrationPair(): exists=%v err=%v", exists, err)
	}
	generatedSQL := readTestFile(t, pair.UpPath)
	for _, expected := range []string{
		"amount <> 0",
		"OR entries <> 0",
		"amount_opening numeric(15, 2)",
		"entries_closing bigint",
	} {
		if !strings.Contains(generatedSQL, expected) {
			t.Fatalf("multi-resource SQL output missing %q\n%s", expected, generatedSQL)
		}
	}
}

func materialsRegisterSpec() RegisterSpec {
	return RegisterSpec{
		Name:    "Materials",
		Comment: "Material stock by construction site.",
		Kind:    "accumulation",
		Period:  "month",
		Dimensions: []RegisterDimensionSpec{
			{
				Name: "construction_site_id",
				Type: "int",
				References: &ReferenceSpec{
					Schema: "public",
					Table:  "construction_sites",
					Column: "id",
				},
			},
			{
				Name: "material_id",
				Type: "int",
				References: &ReferenceSpec{
					Schema: "public",
					Table:  "materials",
					Column: "id",
				},
			},
		},
		Resources: []RegisterResourceSpec{
			{Name: "quant", Type: "numeric", SQLType: "numeric(19, 4)"},
		},
	}
}

func TestParseCommandDefaultsToGenerate(t *testing.T) {
	t.Parallel()

	command, args, err := parseCommand([]string{"-config", "project.yaml"})
	if err != nil {
		t.Fatalf("parseCommand(): %v", err)
	}
	if command != "generate" {
		t.Fatalf("unexpected command %q", command)
	}
	if strings.Join(args, " ") != "-config project.yaml" {
		t.Fatalf("unexpected args %v", args)
	}
}

func TestFinalizeConfigResolvesPathsFromProjectRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/project\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(go.mod): %v", err)
	}

	cfg := DefaultConfig(root)
	cfg.FrontendRoot = "./web"
	cfg.MigrationsDir = "./db/migrations"
	if err := finalizeConfig(&cfg); err != nil {
		t.Fatalf("finalizeConfig(): %v", err)
	}

	assertPath := func(name string, got string, want string) {
		t.Helper()
		if got != filepath.Clean(want) {
			t.Fatalf("%s: got %q, want %q", name, got, filepath.Clean(want))
		}
	}
	assertPath("schema", cfg.SchemaDir, filepath.Join(root, "schema"))
	assertPath("register schema", cfg.RegisterSchemaDir, filepath.Join(root, "schema", "registers"))
	assertPath("server", cfg.ServerRoot, root)
	assertPath("frontend", cfg.FrontendRoot, filepath.Join(root, "web"))
	assertPath("migrations", cfg.MigrationsDir, filepath.Join(root, "db", "migrations"))
	if cfg.GoModule != "example.com/project" {
		t.Fatalf("unexpected module path %q", cfg.GoModule)
	}
}
