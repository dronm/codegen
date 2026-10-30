package codegen

import (
	"fmt"
	"go/ast"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// A generated binder must agree with the input of a handwritten List method.
// Manual HTTP routes remain application-owned: declaring grouped capability
// does not let codegen infer or rewrite their binder and service contracts.
func validateGroupedCollectionServices(cfg Config, objects []ObjectView) error {
	if !cfg.BackendEnabled {
		return nil
	}
	for _, object := range objects {
		if !object.GroupedFilters || !object.GeneratedHTTPCRUD.List || !object.ManualServiceCRUD.List {
			continue
		}
		methods, err := scanAccessServiceMethods(cfg.ServerRoot, cfg.GoModule, object.Name+"Service")
		if err != nil {
			return fmt.Errorf("%s: collection.groupedFilters: %w", sourceLabel(object.SourceFile), err)
		}
		method, found := methods["List"]
		if !found {
			// Ordinary manual ownership validation diagnoses missing methods.
			continue
		}
		method, err = resolveGroupedListAliases(cfg.ServerRoot, object.Name+"Service", method)
		if err != nil {
			return fmt.Errorf("%s: collection.groupedFilters: %w", sourceLabel(object.SourceFile), err)
		}
		if method.HasContext && method.ParamCount == 2 && method.Input == "modelbind.CollectionParamsWithWhere" {
			continue
		}
		return fmt.Errorf(
			"%s: collection.groupedFilters: generated CollectionParamsWithWhereBinder targets handwritten %sService.List in %s with incompatible input %s; migrate List to (context.Context, modelbind.CollectionParamsWithWhere) and use FetchCollectionModelWhere (or FetchCollectionModelWhereWithConn), or transfer the list route to httpRoutes.manualMethods and implement a grouped-capable binding",
			sourceLabel(object.SourceFile), object.Name, method.Source, method.Input,
		)
	}
	return nil
}

// Resolve local Go aliases for this collection contract without broadening the
// access-policy scanner's supported input shapes. Declarations may live in a
// separate handwritten file, and each declaration keeps its file's import names.
func resolveGroupedListAliases(root, service string, method accessMethodSignature) (accessMethodSignature, error) {
	aliases := map[string]string{}
	contextType := ""
	err := scanGoDirectory(filepath.Join(root, "internal/services"), func(_ string, file *ast.File) {
		imports := map[string]string{}
		for _, imp := range file.Imports {
			importPath, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				continue
			}
			name := path.Base(importPath)
			if imp.Name != nil {
				name = imp.Name.Name
			}
			switch importPath {
			case "context":
				imports[name] = "context"
			case "github.com/dronm/modelbind":
				imports[name] = "modelbind"
			default:
				imports[name] = "package(" + importPath + ")"
			}
		}
		for _, declaration := range file.Decls {
			switch decl := declaration.(type) {
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					alias, ok := spec.(*ast.TypeSpec)
					if ok && alias.Assign.IsValid() {
						aliases[alias.Name.Name] = canonicalAccessType(alias.Type, imports)
					}
				}
			case *ast.FuncDecl:
				if decl.Name.Name == "List" && decl.Recv != nil && len(decl.Recv.List) == 1 && receiverTypeName(decl.Recv.List[0].Type) == service && len(decl.Type.Params.List) > 0 {
					contextType = canonicalAccessType(decl.Type.Params.List[0].Type, imports)
				}
			}
		}
	})
	if err != nil {
		return method, err
	}
	resolve := func(value string) string {
		seen := map[string]bool{}
		prefix := ""
		for {
			for strings.HasPrefix(value, "*") {
				prefix += "*"
				value = strings.TrimPrefix(value, "*")
			}
			if seen[value] {
				return prefix + "<cyclic alias>"
			}
			seen[value] = true
			target, ok := aliases[value]
			if !ok {
				return prefix + value
			}
			value = target
		}
	}
	method.HasContext = resolve(contextType) == "context.Context"
	method.Input = resolve(method.Input)
	return method, nil
}
