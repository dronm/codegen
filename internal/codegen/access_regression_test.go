package codegen

import (
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestAccessBindingYAMLRoundTripPreservesPresence(t *testing.T) {
	maps := []struct {
		name  string
		value map[string]string
	}{
		{"omitted", nil},
		{"empty", map[string]string{}},
		{"mapped", map[string]string{"owner": "customer_id"}},
	}
	for _, rows := range maps {
		for _, writes := range maps {
			t.Run(rows.name+"/"+writes.name, func(t *testing.T) {
				original := AccessBindingSpec{Rows: rows.value, Writes: writes.value}
				data, err := yaml.Marshal(original)
				if err != nil {
					t.Fatal(err)
				}
				var decoded AccessBindingSpec
				decoder := yaml.NewDecoder(bytes.NewReader(data))
				decoder.KnownFields(true)
				if err := decoder.Decode(&decoded); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(original, decoded) {
					t.Fatalf("binding changed after YAML round trip:\n%s\noriginal=%#v\ndecoded=%#v", data, original, decoded)
				}
				again, err := yaml.Marshal(decoded)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(data, again) {
					t.Fatalf("unstable YAML encoding:\n%s\n%s", data, again)
				}
			})
		}
	}
	var decoded AccessBindingSpec
	decoder := yaml.NewDecoder(strings.NewReader("rows: {}\nwrites: {}\nunknown: true\n"))
	decoder.KnownFields(true)
	if err := decoder.Decode(&decoded); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("strict unknown-field validation was bypassed: %v", err)
	}
}

func TestAccessEmptyMappingsSurviveObjectYAMLRoundTrip(t *testing.T) {
	cfg, spec := accessTestConfig(t), accessTestSpec()
	spec.Access.Bindings.Model = &AccessBindingSpec{Rows: map[string]string{}, Writes: map[string]string{}}
	spec.Access.Bindings.List = &AccessBindingSpec{Rows: map[string]string{}}
	spec.Access.Bindings.Detail = &AccessBindingSpec{Rows: map[string]string{}}
	spec.Access.Bindings.Key = &AccessBindingSpec{Rows: map[string]string{}}
	data, err := yaml.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	writeAccessTestFile(t, cfg.ServerRoot, "schema/order.yaml", string(data))
	objects, err := LoadObjects(cfg.SchemaDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(objects) != 1 || !reflect.DeepEqual(objects[0].Access.Bindings, spec.Access.Bindings) {
		t.Fatalf("explicit empty bindings changed: %+v", objects)
	}
	view := accessBuild(t, cfg, objects[0])
	if len(view.Access.Models) != 4 {
		t.Fatalf("lost empty-binding representations: %+v", view.Access.Models)
	}

	// Omitted and explicitly empty remain different even after serialization.
	objects[0].Access.Bindings.Model.Writes = nil
	if _, err := BuildObjectView(cfg, objects[0]); err == nil || !strings.Contains(err.Error(), "bindings.model.writes is required") {
		t.Fatalf("omitted writable binding accepted: %v", err)
	}
	objects[0].Access.Bindings.Model.Writes = map[string]string{}
	objects[0].Access.Bindings.List.Writes = map[string]string{}
	if _, err := BuildObjectView(cfg, objects[0]); err == nil || !strings.Contains(err.Error(), "bindings.list.writes is inapplicable") {
		t.Fatalf("explicit read-only writes accepted: %v", err)
	}
}

func TestAccessWriteTargetsNormalizeBeforeMetadataChecksAndRendering(t *testing.T) {
	cfg, spec := accessTestConfig(t), accessTestSpec()
	spec.Access.Bindings.Model.Writes["owner"] = "CUSTOMER_ID"
	view := accessBuild(t, cfg, spec)
	want := []AccessFieldView{{Field: "owner", Column: "customer_id"}}
	if !reflect.DeepEqual(view.Access.Models[0].Writes, want) {
		t.Fatalf("write target was not normalized: %+v", view.Access.Models[0].Writes)
	}
	if spec.Access.Bindings.Model.Writes["owner"] != "CUSTOMER_ID" {
		t.Fatal("building the view mutated the YAML specification")
	}
	if err := NewRenderer(cfg).RenderAccessModels([]ObjectView{view}); err != nil {
		t.Fatal(err)
	}
	registration := readTestFile(t, filepath.Join(cfg.ServerRoot, "internal/services/access_models.gen.go"))
	if strings.Contains(registration, "CUSTOMER_ID") || !strings.Contains(registration, `"owner": "customer_id"`) {
		t.Fatalf("registry must use the case-sensitive modelbind field ID:\n%s", registration)
	}

	for _, tc := range []struct {
		name   string
		change func(*FieldSpec)
		want   string
	}{
		{"server generated", func(f *FieldSpec) { f.ServerGenerated = true }, "non-writable"},
		{"read only", func(f *FieldSpec) { f.ReadOnly = true }, "non-writable"},
		{"different JSON ID", func(f *FieldSpec) { f.JSONName = "customer" }, "modelbind field ID"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec := accessTestSpec()
			spec.Access.Bindings.Model.Writes["owner"] = "CUSTOMER_ID"
			tc.change(&spec.Fields[1])
			if _, err := BuildObjectView(accessTestConfig(t), spec); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("case variant bypassed metadata validation; wanted %q, got %v", tc.want, err)
			}
		})
	}
	// Canonicalization also covers columns intentionally absent from the DTO.
	spec.Fields = append(spec.Fields[:1], spec.Fields[2:]...)
	view = accessBuild(t, cfg, spec)
	if !reflect.DeepEqual(view.Access.Models[0].Writes, want) {
		t.Fatal(view.Access.Models[0].Writes)
	}
}

