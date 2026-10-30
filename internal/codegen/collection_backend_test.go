package codegen

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGroupedCollectionBackendWiring(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	view, err := BuildObjectView(cfg, compositeObjectSpec())
	if err != nil {
		t.Fatal(err)
	}
	renderer := NewRenderer(cfg)
	for _, filename := range []string{"service.go.tmpl", "httpapi.go.tmpl"} {
		t.Run(filename, func(t *testing.T) {
			legacy, err := renderer.executeTemplate(filepath.Join("go", filename), view)
			if err != nil {
				t.Fatal(err)
			}
			groupedView := view
			groupedView.GroupedFilters = true
			grouped, err := renderer.executeTemplate(filepath.Join("go", filename), groupedView)
			if err != nil {
				t.Fatal(err)
			}
			for _, output := range [][]byte{legacy, grouped} {
				if _, err := parser.ParseFile(token.NewFileSet(), filename, output, parser.AllErrors); err != nil {
					t.Fatalf("generated Go is invalid: %v", err)
				}
			}
			var required []string
			if filename == "service.go.tmpl" {
				required = []string{
					"params modelbind.CollectionParamsWithWhere,",
					"webapp.FetchCollectionModelWhere(ctx, s.DB, &models.CatalogueItemList{}, params)",
					"Rows: rows,",
					"Agg:  total,",
				}
			} else {
				required = []string{"webapp.CollectionParamsWithWhereBinder()", `"/catalogue-items"`, `"catalogueItem.list"`}
			}
			for _, needle := range required {
				if !strings.Contains(string(grouped), needle) {
					t.Errorf("grouped output missing %q", needle)
				}
			}
			// Opting in changes only the binder, List argument, and fetch helper;
			// URLs, projections, permissions and all other CRUD bodies stay equal.
			normalized := strings.ReplaceAll(string(grouped), "CollectionParamsWithWhere", "CollectionParams")
			normalized = strings.ReplaceAll(normalized, "FetchCollectionModelWhere", "FetchCollectionModel")
			if normalized != string(legacy) {
				t.Fatal("grouped opt-in changed unrelated generated output")
			}
			if strings.Contains(string(legacy), "WithWhere") || strings.Contains(string(legacy), "FetchCollectionModelWhere") {
				t.Fatal("default generation must retain the legacy collection contract")
			}
		})
	}
}

func TestGroupedManualListSignatureValidation(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name           string
		parameters     string
		grouped        bool
		generatedRoute bool
		backend        bool
		wantError      bool
	}{
		{"grouped value", "ctx ctxpkg.Context, params binding.CollectionParamsWithWhere", true, true, true, false},
		{"legacy value", "ctx ctxpkg.Context, params binding.CollectionParams", true, true, true, true},
		{"grouped pointer", "ctx ctxpkg.Context, params *binding.CollectionParamsWithWhere", true, true, true, true},
		{"wrong context", "ctx string, params binding.CollectionParamsWithWhere", true, true, true, true},
		{"extra argument", "ctx ctxpkg.Context, params binding.CollectionParamsWithWhere, extra string", true, true, true, true},
		{"unopted legacy", "ctx ctxpkg.Context, params binding.CollectionParams", false, true, true, false},
		{"manual route owns adaptation", "ctx ctxpkg.Context, params binding.CollectionParams", true, false, true, false},
		{"frontend only", "ctx ctxpkg.Context, params binding.CollectionParams", true, true, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			cfg := testConfig(t)
			cfg.ServerRoot = t.TempDir()
			cfg.BackendEnabled = test.backend
			// This override must never hide a detectable binder/signature mismatch.
			cfg.AllowManualCollisions = true
			file := filepath.Join(cfg.ServerRoot, "internal", "services", "catalogueItem.go")
			if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
				t.Fatal(err)
			}
			body := "package services\nimport ctxpkg \"context\"\nimport binding \"github.com/dronm/modelbind\"\nfunc (s *CatalogueItemService) List(" + test.parameters + ") {}\n"
			if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			view := ObjectView{
				Name:              "CatalogueItem",
				SourceFile:        "catalogue-items.yaml",
				GroupedFilters:    test.grouped,
				GeneratedHTTPCRUD: CRUDSpec{List: test.generatedRoute},
				ManualServiceCRUD: CRUDSpec{List: true},
			}
			err := validateGroupedCollectionServices(cfg, []ObjectView{view})
			if (err != nil) != test.wantError {
				t.Fatalf("error = %v, want error %v", err, test.wantError)
			}
			if err != nil {
				for _, expected := range []string{"catalogue-items.yaml", "collection.groupedFilters", file, "modelbind.CollectionParamsWithWhere", "FetchCollectionModelWhere"} {
					if !strings.Contains(err.Error(), expected) {
						t.Errorf("diagnostic %q missing %q", err, expected)
					}
				}
			}
		})
	}
}

