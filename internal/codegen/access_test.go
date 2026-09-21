package codegen

import (
	"bytes"
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func accessTestConfig(t *testing.T) Config {
	t.Helper()
	cfg := testConfig(t)
	cfg.ServerRoot = t.TempDir()
	cfg.SchemaDir = filepath.Join(cfg.ServerRoot, "schema")
	cfg.APITestEnabled = false
	cfg.FrontendEnabled = false
	return cfg
}

func accessTestSpec() ObjectSpec {
	binding := func(write bool) *AccessBindingSpec {
		b := &AccessBindingSpec{Rows: map[string]string{"owner": "customer_id"}}
		if write {
			b.Writes = map[string]string{"owner": "customer_id"}
		}
		return b
	}
	return ObjectSpec{
		SourceFile: "schema/order.yaml", Name: "Order", Table: TableSpec{Schema: "public", Name: "orders"},
		Route: "/orders", SessionRequired: boolPointer(false), CRUDNotifications: boolPointer(true),
		Keys: []KeySpec{{Name: "id", PathName: "orderID", Type: "int"}},
		Fields: []FieldSpec{
			{Name: "id", Type: "int", PrimaryKey: true, Required: true},
			{Name: "customer_id", Type: "int", GoType: "*int", Required: true},
			{Name: "name", Type: "text", Required: true},
			{Name: "priority", Type: "int"},
			{Name: "note", Type: "text", Nullable: true},
		},
		List: &ListSpec{Model: "OrderOverview", Table: TableSpec{Schema: "public", Name: "orders", Alias: "o"}, Fields: []FieldSpec{
			{Name: "id", Type: "int", PrimaryKey: true}, {Name: "name", Type: "text"},
		}},
		Detail: &ListSpec{Model: "OrderDetails", Table: TableSpec{Schema: "public", Name: "order_details"}, Fields: []FieldSpec{
			{Name: "id", Type: "int", PrimaryKey: true}, {Name: "name", Type: "text"},
		}},
		CRUD: CRUDSpec{Create: true, List: true, Detail: true, Update: true, Delete: true},
		Access: &AccessSpec{Enabled: true, Bindings: AccessBindingsSpec{
			Model: binding(true), List: &AccessBindingSpec{Rows: map[string]string{"owner": "o.customer_id"}},
			Detail: binding(false), Key: binding(false),
		}},
	}
}

func accessBuild(t *testing.T, cfg Config, spec ObjectSpec) ObjectView {
	t.Helper()
	view, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatal(err)
	}
	return view
}