func TestAccessManualExtensionsWorkThroughGenerateAndCheck(t *testing.T) {
	cfg, spec := accessTestConfig(t), accessTestSpec()
	spec.Service.ManualMethods = []string{"create", "update"}
	spec.HTTPRoutes.ManualMethods = []string{"update"}
	spec.Access.ServiceInputs = map[string]AccessServiceInputSpec{"Submit": {Operation: "insert"}}
	manualService := `package services
import (
	"context"
	"github.com/dronm/modelbind"
	"example.com/project/internal/models"
)
func (s *OrderService) Create(ctx context.Context, input modelbind.ModelInput[models.Order]) (map[string]any, error) { return nil, nil }
func (s *OrderService) Update(ctx context.Context, input modelbind.ModelInput[*models.Order]) error { return nil }
func (s *OrderService) Submit(ctx context.Context, input *models.Order) error { return nil }
`
	manualHTTP := `package httpapi
import "github.com/dronm/webapp"
func orderCustomRoutes(api *webapp.Group) {
	api.PATCH("/orders/{orderID}", webapp.WithService("Order", "Update"))
}
`
	writeAccessTestFile(t, cfg.ServerRoot, "internal/services/order.go", manualService)
	writeAccessTestFile(t, cfg.ServerRoot, "internal/httpapi/order.go", manualHTTP)
	data, err := yaml.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	writeAccessTestFile(t, cfg.ServerRoot, "schema/order.yaml", string(data))
	if cfg.AllowManualCollisions {
		t.Fatal("test must run with collision protection enabled")
	}
	if err := Generate(cfg); err != nil {
		t.Fatal(err)
	}
	before := accessTree(t, cfg.ServerRoot)
	if err := Generate(cfg); err != nil {
		t.Fatal(err)
	}
	check := cfg
	check.Check = true
	if err := Generate(check); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, accessTree(t, cfg.ServerRoot)) {
		t.Fatal("manual-extension generation/check was not deterministic")
	}
	for path, want := range map[string]string{"internal/services/order.go": manualService, "internal/httpapi/order.go": manualHTTP} {
		if got := readTestFile(t, filepath.Join(cfg.ServerRoot, path)); got != want {
			t.Fatalf("handwritten source changed: %s", path)
		}
	}
	service := readTestFile(t, filepath.Join(cfg.ServerRoot, "internal/services/order.gen.go"))
	if strings.Count(service, "WithMethodInputDecoder(") != 3 || strings.Contains(service, "func (s *OrderService) Create(") || strings.Contains(service, "func (s *OrderService) Update(") {
		t.Fatalf("incorrect manual-method registration/ownership:\n%s", service)
	}
}

func TestAccessSymbolCollisionChecksRemainEnabled(t *testing.T) {
	for _, tc := range []struct{ name, path, source, want string }{
		{"service type", "internal/services/order.go", "package services\ntype OrderService struct{}\n", "service symbol"},
		{"service factory", "internal/services/order.go", "package services\nfunc NewOrderService() {}\n", "service symbol"},
		{"registration", "internal/services/order.go", "package services\nfunc RegisterOrderService() {}\n", "service symbol"},
		{"generated method", "internal/services/order.go", "package services\nfunc (*OrderService) List() {}\n", "service method"},
		{"database helper", "internal/services/order.go", "package services\nfunc (*OrderService) requireDB() {}\n", "service helper"},
		{"session helper", "internal/services/order.go", "package services\nfunc (*OrderService) requireSession() {}\n", "service helper"},
		{"HTTP function", "internal/httpapi/order.go", "package httpapi\nfunc orderRoutes() {}\n", "HTTP symbol"},
		{"HTTP route", "internal/httpapi/order.go", "package httpapi\nfunc custom(api any) { api.PATCH(\"/orders/{orderID}\") }\n", "HTTP route"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, spec := accessTestConfig(t), accessTestSpec()
			spec.SessionRequired = boolPointer(true)
			writeAccessTestFile(t, cfg.ServerRoot, tc.path, tc.source)
			view := accessBuild(t, cfg, spec)
			before := accessTree(t, cfg.ServerRoot)
			if err := NewRenderer(cfg).RenderObject(view); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("wanted %q collision, got %v", tc.want, err)
			}
			if !reflect.DeepEqual(before, accessTree(t, cfg.ServerRoot)) {
				t.Fatal("collision modified source or wrote partial output")
			}
		})
	}
}
