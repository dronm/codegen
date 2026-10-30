package codegen

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCollectionConfigurationOptIn(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		change  func(*ObjectSpec)
		grouped bool
		want    string
	}{
		{"automatic", func(*ObjectSpec) {}, true, ""},
		{"omitted", func(s *ObjectSpec) { s.Frontend.List.InfiniteScroll = nil }, false, ""},
		{"disabled", func(s *ObjectSpec) { s.Frontend.List.InfiniteScroll.Enabled = false }, false, ""},
		{"backend only", func(s *ObjectSpec) {
			s.Frontend = FrontendSpec{}
			s.Collection.GroupedFilters = boolPointer(true)
		}, true, ""},
		{"explicit grouped false", func(s *ObjectSpec) { s.Collection.GroupedFilters = boolPointer(false) }, false, "collection.groupedFilters"},
		{"no list infinite", func(s *ObjectSpec) { s.CRUD.List = false }, false, "frontend.list.infiniteScroll.enabled"},
		{"no list grouped", func(s *ObjectSpec) {
			s.Frontend.List.InfiniteScroll = nil
			s.Collection.GroupedFilters = boolPointer(true)
			s.CRUD.List = false
		}, false, "collection.groupedFilters"},
		{"no scaffold", func(s *ObjectSpec) { s.Frontend.Scaffold = false }, false, "frontend.scaffold"},
		{"disabled frontend list", func(s *ObjectSpec) { s.Frontend.List.Enabled = boolPointer(false) }, false, "frontend.list.enabled"},
		{"manual route implicit", func(s *ObjectSpec) { s.HTTPRoutes.ManualMethods = []string{"list"} }, false, "explicit collection.groupedFilters: true"},
		{"manual route declared", func(s *ObjectSpec) {
			s.HTTPRoutes.ManualMethods = []string{"list"}
			s.Collection.GroupedFilters = boolPointer(true)
		}, true, ""},
		{"external route implicit", func(s *ObjectSpec) { s.HTTPRoutes.Enabled = boolPointer(false) }, false, "explicit collection.groupedFilters: true"},
		{"external route declared", func(s *ObjectSpec) {
			s.HTTPRoutes.Enabled = boolPointer(false)
			s.Collection.GroupedFilters = boolPointer(true)
		}, true, ""},
		{"manual service automatic", func(s *ObjectSpec) { s.Service.ManualMethods = []string{"list"} }, true, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			spec := infiniteFrontendSpec()
			test.change(&spec)
			view, err := BuildObjectView(testConfig(t), spec)
			if test.want != "" {
				if err == nil || !strings.Contains(err.Error(), test.want) {
					t.Fatalf("wanted %q, got %v", test.want, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if view.GroupedFilters != test.grouped {
				t.Fatalf("GroupedFilters = %v, want %v", view.GroupedFilters, test.grouped)
			}
		})
	}
}

func TestCollectionOptionsValidationNamesSchemaAndProperty(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ property, value, want string }{
		{"chunkSize", "0", "chunkSize"},
		{"chunkSize", "-1", "chunkSize"},
		{"chunkSize", "1.5", "chunkSize"},
		{"chunkSize", "not-a-number", "chunkSize"},
		{"chunkSize", "99999999999999999999999999", "chunkSize"},
		{"prefetchRows", "-1", "prefetchRows"},
		{"prefetchRows", "0.5", "prefetchRows"},
		{"maxCachedChunks", "-1", "maxCachedChunks"},
		{"maxCachedChunks", "false", "maxCachedChunks"},
		{"itemSize", "0", "itemSize"},
		{"itemSize", "-2", "itemSize"},
		{"itemSize", ".inf", "itemSize"},
		{"itemSize", ".nan", "itemSize"},
		{"itemSize", "big", "itemSize"},
		{"scrollHeight", "40", "scrollHeight"},
		{"scrollHeight", "'   '", "scrollHeight"},
		{"chunkSiz", "10", "chunkSiz"},
	} {
		t.Run(test.property+"="+test.value, func(t *testing.T) {
			spec := infiniteFrontendSpec()
			// Test parsing and validation through the same schema file path used
			// by generate/check/validate, including disabled configuration.
			spec.Frontend = FrontendSpec{}
			data, err := yaml.Marshal(spec)
			if err != nil {
				t.Fatal(err)
			}
			// Preserve the strict decoder: replace the old frontend section by
			// constructing a YAML node rather than adding duplicate properties.
			var object map[string]any
			if err := yaml.Unmarshal(data, &object); err != nil {
				t.Fatal(err)
			}
			delete(object, "frontend")
			data, err = yaml.Marshal(object)
			if err != nil {
				t.Fatal(err)
			}
			body := string(data) + "frontend:\n  list:\n    infiniteScroll:\n      enabled: false\n      " + test.property + ": " + test.value + "\n"
			dir := t.TempDir()
			file := filepath.Join(dir, "invalid-scroll.yaml")
			if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err = LoadObjects(dir)
			if err == nil || !strings.Contains(err.Error(), file) || !strings.Contains(err.Error(), "frontend.list.infiniteScroll."+test.want) {
				t.Fatalf("error must identify schema and property, got %v", err)
			}
		})
	}
	// Programmatically supplied non-finite values follow the same checks.
	for _, value := range []float64{math.Inf(-1), math.NaN()} {
		spec := infiniteFrontendSpec()
		spec.Frontend.List.InfiniteScroll.ItemSize = &value
		if err := spec.Validate(); err == nil || !strings.Contains(err.Error(), "itemSize") {
			t.Fatalf("accepted non-finite itemSize %v: %v", value, err)
		}
	}
}