func TestGroupedCollectionManualOwnershipRemainsIndependent(t *testing.T) {
	t.Parallel()

	for _, manualService := range []bool{false, true} {
		for _, manualRoute := range []bool{false, true} {
			spec := compositeObjectSpec()
			if manualService {
				spec.Service.ManualMethods = []string{"list"}
			}
			if manualRoute {
				spec.HTTPRoutes.ManualMethods = []string{"list"}
			}
			cfg := testConfig(t)
			view, err := BuildObjectView(cfg, spec)
			if err != nil {
				t.Fatal(err)
			}
			view.GroupedFilters = true
			renderer := NewRenderer(cfg)
			service, err := renderer.executeTemplate(filepath.Join("go", "service.go.tmpl"), view)
			if err != nil {
				t.Fatal(err)
			}
			routes, err := renderer.executeTemplate(filepath.Join("go", "httpapi.go.tmpl"), view)
			if err != nil {
				t.Fatal(err)
			}
			if hasList := strings.Contains(string(service), "func (s *CatalogueItemService) List("); hasList == manualService {
				t.Errorf("manual service=%v manual route=%v: List ownership was not preserved", manualService, manualRoute)
			}
			if hasBinder := strings.Contains(string(routes), "CollectionParamsWithWhereBinder"); hasBinder == manualRoute {
				t.Errorf("manual service=%v manual route=%v: HTTP ownership was not preserved", manualService, manualRoute)
			}
		}
	}
}

func TestGroupedManualListValidationRunsBeforeRendering(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	cfg.ServerRoot = t.TempDir()
	cfg.AllowManualCollisions = true
	spec := compositeObjectSpec()
	spec.Collection.GroupedFilters = boolPointer(true)
	spec.Service.ManualMethods = []string{"list"}
	view, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatal(err)
	}
	manualFile := filepath.Join(cfg.ServerRoot, "internal", "services", "catalogueItem.go")
	if err := os.MkdirAll(filepath.Dir(manualFile), 0o755); err != nil {
		t.Fatal(err)
	}
	manual := "package services\nimport \"context\"\nimport \"github.com/dronm/modelbind\"\nfunc (s *CatalogueItemService) List(ctx context.Context, params modelbind.CollectionParams) {}\n"
	if err := os.WriteFile(manualFile, []byte(manual), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := NewRenderer(cfg).RenderObject(view); err == nil || !strings.Contains(err.Error(), "incompatible input") {
		t.Fatalf("RenderObject accepted the stale manual List contract: %v", err)
	}
	generated := filepath.Join(cfg.ServerRoot, "internal", "services", "catalogueItem.gen.go")
	if _, err := os.Stat(generated); !os.IsNotExist(err) {
		t.Fatalf("generated output was written before signature validation: %v", err)
	}
	if actual := readTestFile(t, manualFile); actual != manual {
		t.Fatal("signature validation modified the manually owned service")
	}

	manual = strings.ReplaceAll(manual, "modelbind.CollectionParams)", "modelbind.CollectionParamsWithWhere)")
	if err := os.WriteFile(manualFile, []byte(manual), 0o644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := NewRenderer(cfg).RenderObject(view); err != nil {
			t.Fatal(err)
		}
	}
	cfg.Check = true
	if err := NewRenderer(cfg).RenderObject(view); err != nil {
		t.Fatalf("check after repeated generation: %v", err)
	}
	if actual := readTestFile(t, manualFile); actual != manual {
		t.Fatal("generation/check modified the manually owned List method")
	}
	if strings.Contains(readTestFile(t, generated), "func (s *CatalogueItemService) List(") {
		t.Fatal("manually owned List was regenerated")
	}
}

func TestGroupedManualListLocalTypeAliases(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		declaration string
		wantError   bool
	}{
		{"grouped alias", "type ListParams = mb.CollectionParamsWithWhere", false},
		{"grouped alias chain", "type Original = mb.CollectionParamsWithWhere\ntype ListParams = Original", false},
		{"legacy alias", "type ListParams = mb.CollectionParams", true},
		{"legacy alias chain", "type Original = mb.CollectionParams\ntype ListParams = Original", true},
		{"distinct defined type", "type ListParams mb.CollectionParamsWithWhere", true},
		{"pointer alias", "type ListParams = *mb.CollectionParamsWithWhere", true},
		{"cyclic alias", "type ListParams = Other\ntype Other = ListParams", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			cfg := testConfig(t)
			cfg.ServerRoot = t.TempDir()
			dir := filepath.Join(cfg.ServerRoot, "internal", "services")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			declarations := "package services\nimport c \"context\"\nimport mb \"github.com/dronm/modelbind\"\ntype RequestContext = c.Context\n" + test.declaration + "\n"
			method := "package services\nfunc (s *ExampleService) List(ctx RequestContext, params ListParams) {}\n"
			for file, content := range map[string]string{"aliases.go": declarations, "example.go": method} {
				if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			view := ObjectView{
				Name:              "Example",
				GroupedFilters:    true,
				GeneratedHTTPCRUD: CRUDSpec{List: true},
				ManualServiceCRUD: CRUDSpec{List: true},
			}
			err := validateGroupedCollectionServices(cfg, []ObjectView{view})
			if (err != nil) != test.wantError {
				t.Fatalf("error = %v, want error %v", err, test.wantError)
			}
			if test.wantError && !strings.Contains(err.Error(), "incompatible input") {
				t.Fatalf("unexpected diagnostic: %v", err)
			}
		})
	}
}