func writeAccessTestFile(t *testing.T, root, path, content string) {
	t.Helper()
	filename := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAccessYAMLStrictParsing(t *testing.T) {
	cfg := accessTestConfig(t)
	base := `name: Order
table: {name: orders}
route: /orders
keys: [{name: id, type: int}]
fields: [{name: id, type: int, primaryKey: true}]
crud: {list: true}
access:
  enabled: true
  bindings:
    model: {rows: {owner: customer_id}}
    key: {rows: {owner: customer_id}}
`
	writeAccessTestFile(t, cfg.ServerRoot, "schema/order.yaml", base)
	objects, err := LoadObjects(cfg.SchemaDir)
	if err != nil {
		t.Fatal(err)
	}
	view := accessBuild(t, cfg, objects[0])
	if view.Access.Resource != "Order" || len(view.Access.Models) != 2 {
		t.Fatalf("incorrect defaults: %+v", view.Access)
	}
	for _, change := range []struct{ old, replacement, want string }{
		{"  enabled: true", "  enabled: true\n  resource: null", "resource"},
		{"  enabled: true", "  enabled: true\n  resource: '   '", "resource"},
		{"  bindings:", "  typo: true\n  bindings:", "typo"},
		{"model: {rows: {owner: customer_id}}", "model: {rows: {owner: customer_id}, sql: true}", "sql"},
	} {
		writeAccessTestFile(t, cfg.ServerRoot, "schema/order.yaml", strings.Replace(base, change.old, change.replacement, 1))
		objects, err := LoadObjects(cfg.SchemaDir)
		if err == nil {
			_, err = BuildObjectView(cfg, objects[0])
		}
		if err == nil || !strings.Contains(err.Error(), change.want) {
			t.Fatalf("wanted %q, got %v", change.want, err)
		}
	}
}

func TestAccessValidationAndExplicitBindings(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*ObjectSpec)
		want   string
	}{
		{"missing model", func(s *ObjectSpec) { s.Access.Bindings.Model = nil }, "bindings.model.rows"},
		{"missing list", func(s *ObjectSpec) { s.Access.Bindings.List = nil }, "bindings.list.rows"},
		{"missing detail", func(s *ObjectSpec) { s.Access.Bindings.Detail = nil }, "bindings.detail.rows"},
		{"missing key", func(s *ObjectSpec) { s.Access.Bindings.Key = nil }, "bindings.key.rows"},
		{"missing writes", func(s *ObjectSpec) { s.Access.Bindings.Model.Writes = nil }, "bindings.model.writes"},
		{"readonly write", func(s *ObjectSpec) { s.Access.Bindings.List.Writes = map[string]string{} }, "bindings.list.writes"},
		{"write qualifier", func(s *ObjectSpec) { s.Access.Bindings.Model.Writes["owner"] = "orders.customer_id" }, "unqualified"},
		{"invented alias", func(s *ObjectSpec) { s.Access.Bindings.List.Rows["owner"] = "unknown.customer_id" }, "not exposed"},
		{"expression", func(s *ObjectSpec) { s.Access.Bindings.Model.Rows["owner"] = "customer_id::int" }, "expressions"},
		{"logical sql", func(s *ObjectSpec) { s.Access.Bindings.Model.Rows["bad()"] = "customer_id" }, "logical identifier"},
		{"duplicate target", func(s *ObjectSpec) { s.Access.Bindings.Model.Writes["other"] = "customer_id" }, "conflict"},
		{"case-folded duplicate target", func(s *ObjectSpec) { s.Access.Bindings.Model.Writes["other"] = "CUSTOMER_ID" }, "conflict"},
		{"server calc", func(s *ObjectSpec) { s.Fields[1].ServerGenerated = true }, "non-writable"},
		{"different JSON ID", func(s *ObjectSpec) { s.Fields[1].JSONName = "customer" }, "modelbind field ID"},
		{"writable alias", func(s *ObjectSpec) { s.Table.Alias = "o" }, "writable table"},
		{"unknown method", func(s *ObjectSpec) {
			s.Access.ServiceInputs = map[string]AccessServiceInputSpec{"Save": {Operation: "insert"}}
		}, "existing"},
		{"wrong CRUD operation", func(s *ObjectSpec) {
			s.Access.ServiceInputs = map[string]AccessServiceInputSpec{"Create": {Operation: "update"}}
		}, "conflicts"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, spec := accessTestConfig(t), accessTestSpec()
			tc.change(&spec)
			_, err := BuildObjectView(cfg, spec)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("wanted %q, got %v", tc.want, err)
			}
		})
	}
	cfg, spec := accessTestConfig(t), accessTestSpec()
	// A policy column deliberately absent from every selected/input field.
	spec.Fields = append(spec.Fields[:1], spec.Fields[2:]...)
	view := accessBuild(t, cfg, spec)
	if len(view.Access.Models) != 4 || view.ListRelation != "public.orders AS o" {
		t.Fatalf("wrong concrete representations: %+v", view.Access)
	}
	for _, model := range view.Access.Models {
		if model.Resource != "Order" || len(model.Rows) != 1 {
			t.Fatal(model)
		}
	}
	spec.List, spec.Detail = nil, nil
	spec.Access.Bindings.List, spec.Access.Bindings.Detail = nil, nil
	view = accessBuild(t, cfg, spec)
	if len(view.Access.Models) != 2 {
		t.Fatal("reused base type must register once")
	}
	spec.Access.Bindings.List = &AccessBindingSpec{Rows: map[string]string{"owner": "customer_id"}}
	accessBuild(t, cfg, spec) // Explicit equality assertion is supported.
	spec.Access.Bindings.List.Rows["owner"] = "different_owner"
	if _, err := BuildObjectView(cfg, spec); err == nil || !strings.Contains(err.Error(), "reuses") {
		t.Fatalf("conflicting reused binding accepted: %v", err)
	}
}

