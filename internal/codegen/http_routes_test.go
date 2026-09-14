package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadObjectsSupportsHTTPRoutes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content := `
name: Example
table:
  name: examples
route: /examples
keys:
  - name: id
    type: int
fields:
  - name: id
    type: int
    primaryKey: true
crud:
  create: true
  list: true
httpRoutes:
  manualMethods:
    - create
`
	if err := os.WriteFile(filepath.Join(dir, "example.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}

	objects, err := LoadObjects(dir)
	if err != nil {
		t.Fatalf("LoadObjects(): %v", err)
	}
	if len(objects) != 1 || len(objects[0].HTTPRoutes.ManualMethods) != 1 || objects[0].HTTPRoutes.ManualMethods[0] != "create" {
		t.Fatalf("HTTP route ownership was not loaded: %+v", objects)
	}
}

func TestHTTPRoutesDefaultToGeneratedCRUD(t *testing.T) {
	t.Parallel()

	spec := compositeObjectSpec()
	if err := spec.Validate(); err != nil {
		t.Fatalf("Validate(): %v", err)
	}

	view, err := BuildObjectView(testConfig(t), spec)
	if err != nil {
		t.Fatalf("BuildObjectView(): %v", err)
	}

	if !view.HTTPRoutesEnabled {
		t.Fatal("HTTP routes must default to enabled")
	}
	if view.ManualHTTPCRUD != (CRUDSpec{}) {
		t.Fatalf("HTTP routes unexpectedly defaulted to manual ownership: %+v", view.ManualHTTPCRUD)
	}
	if view.GeneratedHTTPCRUD != view.CRUD {
		t.Fatalf("default HTTP CRUD differs from the CRUD contract: got %+v, want %+v", view.GeneratedHTTPCRUD, view.CRUD)
	}
	if !view.HasGeneratedHTTPRoutes {
		t.Fatal("full CRUD must produce generated HTTP routes by default")
	}
}

func TestHTTPRoutesManualMethodsValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		crud    CRUDSpec
		enabled *bool
		methods []string
		want    string
	}{
		{
			name:    "unsupported",
			crud:    CRUDSpec{Create: true},
			methods: []string{"archive"},
			want:    `httpRoutes.manualMethods contains unsupported method "archive"`,
		},
		{
			name:    "disabled CRUD operation",
			crud:    CRUDSpec{Create: true},
			methods: []string{"update"},
			want:    `httpRoutes.manualMethods method "update" requires crud.update: true`,
		},
		{
			name:    "duplicate",
			crud:    CRUDSpec{Create: true},
			methods: []string{"create", " Create "},
			want:    `httpRoutes.manualMethods contains duplicate method "create"`,
		},
		{
			name:    "manual methods with all routes disabled",
			crud:    CRUDSpec{Create: true},
			enabled: boolPointer(false),
			methods: []string{"create"},
			want:    "httpRoutes.manualMethods cannot be set when httpRoutes.enabled is false",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			spec := compositeObjectSpec()
			spec.CRUD = test.crud
			spec.HTTPRoutes = HTTPRoutesSpec{
				Enabled:       test.enabled,
				ManualMethods: test.methods,
			}

			err := spec.Validate()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected error containing %q, got %v", test.want, err)
			}
		})
	}
}

func TestServiceAndHTTPRouteOwnershipAreIndependent(t *testing.T) {
	t.Parallel()

	spec := compositeObjectSpec()
	spec.Service.ManualMethods = []string{" Create "}
	spec.HTTPRoutes.ManualMethods = []string{" Detail ", "update"}
	if err := spec.Validate(); err != nil {
		t.Fatalf("Validate(): %v", err)
	}

	view, err := BuildObjectView(testConfig(t), spec)
	if err != nil {
		t.Fatalf("BuildObjectView(): %v", err)
	}

	if !view.ManualServiceCRUD.Create || view.GeneratedServiceCRUD.Create {
		t.Fatalf("create service ownership was not transferred: manual=%+v generated=%+v", view.ManualServiceCRUD, view.GeneratedServiceCRUD)
	}
	if !view.GeneratedHTTPCRUD.Create {
		t.Fatal("manual service ownership must not suppress the generated create route")
	}
	if !view.ManualHTTPCRUD.Detail || !view.ManualHTTPCRUD.Update {
		t.Fatalf("HTTP route ownership was not transferred: %+v", view.ManualHTTPCRUD)
	}
	if view.GeneratedHTTPCRUD.Detail || view.GeneratedHTTPCRUD.Update {
		t.Fatalf("manually owned HTTP routes remained generated: %+v", view.GeneratedHTTPCRUD)
	}
	if !view.GeneratedServiceCRUD.Detail || !view.GeneratedServiceCRUD.Update {
		t.Fatal("manual HTTP route ownership must not suppress generated service methods")
	}
	if len(view.PermissionRows) != 5 {
		t.Fatalf("manual ownership changed CRUD permissions: %+v", view.PermissionRows)
	}
}

