package codegen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func detailObjectSpec() ObjectSpec {
	spec := compositeObjectSpec()
	spec.Detail = &ListSpec{
		Model: "CatalogueItemDetail",
		Table: TableSpec{Schema: "reporting", Name: "catalogue_items_detail"},
		Fields: append(append([]FieldSpec(nil), spec.Fields...),
			FieldSpec{Name: "reference", Type: "jsonb", Nullable: true},
			FieldSpec{Name: "received_at", Type: "timestamptz", Nullable: true},
		),
	}
	return spec
}

func TestDetailProjectionGeneration(t *testing.T) {
	for _, customList := range []bool{false, true} {
		t.Run(map[bool]string{false: "base list", true: "custom list"}[customList], func(t *testing.T) {
			cfg := testConfig(t)
			cfg.ServerRoot = t.TempDir()
			cfg.FrontendRoot = filepath.Join(cfg.ServerRoot, "front")
			cfg.FrontendEnabled = true
			spec := detailObjectSpec()
			if !customList {
				spec.List = nil
			}
			// Exercise the strict YAML loader as well as view building and rendering.
			schemaDir := t.TempDir()
			content := `{"name":"Simple","table":{"name":"simple"},"route":"/simple","keys":[{"name":"id","type":"int"}],"fields":[{"name":"id","type":"int","primaryKey":true}],"crud":{"detail":true},"detail":{"model":"SimpleDetail","table":{"name":"simple_detail"},"fields":[{"name":"id","type":"int"}]}}`
			if !json.Valid([]byte(content)) {
				t.Fatal("invalid fixture")
			}
			if err := os.WriteFile(filepath.Join(schemaDir, "simple.yaml"), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadObjects(schemaDir); err != nil {
				t.Fatal(err)
			}
			view, err := BuildObjectView(cfg, spec)
			if err != nil {
				t.Fatal(err)
			}
			if err := NewRenderer(cfg).RenderObject(view); err != nil {
				t.Fatal(err)
			}
			checks := map[string][]string{
				"internal/models/catalogueItem.gen.go":         {"type CatalogueItemDetail struct", `catalogueItemDetailRelation = "reporting.catalogue_items_detail"`, "ReceivedAt *time.Time"},
				"internal/services/catalogueItem.gen.go":       {"(*models.CatalogueItemDetail, error)", "&models.CatalogueItemDetail{}", "key models.CatalogueItemKey", "modelbind.ModelInput[*models.CatalogueItem]", "webapp.UpdateByKeysInput[*models.CatalogueItemKey, *models.CatalogueItem]"},
				"front/src/types/catalogueItemDetail.gen.ts":   {"export interface CatalogueItemDetailDTO", "reference: Record<string, unknown> | null;", "received_at: Date | null;"},
				"front/src/schemas/catalogueItemDetail.gen.ts": {"export const catalogueItemDetailFromDTO", "new Date(parsedDTO.received_at)"},
				"front/src/api/catalogueItem.gen.ts":           {"fromDetailDTO: catalogueItemDetailFromDTO", "Promise<CatalogueItemDetail>", "api.get<CatalogueItemDetailDTO>", "return catalogueItemDetailFromDTO(response);"},
			}
			for path, expected := range checks {
				output := readTestFile(t, filepath.Join(cfg.ServerRoot, path))
				for _, value := range expected {
					// gofmt aligns struct fields.
					if !strings.Contains(strings.Join(strings.Fields(output), " "), strings.Join(strings.Fields(value), " ")) {
						t.Errorf("%s missing %q", path, value)
					}
				}
			}
			base := readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/types/catalogueItem.gen.ts"))
			if strings.Contains(base, "reference:") {
				t.Fatal("detail-only field leaked into mutation types")
			}
		})
	}
}

func TestDetailProjectionValidation(t *testing.T) {
	cases := []struct {
		name   string
		change func(*ObjectSpec)
		want   string
	}{
		{"missing model", func(s *ObjectSpec) { s.Detail.Model = "" }, "detail.model is required"},
		{"base collision", func(s *ObjectSpec) { s.Detail.Model = s.Name }, "must differ from the base"},
		{"list collision", func(s *ObjectSpec) { s.Detail.Model = s.List.Model }, "must differ from list.model"},
		{"key model collision", func(s *ObjectSpec) { s.Detail.Model = s.Name + "Key" }, "conflicts with a generated"},
		{"missing relation", func(s *ObjectSpec) { s.Detail.Table.Name = "" }, "detail.table.name"},
		{"empty fields", func(s *ObjectSpec) { s.Detail.Fields = nil }, "detail.fields"},
		{"duplicate fields", func(s *ObjectSpec) { s.Detail.Fields = append(s.Detail.Fields, s.Detail.Fields[0]) }, "detail: duplicate field"},
		{"missing key", func(s *ObjectSpec) { s.Detail.Fields = s.Detail.Fields[1:] }, "must include key field"},
		{"wrong key type", func(s *ObjectSpec) { s.Detail.Fields[0].Type = "int" }, "must have Go type"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := detailObjectSpec()
			tc.change(&spec)
			_, err := BuildObjectView(testConfig(t), spec)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
}

func TestDetailProjectionDefault(t *testing.T) {
	view, err := BuildObjectView(testConfig(t), compositeObjectSpec())
	if err != nil {
		t.Fatal(err)
	}
	if view.HasCustomDetail || view.DetailModelName != view.Name || view.DetailRelation != view.Relation {
		t.Fatal("omitted detail must use the base model and relation")
	}
}

func TestDetailProjectionScaffold(t *testing.T) {
	cfg := testConfig(t)
	cfg.ServerRoot = t.TempDir()
	cfg.FrontendRoot = filepath.Join(cfg.ServerRoot, "front")
	cfg.FrontendEnabled = true
	spec := detailObjectSpec()
	spec.Keys = spec.Keys[:1]
	spec.Fields[1].PrimaryKey = false
	spec.Detail.Fields[1].PrimaryKey = false
	spec.Frontend.Scaffold = true
	spec.ApplicationRoute = ApplicationRouteSpec{Enabled: true, Name: "catalogueItems", Path: "/catalogue-items", Description: "Items", Section: "Catalogues"}
	spec.Frontend.DetailTypeImports = []FrontendImportSpec{{From: "@/types/reference", Names: []string{"Reference"}, TypeOnly: true}}
	spec.Detail.Fields[5].TSType = "Reference | null"
	spec.Detail.Fields[5].TSDTOType = "Reference | null"
	view, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := NewRenderer(cfg).RenderObject(view); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"src/collections/catalogueItem.gen.ts", "src/views/catalogueItem/CatalogueItemEditPage.vue"} {
		output := readTestFile(t, filepath.Join(cfg.FrontendRoot, path))
		if !strings.Contains(output, `from "@/types/catalogueItemDetail.gen"`) || !strings.Contains(output, "CatalogueItemDetail\n>(") {
			t.Errorf("%s does not use the detail projection", path)
		}
	}
	types := readTestFile(t, filepath.Join(cfg.FrontendRoot, "src/types/catalogueItemDetail.gen.ts"))
	if !strings.Contains(types, `from "@/types/reference"`) {
		t.Fatal("detail imports missing")
	}
	spec.Detail.Fields[2].TSType = "number"
	if _, err := BuildObjectView(cfg, spec); err == nil || !strings.Contains(err.Error(), "compatible detail field name") {
		t.Fatalf("expected form compatibility error, got %v", err)
	}
}