func TestAccessRenderingMarkersBindingsAndTransport(t *testing.T) {
	cfg, spec := accessTestConfig(t), accessTestSpec()
	view := accessBuild(t, cfg, spec)
	r := NewRenderer(cfg)
	if err := r.RenderObject(view); err != nil {
		t.Fatal(err)
	}
	if err := r.RenderAccessModels([]ObjectView{view}); err != nil {
		t.Fatal(err)
	}
	markers := readTestFile(t, filepath.Join(cfg.ServerRoot, "internal/models/order.access.gen.go"))
	registrations := readTestFile(t, filepath.Join(cfg.ServerRoot, "internal/services/access_models.gen.go"))
	for _, name := range []string{"Order", "OrderOverview", "OrderDetails", "OrderKey"} {
		if strings.Count(markers, "func ("+name+") AccessResource() string") != 1 || strings.Count(registrations, "registry.RegisterModel(models."+name+"{}") != 1 {
			t.Fatal("marker or registration not emitted exactly once for", name)
		}
		if !strings.Contains(markers, "(*"+name+")(nil)") {
			t.Fatal("missing pointer assertion", name)
		}
	}
	if !strings.Contains(registrations, `"owner": "o.customer_id"`) || !strings.Contains(registrations, "return fmt.Errorf(") || strings.Contains(registrations, "AllowAll") {
		t.Fatal(registrations)
	}
	binder := readTestFile(t, filepath.Join(cfg.ServerRoot, "internal/httpapi/order.access.gen.go"))
	for _, expected := range []string{"DecodeRequestInput[*models.Order]", "PrepareAccessInput(r.Context(), input, access.Update)", "keyInput.Validate(true)", `values.Set("id", r.PathValue("orderID"))`} {
		if !strings.Contains(binder, expected) {
			t.Fatal("missing", expected)
		}
	}
	if strings.Contains(binder, "input.Validate(") || strings.Contains(binder, "context.Background") {
		t.Fatal(binder)
	}
	http := readTestFile(t, filepath.Join(cfg.ServerRoot, "internal/httpapi/order.gen.go"))
	if strings.Contains(http, "UpdateByPathKeysBinder") || !strings.Contains(http, "orderAccessUpdateBinder()") {
		t.Fatal(http)
	}
	service := readTestFile(t, filepath.Join(cfg.ServerRoot, "internal/services/order.gen.go"))
	for _, expected := range []string{`WithMethodInputDecoder("Create", webapp.ModelInputJSONDecoder[*models.Order](true))`, `WithMethodInputDecoder("Update", webapp.UpdateByKeysJSONDecoder[*models.OrderKey, *models.Order]())`, "webapp.WithCRUDNotifications()", "webapp.InsertModelInput(ctx,", "webapp.UpdateModelInput(ctx,"} {
		if !strings.Contains(service, expected) {
			t.Fatal("missing", expected)
		}
	}
	if strings.Count(service, "WithMethodInputDecoder(") != 2 {
		t.Fatal(service)
	}
	_ = filepath.WalkDir(cfg.ServerRoot, func(path string, entry os.DirEntry, err error) error {
		if err == nil && strings.HasSuffix(path, ".go") {
			if _, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.AllErrors); err != nil {
				t.Error(err)
			}
		}
		return err
	})
}

