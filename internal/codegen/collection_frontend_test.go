package codegen

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func infiniteFrontendSpec() ObjectSpec {
	return ObjectSpec{
		Name: "StockItem", Table: TableSpec{Name: "stock_items"}, Route: "/stock-items",
		Keys: []KeySpec{{Name: "sku_code", Type: "string"}},
		Fields: []FieldSpec{
			{Name: "sku_code", JSONName: "code", Type: "string", PrimaryKey: true, Required: true},
			{Name: "name", Type: "string", Required: true},
		},
		CRUD:             CRUDSpec{List: true, Detail: true},
		ApplicationRoute: ApplicationRouteSpec{Enabled: true, Name: "stockItems", Path: "/stock-items", Description: "Stock items", Section: "Catalogue"},
		Frontend: FrontendSpec{
			Scaffold: true,
			Form:     FrontendFormSpec{Enabled: boolPointer(false)},
			List: FrontendListSpec{
				PageSize:       73,
				InfiniteScroll: &FrontendInfiniteScrollSpec{Enabled: true},
				Columns: []FrontendListColumnSpec{
					{Field: "name", SortField: "normalized_name"},
				},
			},
		},
	}
}

func infiniteFrontendConfig(t *testing.T) Config {
	t.Helper()
	cfg := testConfig(t)
	cfg.ServerRoot = t.TempDir()
	cfg.FrontendRoot = filepath.Join(cfg.ServerRoot, "front")
	cfg.SchemaDir = filepath.Join(cfg.ServerRoot, "schema")
	cfg.BackendEnabled = false
	cfg.FrontendEnabled = true
	cfg.APITestEnabled = false
	return cfg
}

func TestInfiniteFrontendRenderingAndPaginatedCompatibility(t *testing.T) {
	cfg := infiniteFrontendConfig(t)
	spec := infiniteFrontendSpec()
	view, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatal(err)
	}
	renderer := NewRenderer(cfg)
	if err := renderer.RenderObject(view); err != nil {
		t.Fatal(err)
	}
	collectionPath := filepath.Join(cfg.FrontendRoot, "src/collections/stockItem.gen.ts")
	collection := readTestFile(t, collectionPath)
	for _, expected := range []string{
		`infiniteScroll: {`, `keyFields: ["code"]`, `chunkSize: 100`,
		`prefetchRows: 30`, `maxCachedChunks: 5`, `scrollHeight: "32rem"`,
		`itemSize: 40`, `sortField: "normalized_name"`, `pageSize: 73`,
	} {
		if !strings.Contains(collection, expected) {
			t.Fatalf("missing %q in collection:\n%s", expected, collection)
		}
	}
	list := readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/views/stockItem/StockItemList.vue"))
	if !strings.Contains(list, `CollectionListPage :collection="stockItemCollection"`) || strings.Contains(list, "infiniteScroll") {
		t.Fatal("scroll options must remain in the managed collection definition", list)
	}

	zero := 0
	itemSize := 41.5
	spec.Frontend.List.InfiniteScroll.PrefetchRows = &zero
	spec.Frontend.List.InfiniteScroll.MaxCachedChunks = &zero
	spec.Frontend.List.InfiniteScroll.ItemSize = &itemSize
	view, err = BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := renderer.RenderObject(view); err != nil {
		t.Fatal(err)
	}
	collection = readTestFile(t, collectionPath)
	for _, expected := range []string{`prefetchRows: 0`, `maxCachedChunks: 0`, `itemSize: 41.5`} {
		if !strings.Contains(collection, expected) {
			t.Fatalf("explicit value lost: %s", expected)
		}
	}

	spec.Frontend.List.InfiniteScroll = nil
	view, err = BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := renderer.RenderObject(view); err != nil {
		t.Fatal(err)
	}
	paginated := readTestFile(t, collectionPath)
	paginatedAPI := readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/api/stockItem.gen.ts"))
	if strings.Contains(paginated, "infiniteScroll") || !strings.Contains(paginated, "pageSize: 73") {
		t.Fatal("paginated configuration changed", paginated)
	}
	spec.Frontend.List.InfiniteScroll = &FrontendInfiniteScrollSpec{Enabled: false}
	view, err = BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := renderer.RenderObject(view); err != nil {
		t.Fatal(err)
	}
	if readTestFile(t, collectionPath) != paginated || readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/api/stockItem.gen.ts")) != paginatedAPI {
		t.Fatal("disabled options must generate byte-identical paginated definitions and API")
	}
}

