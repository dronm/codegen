package codegen

import (
	"fmt"
	"math"
	"strings"

	"gopkg.in/yaml.v3"
)

// Decode these settings strictly so YAML's permissive float-to-int conversion
// cannot turn a fractional row count into a different, apparently valid value.
// Checking every property also preserves KnownFields inside this custom decoder.
func (s *FrontendInfiniteScrollSpec) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("frontend.list.infiniteScroll must be an object")
	}
	seen := make(map[string]bool)
	for i := 0; i < len(node.Content); i += 2 {
		name, value := node.Content[i].Value, node.Content[i+1]
		property := "frontend.list.infiniteScroll." + name
		if seen[name] {
			return fmt.Errorf("%s is duplicated", property)
		}
		seen[name] = true
		var target any
		switch name {
		case "enabled":
			target = &s.Enabled
		case "chunkSize", "prefetchRows", "maxCachedChunks":
			if value.ShortTag() != "!!int" && value.ShortTag() != "!!null" {
				return fmt.Errorf("%s must be an integer", property)
			}
			switch name {
			case "chunkSize":
				target = &s.ChunkSize
			case "prefetchRows":
				target = &s.PrefetchRows
			default:
				target = &s.MaxCachedChunks
			}
		case "itemSize":
			if value.ShortTag() != "!!int" && value.ShortTag() != "!!float" && value.ShortTag() != "!!null" {
				return fmt.Errorf("%s must be a positive finite number", property)
			}
			target = &s.ItemSize
		case "scrollHeight":
			if value.ShortTag() != "!!str" {
				return fmt.Errorf("%s must be a CSS height string", property)
			}
			target = &s.ScrollHeight
		default:
			return fmt.Errorf("unknown property %s", property)
		}
		if err := value.Decode(target); err != nil {
			return fmt.Errorf("%s: %w", property, err)
		}
	}
	return nil
}

func (s ObjectSpec) infiniteScrollEnabled() bool {
	return s.Frontend.List.InfiniteScroll != nil && s.Frontend.List.InfiniteScroll.Enabled
}

func (s ObjectSpec) groupedCollectionFilters() bool {
	return boolOrDefault(s.Collection.GroupedFilters, s.infiniteScrollEnabled())
}

func validateCollectionSpec(spec ObjectSpec) error {
	infinite := spec.infiniteScrollEnabled()
	if boolOrDefault(spec.Collection.GroupedFilters, false) && !spec.CRUD.List {
		return fmt.Errorf("collection.groupedFilters requires crud.list: true")
	}
	if options := spec.Frontend.List.InfiniteScroll; options != nil {
		for _, setting := range []struct {
			name  string
			value *int
			min   int
		}{
			{"chunkSize", options.ChunkSize, 1},
			{"prefetchRows", options.PrefetchRows, 0},
			{"maxCachedChunks", options.MaxCachedChunks, 0},
		} {
			if setting.value != nil && *setting.value < setting.min {
				return fmt.Errorf("frontend.list.infiniteScroll.%s must be at least %d", setting.name, setting.min)
			}
		}
		if options.ItemSize != nil && (math.IsNaN(*options.ItemSize) || math.IsInf(*options.ItemSize, 0) || *options.ItemSize <= 0) {
			return fmt.Errorf("frontend.list.infiniteScroll.itemSize must be a positive finite number")
		}
		if options.ScrollHeight != "" && strings.TrimSpace(options.ScrollHeight) == "" {
			return fmt.Errorf("frontend.list.infiniteScroll.scrollHeight must be a non-empty CSS height")
		}
	}
	if !infinite {
		return nil
	}
	if !spec.CRUD.List {
		return fmt.Errorf("frontend.list.infiniteScroll.enabled requires crud.list: true")
	}
	if spec.Collection.GroupedFilters != nil && !*spec.Collection.GroupedFilters {
		return fmt.Errorf("collection.groupedFilters: false conflicts with frontend.list.infiniteScroll.enabled: true")
	}
	if !spec.Frontend.Scaffold || !boolOrDefault(spec.Frontend.List.Enabled, true) {
		return fmt.Errorf("frontend.list.infiniteScroll.enabled requires frontend.scaffold: true and frontend.list.enabled: true")
	}
	// A manual/externally registered route is a capability declaration, not
	// evidence that its binder and implementation were converted by codegen.
	manualRoute := !boolOrDefault(spec.HTTPRoutes.Enabled, true) || manualHTTPCRUD(spec.HTTPRoutes.ManualMethods).List
	if manualRoute && !boolOrDefault(spec.Collection.GroupedFilters, false) {
		return fmt.Errorf("frontend.list.infiniteScroll.enabled with a manually owned list route requires explicit collection.groupedFilters: true; migrate its binder, List argument and fetch helper to the grouped APIs")
	}
	return nil
}

func validateInfiniteScrollCursor(object ObjectView) error {
	if object.Frontend.List.InfiniteScroll == nil {
		return nil
	}
	for _, key := range object.FrontendKeys {
		fail := func(reason string) error {
			return fmt.Errorf("%s: frontend.list.infiniteScroll cursor key %s: %s", sourceLabel(object.SourceFile), key.JSONName, reason)
		}
		// Key types are Pick<BaseModel, ...>, even with a custom list model.
		// Validate both projections so a mandatory list key cannot mask an
		// optional or incompatible base key in the generated frontend API.
		for _, projection := range []struct {
			name   string
			fields []FieldView
		}{
			{"fields", object.Fields},
			{"list.fields", object.ListFields},
		} {
			var cursor *FieldView
			for i := range projection.fields {
				field := &projection.fields[i]
				if field.JSONName == key.JSONName && field.JSONEnabled {
					cursor = field
					break
				}
			}
			if cursor == nil {
				return fail(projection.name + " must include the JSON-visible key field (including custom list projections)")
			}
			if cursor.GoType != key.GoType || cursor.TSType != key.TSType || cursor.TSDTOType != key.TSType {
				return fail(projection.name + " model and DTO must preserve the base key's Go and TypeScript types")
			}
			if cursor.Nullable || cursor.TSOptional || strings.HasPrefix(cursor.GoType, "*") {
				return fail(projection.name + " model, DTO and validation schema must require a non-null key; remove nullable/jsonOmitEmpty")
			}
			for _, expression := range []string{cursor.ValibotDTO, cursor.ValibotModel} {
				for _, optional := range []string{"optional", "exactOptional", "nullable", "nullish", "undefined_", "null_", "any", "unknown"} {
					if containsTypeScriptIdentifier(expression, optional) {
						return fail(projection.name + " DTO and model validation schemas must require a non-null key")
					}
				}
			}
		}
	}
	return nil
}