func TestRenderPartialManualHTTPRoutesUsesOnlyRequiredBindersAndImports(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	cfg.ServerRoot = t.TempDir()
	cfg.MigrationsEnabled = false
	cfg.APITestEnabled = false
	cfg.FrontendEnabled = false

	spec := compositeObjectSpec()
	spec.HTTPRoutes.ManualMethods = []string{"create", "update"}
	view, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatalf("BuildObjectView(): %v", err)
	}
	if err := NewRenderer(cfg).RenderObject(view); err != nil {
		t.Fatalf("RenderObject(): %v", err)
	}

	httpAPI := readTestFile(t, filepath.Join(cfg.ServerRoot, "internal/httpapi/catalogueItem.gen.go"))
	for _, unexpected := range []string{
		"api.POST(",
		"api.PATCH(",
		"func catalogueItemUpdateBinder",
		`"github.com/dronm/modelbind"`,
	} {
		if strings.Contains(httpAPI, unexpected) {
			t.Fatalf("manually owned HTTP operation retained generated code %q\n%s", unexpected, httpAPI)
		}
	}
	for _, expected := range []string{
		"api.GET(",
		"api.DELETE(",
		`"/catalogue-items"`,
		`"/catalogue-items/{groupCode}/{itemCode}"`,
		"func catalogueItemKeyBinder",
		"func catalogueItemKeyFromRequest",
		`"net/http"`,
		`"example.com/project/internal/models"`,
	} {
		if !strings.Contains(httpAPI, expected) {
			t.Fatalf("remaining generated HTTP routes are missing %q\n%s", expected, httpAPI)
		}
	}

	service := readTestFile(t, filepath.Join(cfg.ServerRoot, "internal/services/catalogueItem.gen.go"))
	for _, expected := range []string{
		"func (s *CatalogueItemService) Create(",
		"func (s *CatalogueItemService) Update(",
	} {
		if !strings.Contains(service, expected) {
			t.Fatalf("manual HTTP ownership changed service generation; missing %q\n%s", expected, service)
		}
	}
}

func TestDisabledHTTPRoutesRenderPackageOnlyAndAreOmittedFromRegistry(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	cfg.ServerRoot = t.TempDir()
	cfg.MigrationsEnabled = false
	cfg.APITestEnabled = false
	cfg.FrontendEnabled = false

	renderer := NewRenderer(cfg)
	generatedView, err := BuildObjectView(cfg, compositeObjectSpec())
	if err != nil {
		t.Fatalf("BuildObjectView(generated): %v", err)
	}
	if err := renderer.RenderObject(generatedView); err != nil {
		t.Fatalf("RenderObject(generated): %v", err)
	}

	spec := compositeObjectSpec()
	spec.HTTPRoutes.Enabled = boolPointer(false)
	view, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatalf("BuildObjectView(): %v", err)
	}
	if view.HasGeneratedHTTPRoutes || view.GeneratedHTTPCRUD != (CRUDSpec{}) {
		t.Fatalf("disabled HTTP routes retained generated ownership: %+v", view.GeneratedHTTPCRUD)
	}

	if err := renderer.RenderObject(view); err != nil {
		t.Fatalf("RenderObject(): %v", err)
	}
	if err := renderer.RenderRoutes([]ObjectView{view}); err != nil {
		t.Fatalf("RenderRoutes(): %v", err)
	}

	httpAPI := readTestFile(t, filepath.Join(cfg.ServerRoot, "internal/httpapi/catalogueItem.gen.go"))
	if !strings.Contains(httpAPI, "package httpapi") {
		t.Fatalf("disabled HTTP output is not a valid package file\n%s", httpAPI)
	}
	for _, unexpected := range []string{"import ", "func catalogueItemRoutes", "api."} {
		if strings.Contains(httpAPI, unexpected) {
			t.Fatalf("disabled HTTP output contains %q\n%s", unexpected, httpAPI)
		}
	}

	routes := readTestFile(t, filepath.Join(cfg.ServerRoot, "internal/httpapi/routes_gen.go"))
	if strings.Contains(routes, "catalogueItemRoutes(api)") {
		t.Fatalf("disabled object remained in generated route registry\n%s", routes)
	}
}

func TestManualHTTPRouteDoesNotConflictWithSuppressedGeneratedRoute(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "internal", "httpapi", "catalogueItem_actions.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(): %v", err)
	}
	content := `package httpapi
import "github.com/dronm/webapp"
func catalogueItemActions(api *webapp.Group) {
	api.POST("/catalogue-items", webapp.WithName("catalogueItem.create"), webapp.WithPermission("catalogueItem.create"))
}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}

	cfg := testConfig(t)
	manualSpec := compositeObjectSpec()
	manualSpec.HTTPRoutes.ManualMethods = []string{"create"}
	manualView, err := BuildObjectView(cfg, manualSpec)
	if err != nil {
		t.Fatalf("BuildObjectView(manual): %v", err)
	}
	if err := validateManualBackendCollisions(root, []ObjectView{manualView}); err != nil {
		t.Fatalf("manually owned HTTP route was treated as a generated collision: %v", err)
	}

	generatedView, err := BuildObjectView(cfg, compositeObjectSpec())
	if err != nil {
		t.Fatalf("BuildObjectView(generated): %v", err)
	}
	err = validateManualBackendCollisions(root, []ObjectView{generatedView})
	if err == nil || !strings.Contains(err.Error(), `HTTP route "POST /catalogue-items"`) {
		t.Fatalf("expected generator-owned HTTP route collision, got %v", err)
	}
}