func TestInfiniteFrontendDependencyMinimumAndLocalPackages(t *testing.T) {
	cfg := infiniteFrontendConfig(t)
	view, err := BuildObjectView(cfg, infiniteFrontendSpec())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cfg.FrontendRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(cfg.FrontendRoot, "package.json")
	writeManifest := func(version string) {
		t.Helper()
		content, _ := json.Marshal(map[string]any{"dependencies": map[string]string{"@katren/vue-collection-lib": version}})
		if err := os.WriteFile(manifest, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, version := range []string{"^0.1.15", "0.1.15", "~0.1.16", ">=0.1.15", "^0.2.0", "^1.0.0", "workspace:^0.1.15", "^0.1.14", "*", "latest", "0.1.15-beta.1"} {
		writeManifest(version)
		before := readTestFile(t, manifest)
		err := validateEnumCollectionDependency(cfg.FrontendRoot, []ObjectView{view})
		expected := collectionLibraryRangeCompatible(strings.TrimPrefix(version, "workspace:"), MinInfiniteCollectionLibraryVersion)
		if (err == nil) != expected {
			t.Fatalf("%s: %v", version, err)
		}
		if readTestFile(t, manifest) != before {
			t.Fatal("application dependencies were modified")
		}
	}

	for _, protocol := range []string{"file:", "link:", "workspace:"} {
		t.Run(protocol, func(t *testing.T) {
			var localRoot string
			if protocol == "workspace:" {
				writeManifest("workspace:*")
				localRoot = filepath.Join(cfg.FrontendRoot, "node_modules/@katren/vue-collection-lib")
			} else {
				writeManifest(protocol + "../collection-lib")
				localRoot = filepath.Join(cfg.ServerRoot, "collection-lib")
			}
			if err := os.MkdirAll(localRoot, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, version := range []string{"0.1.14", "0.1.15"} {
				if err := os.WriteFile(filepath.Join(localRoot, "package.json"), []byte(`{"version":"`+version+`"}`), 0o644); err != nil {
					t.Fatal(err)
				}
				err := validateEnumCollectionDependency(cfg.FrontendRoot, []ObjectView{view})
				if (err == nil) != (version == "0.1.15") {
					t.Fatalf("local version %s: %v", version, err)
				}
			}
		})
	}
}

func TestInfiniteFrontendGenerationPreservesScaffoldsAndCheck(t *testing.T) {
	cfg := infiniteFrontendConfig(t)
	if err := os.MkdirAll(cfg.SchemaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cfg.FrontendRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	spec := infiniteFrontendSpec()
	writeSpec := func() {
		t.Helper()
		content, err := yaml.Marshal(spec)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cfg.SchemaDir, "stock-items.yaml"), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(cfg.FrontendRoot, "package.json"), []byte(`{"dependencies":{"@katren/vue-collection-lib":"^0.1.15"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// First generate a paginated project, then customize its create-once page.
	spec.Frontend.List.InfiniteScroll.Enabled = false
	writeSpec()
	if err := Generate(cfg); err != nil {
		t.Fatal(err)
	}
	listPath := filepath.Join(cfg.FrontendRoot, "src/views/stockItem/StockItemList.vue")
	manual := readTestFile(t, listPath) + "\n<!-- application customization -->\n"
	if err := os.WriteFile(listPath, []byte(manual), 0o644); err != nil {
		t.Fatal(err)
	}
	spec.Frontend.List.InfiniteScroll.Enabled = true
	writeSpec()
	for range 2 {
		if err := Generate(cfg); err != nil {
			t.Fatal(err)
		}
	}
	if readTestFile(t, listPath) != manual {
		t.Fatal("enabling infinite scrolling overwrote a customized scaffold")
	}
	cfg.Check = true
	if err := Generate(cfg); err != nil {
		t.Fatal(err)
	}
	if err := Validate(cfg); err != nil {
		t.Fatal(err)
	}
}

// Run with CODEGEN_COLLECTION_LIB_ROOT pointing to an installed v0.1.15+
// checkout. This tests generated contracts against the real library, including
// its CollectionParams serializer; application-owned adapter code is a fixture.
func TestInfiniteGeneratedTypeScriptAndWhereForwarding(t *testing.T) {
	libraryRoot := os.Getenv("CODEGEN_COLLECTION_LIB_ROOT")
	if libraryRoot == "" {
		t.Skip("set CODEGEN_COLLECTION_LIB_ROOT to an installed collection library checkout")
	}
	libraryRoot, err := filepath.Abs(libraryRoot)
	if err != nil {
		t.Fatal(err)
	}
	cfg := infiniteFrontendConfig(t)
	spec := infiniteFrontendSpec()
	spec.List = &ListSpec{
		Model: "StockItemList", Table: TableSpec{Name: "stock_items_list"},
		Fields: []FieldSpec{
			{Name: "sku_code", JSONName: "code", Type: "string", Required: true},
			{Name: "name", Type: "string", Required: true},
		},
	}
	view, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := NewRenderer(cfg).RenderObject(view); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(libraryRoot, "node_modules"), filepath.Join(cfg.FrontendRoot, "node_modules")); err != nil {
		t.Fatal(err)
	}
	fixtureRoot := filepath.Join("testdata", "collection-frontend")
	for _, name := range []string{"common.ts", "createCrudApi.ts", "http.ts", "library.ts", "runtime.ts"} {
		content, err := os.ReadFile(filepath.Join(fixtureRoot, name))
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(cfg.FrontendRoot, name)
		if name == "common.ts" {
			target = filepath.Join(cfg.FrontendRoot, "src/schemas/common.ts")
		} else if name == "createCrudApi.ts" || name == "http.ts" {
			target = filepath.Join(cfg.FrontendRoot, "src/api", name)
		}
		if err := os.WriteFile(target, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tsconfig := map[string]any{
		"compilerOptions": map[string]any{
			"target": "ES2022", "module": "ESNext", "moduleResolution": "Bundler", "strict": true,
			"skipLibCheck": true, "noEmit": true,
			"paths": map[string]any{
				"@/*":                          []string{"./src/*"},
				"@katren/vue-collection-lib":   []string{"./library.ts"},
				"@katren/vue-collection-lib/*": []string{filepath.ToSlash(filepath.Join(libraryRoot, "src/*"))},
			},
		},
		"include": []string{"src/types/*.ts", "src/schemas/*.ts", "src/api/*.ts", "src/collections/*.ts", "library.ts", "runtime.ts"},
	}
	configData, _ := json.MarshalIndent(tsconfig, "", "\t")
	if err := os.WriteFile(filepath.Join(cfg.FrontendRoot, "tsconfig.json"), configData, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{filepath.Join(libraryRoot, "node_modules/typescript/bin/tsc"), "-p", "tsconfig.json"},
		{"--import", filepath.Join(libraryRoot, "node_modules/tsx/dist/loader.mjs"), "runtime.ts"},
	} {
		cmd := exec.Command("node", args...)
		cmd.Dir = cfg.FrontendRoot
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, output)
		}
	}
}