func TestAccessManualMethodSignaturesAndNames(t *testing.T) {
	cfg, spec := accessTestConfig(t), accessTestSpec()
	spec.Service.ManualMethods = []string{"create", "update"}
	spec.HTTPRoutes.ManualMethods = []string{"update"}
	spec.Access.ServiceInputs = map[string]AccessServiceInputSpec{"Submit": {Operation: "insert"}}
	manual := `package services
import (
 c "context"
 mb "github.com/dronm/modelbind"
 m "example.com/project/internal/models"
)
func (s *OrderService) Create(ctx c.Context, input mb.ModelInput[m.Order]) (map[string]any, error) { return nil,nil }
func (s *OrderService) Update(ctx c.Context, input mb.ModelInput[*m.Order]) error { return nil }
func (s *OrderService) Submit(ctx c.Context, input *m.Order) error { return nil }
`
	writeAccessTestFile(t, cfg.ServerRoot, "internal/services/order.go", manual)
	view := accessBuild(t, cfg, spec)
	if view.Access.CreateBinder != "orderAccessCreateBinder()" || view.GeneratedServiceCRUD.Create {
		t.Fatal(view.Access)
	}
	decoders := map[string]string{}
	for _, decoder := range view.Access.Decoders {
		decoders[decoder.Method] = decoder.Expression
	}
	for method, expected := range map[string]string{
		"Create": "orderCreateAccessJSONDecoder()",
		"Update": "webapp.ModelInputJSONDecoder[*models.Order](false)",
		"Submit": "webapp.ModelJSONDecoder[*models.Order](true)",
	} {
		if decoders[method] != expected {
			t.Fatal(method, decoders[method])
		}
	}
	r := NewRenderer(cfg)
	if err := r.RenderObject(view); err != nil {
		t.Fatal(err)
	}
	if err := r.RenderAccessModels([]ObjectView{view}); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, filepath.Join(cfg.ServerRoot, "internal/services/order.go")); got != manual {
		t.Fatal("handwritten service was modified")
	}
	delete(spec.Access.ServiceInputs, "Submit")
	if _, err := BuildObjectView(cfg, spec); err == nil || !strings.Contains(err.Error(), "serviceInputs.Submit.operation") {
		t.Fatal(err)
	}
}

func TestAccessUnsupportedCustomSignatureRequiresExplicitIntegration(t *testing.T) {
	cfg, spec := accessTestConfig(t), accessTestSpec()
	spec.Service.ManualMethods = []string{"update"}
	writeAccessTestFile(t, cfg.ServerRoot, "internal/services/order.go", `package services
import "context"
type CustomInput struct { Value string }
func(s *OrderService) Update(ctx context.Context, input CustomInput) error { return nil }
`)
	if _, err := BuildObjectView(cfg, spec); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatal(err)
	}
	spec.Access.ServiceInputs = map[string]AccessServiceInputSpec{"Update": {Operation: "update", Decoder: &AccessDecoderSpec{Name: "DecodeOrder", Import: "example.com/project/internal/policyinput"}}}
	if _, err := BuildObjectView(cfg, spec); err == nil || !strings.Contains(err.Error(), "manualMethods") {
		t.Fatal(err)
	}
	spec.HTTPRoutes.ManualMethods = []string{"update"}
	view := accessBuild(t, cfg, spec)
	if len(view.Access.DecoderImports) != 1 || view.Access.DecoderImports[0].Path != "example.com/project/internal/policyinput" {
		t.Fatal(view.Access)
	}
}

func TestAccessOwnershipCollisionsCannotBeOverridden(t *testing.T) {
	for _, tc := range []struct{ path, content, want string }{
		{"internal/models/order_manual.go", "package models\nfunc(Order) AccessResource() string { return \"Other\" }", "AccessResource"},
		{"internal/models/order_manual.go", "package models\ntype Order struct{}", "manually owned model"},
		{"internal/services/access_manual.go", "package services\nfunc RegisterAccessModels(){}", "RegisterAccessModels"},
		{"internal/httpapi/order_manual.go", "package httpapi\nfunc orderAccessUpdateBinder(){}", "orderAccessUpdateBinder"},
		{"internal/models/order.gen.go", "package models\ntype Order struct{}", "developer-owned"},
	} {
		t.Run(tc.path+tc.want, func(t *testing.T) {
			cfg, spec := accessTestConfig(t), accessTestSpec()
			cfg.AllowManualCollisions = true
			writeAccessTestFile(t, cfg.ServerRoot, tc.path, tc.content)
			view := accessBuild(t, cfg, spec)
			err := validateAccessObjects(cfg, []ObjectView{view})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("wanted %s: %v", tc.want, err)
			}
			if got := readTestFile(t, filepath.Join(cfg.ServerRoot, tc.path)); got != tc.content {
				t.Fatal("collision file changed")
			}
		})
	}
}