func TestInfiniteCursorProjectionValidation(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		change func(*ObjectSpec)
		want   string
	}{
		{"custom projection", func(*ObjectSpec) {}, ""},
		{"missing", func(s *ObjectSpec) { s.List.Fields = s.List.Fields[1:] }, "cursor key code"},
		{"hidden", func(s *ObjectSpec) {
			s.List.Fields[0].JSON = boolPointer(false)
			s.List.Fields[0].JSONName = ""
		}, "cursor key code"},
		{"renamed", func(s *ObjectSpec) { s.List.Fields[0].JSONName = "other_code" }, "cursor key code"},
		{"optional", func(s *ObjectSpec) { s.List.Fields[0].JSONOmitEmpty = true }, "non-null key"},
		{"nullable", func(s *ObjectSpec) {
			s.List.Fields[0].Nullable = true
			s.List.Fields[0].Required = false
		}, "cursor key code"},
		{"wrong go type", func(s *ObjectSpec) { s.List.Fields[0].GoType = "int" }, "preserve the base key"},
		{"wrong model type", func(s *ObjectSpec) { s.List.Fields[0].TSType = "string | null" }, "preserve the base key"},
		{"wrong dto type", func(s *ObjectSpec) { s.List.Fields[0].TSDTOType = "number" }, "preserve the base key"},
		{"optional dto validator", func(s *ObjectSpec) { s.List.Fields[0].ValibotDTO = "v.optional(v.string())" }, "validation schemas"},
		{"nullish model validator", func(s *ObjectSpec) { s.List.Fields[0].ValibotModel = "v.nullish(v.string())" }, "validation schemas"},
		{"optional base key", func(s *ObjectSpec) { s.Fields[0].JSONOmitEmpty = true }, "fields model, DTO and validation schema"},
		{"incompatible base key type", func(s *ObjectSpec) { s.Fields[0].TSType = "string | undefined" }, "fields model and DTO"},
		{"optional base key schema", func(s *ObjectSpec) { s.Fields[0].ValibotDTO = "v.optional(v.string())" }, "fields DTO and model validation schemas"},
		{"composite restriction", func(s *ObjectSpec) {
			s.Fields[1].PrimaryKey = true
			s.Keys = append(s.Keys, KeySpec{Name: "name", Type: "string"})
		}, "require a single key"},
	} {
		t.Run(test.name, func(t *testing.T) {
			spec := infiniteFrontendSpec()
			spec.SourceFile = "cursor-projection.yaml"
			spec.List = &ListSpec{Model: "StockItemList", Table: TableSpec{Name: "stock_items_list"}, Fields: append([]FieldSpec(nil), spec.Fields...)}
			spec.List.Fields[0].PrimaryKey = false
			test.change(&spec)
			view, err := BuildObjectView(testConfig(t), spec)
			if test.want != "" {
				if err == nil || !strings.Contains(err.Error(), test.want) {
					t.Fatalf("wanted %q, got %v", test.want, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := view.Frontend.List.InfiniteScroll.KeyFields; len(got) != 1 || got[0] != "code" {
				t.Fatalf("wrong cursor key names: %v", got)
			}
		})
	}
}