func TestAccessDeterminismDisableAndSchemaRemoval(t *testing.T) {
	cfg, spec := accessTestConfig(t), accessTestSpec()
	data, err := yaml.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	writeAccessTestFile(t, cfg.ServerRoot, "schema/order.yaml", string(data))
	if err := Generate(cfg); err != nil {
		t.Fatal(err)
	}
	before := accessTree(t, cfg.ServerRoot)
	if err := Generate(cfg); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, accessTree(t, cfg.ServerRoot)) {
		t.Fatal("generation is not deterministic")
	}
	check := cfg
	check.Check = true
	if err := Generate(check); err != nil {
		t.Fatal(err)
	}
	spec.Access.Enabled = false
	data, _ = yaml.Marshal(spec)
	writeAccessTestFile(t, cfg.ServerRoot, "schema/order.yaml", string(data))
	if err := Generate(check); err == nil {
		t.Fatal("check accepted stale protected artifacts")
	}
	if err := Generate(cfg); err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{".codegen/access.json", "internal/models/order.access.gen.go", "internal/httpapi/order.access.gen.go", "internal/services/access_models.gen.go"} {
		if _, err := os.Stat(filepath.Join(cfg.ServerRoot, relative)); !os.IsNotExist(err) {
			t.Fatal("stale artifact:", relative, err)
		}
	}
	if strings.Contains(readTestFile(t, filepath.Join(cfg.ServerRoot, "internal/services/order.gen.go")), "WithMethodInputDecoder") {
		t.Fatal("stale decoder")
	}
	if _, err := os.Stat(filepath.Join(cfg.ServerRoot, "internal/models/order.gen.go")); err != nil {
		t.Fatal("active legacy object removed", err)
	}
	// Re-enable, then remove the object entirely: no orphan unmarked backend remains.
	spec.Access.Enabled = true
	data, _ = yaml.Marshal(spec)
	writeAccessTestFile(t, cfg.ServerRoot, "schema/order.yaml", string(data))
	if err := Generate(cfg); err != nil {
		t.Fatal(err)
	}
	writeAccessTestFile(t, cfg.ServerRoot, "internal/models/keep.go", "package models\n// handwritten\n")
	if err := os.Remove(filepath.Join(cfg.SchemaDir, "order.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := Generate(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cfg.ServerRoot, "internal/models/order.gen.go")); !os.IsNotExist(err) {
		t.Fatal("orphan model", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.ServerRoot, "internal/models/keep.go")); err != nil {
		t.Fatal("deleted manual file", err)
	}
}

func TestAccessCleanupRefusesModifiedOrUntrackedFiles(t *testing.T) {
	cfg := accessTestConfig(t)
	view := accessBuild(t, cfg, accessTestSpec())
	r := NewRenderer(cfg)
	if err := r.RenderAccessModels([]ObjectView{view}); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(cfg.ServerRoot, "internal/models/order.access.gen.go")
	data := readTestFile(t, filename) + "\n// manually changed\n"
	writeAccessTestFile(t, cfg.ServerRoot, "internal/models/order.access.gen.go", data)
	if err := r.RenderAccessModels(nil); err == nil || !strings.Contains(err.Error(), "modified stale") {
		t.Fatal(err)
	}
	if got := readTestFile(t, filename); got != data {
		t.Fatal("changed file deleted")
	}
}

func TestAccessDoesNotChangeFrontendOrLegacyOutputs(t *testing.T) {
	cfg, spec := accessTestConfig(t), accessTestSpec()
	cfg.FrontendEnabled = true
	// Frontend projections/reference types are not needed for this contract check.
	spec.List, spec.Detail = nil, nil
	spec.Access.Bindings.List, spec.Access.Bindings.Detail = nil, nil
	protected := accessBuild(t, cfg, spec)
	spec.Access = nil
	legacy := accessBuild(t, cfg, spec)
	if !reflect.DeepEqual(protected.Frontend, legacy.Frontend) {
		t.Fatal("access metadata changed frontend contracts")
	}
	r := NewRenderer(cfg)
	for _, name := range []string{"model.go.tmpl", "service.go.tmpl", "httpapi.go.tmpl"} {
		spec.Access = &AccessSpec{Enabled: false}
		disabled := accessBuild(t, cfg, spec)
		a, err := r.executeTemplate(filepath.Join("go", name), legacy)
		if err != nil {
			t.Fatal(err)
		}
		b, err := r.executeTemplate(filepath.Join("go", name), disabled)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a, b) {
			t.Fatal("disabled access changed", name)
		}
	}
}

func accessTree(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			relative, _ := filepath.Rel(root, path)
			result[relative] = string(data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestAccessManifestRejectsTraversal(t *testing.T) {
	cfg := accessTestConfig(t)
	data, _ := json.Marshal(accessManifest{Version: 1, Files: []accessArtifact{{Path: "../../outside.go", Kind: "access", SHA256: strings.Repeat("0", 64)}}})
	writeAccessTestFile(t, cfg.ServerRoot, accessManifestPath, string(data))
	if _, _, err := readAccessManifest(cfg.ServerRoot); err == nil {
		t.Fatal("unsafe manifest accepted")
	}
}

func TestAccessAdditionalWriteMethodWithoutCRUDCreateOrUpdate(t *testing.T) {
	cfg, spec := accessTestConfig(t), accessTestSpec()
	spec.CRUD = CRUDSpec{List: true}
	spec.Access.ServiceInputs = map[string]AccessServiceInputSpec{"Submit": {Operation: "insert"}}
	writeAccessTestFile(t, cfg.ServerRoot, "internal/services/order_manual.go", `package services
import (
	"context"
	"github.com/dronm/modelbind"
	"example.com/project/internal/models"
)
func (s *OrderService) Submit(ctx context.Context, input modelbind.ModelInput[*models.Order]) error {return nil}
`)
	view := accessBuild(t, cfg, spec)
	if len(view.Access.Decoders) != 1 || view.Access.Decoders[0].Method != "Submit" || len(view.Access.Models[0].Writes) != 1 {
		t.Fatal(view.Access)
	}
}

func TestAccessValueAdaptersUsePointerValidationAndExactReturnTypes(t *testing.T) {
	cfg, spec := accessTestConfig(t), accessTestSpec()
	spec.Service.ManualMethods = []string{"create", "update"}
	writeAccessTestFile(t, cfg.ServerRoot, "internal/services/order_manual.go", `package services
import (
	"context"
	"github.com/dronm/webapp"
	"example.com/project/internal/models"
)
func (s *OrderService) Create(ctx context.Context, model models.Order) error{return nil}
func (s *OrderService) Update(ctx context.Context, input webapp.UpdateByKeysInput[models.OrderKey,models.Order]) error{return nil}
`)
	view := accessBuild(t, cfg, spec)
	if len(view.Access.DecoderAdapters) != 2 || !view.Access.CreateValue || !view.Access.UpdateKeyValue || !view.Access.UpdateModelValue {
		t.Fatal(view.Access)
	}
	r := NewRenderer(cfg)
	if err := r.RenderObject(view); err != nil {
		t.Fatal(err)
	}
	if err := r.RenderAccessModels([]ObjectView{view}); err != nil {
		t.Fatal(err)
	}
	jsonAdapter := readTestFile(t, filepath.Join(cfg.ServerRoot, "internal/services/order.access.gen.go"))
	for _, expected := range []string{"webapp.ModelJSONDecoder[*models.Order](true)", "webapp.UpdateByKeysJSONDecoder[*models.OrderKey, *models.Order]()", "webapp.UpdateByKeysInput[models.OrderKey, models.Order]", "AbsentFields: input.Input.AbsentFields"} {
		if !strings.Contains(jsonAdapter, expected) {
			t.Fatal("missing value adapter", expected, jsonAdapter)
		}
	}
	httpAdapter := readTestFile(t, filepath.Join(cfg.ServerRoot, "internal/httpapi/order.access.gen.go"))
	if !strings.Contains(httpAdapter, "DecodeURLValuesInput[*models.OrderKey]") || !strings.Contains(httpAdapter, "webapp.InsertModelBinder[*models.Order]()") {
		t.Fatal(httpAdapter)
	}
	// Value signatures must not instantiate APIs that only validate pointers.
	if strings.Contains(jsonAdapter, "webapp.ModelJSONDecoder[models.Order]") {
		t.Fatal(jsonAdapter)
	}
}

func TestAccessConflictingResourcesOnOneRelationFailEarly(t *testing.T) {
	cfg, spec := accessTestConfig(t), accessTestSpec()
	a := accessBuild(t, cfg, spec)
	spec.Name = "Other"
	spec.List, spec.Detail = nil, nil
	spec.Access.Bindings.List, spec.Access.Bindings.Detail = nil, nil
	b := accessBuild(t, cfg, spec)
	if err := validateAccessObjects(cfg, []ObjectView{a, b}); err == nil || !strings.Contains(err.Error(), "already assigned") {
		t.Fatal(err)
	}
}
