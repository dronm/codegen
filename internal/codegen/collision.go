package codegen

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func validateManualFrontendCollisions(frontendRoot string, objects []ObjectView) error {
	conflicts := make([]string, 0)
	for _, object := range objects {
		paths := []string{
			filepath.Join(frontendRoot, "src", "types", object.FileBase+".ts"),
			filepath.Join(frontendRoot, "src", "schemas", object.FileBase+".ts"),
			filepath.Join(frontendRoot, "src", "api", object.FileBase+".ts"),
			filepath.Join(frontendRoot, "src", "composables", "schemas", "use"+object.Name+"Schemas.ts"),
		}
		if object.HasCustomList {
			paths = append(paths,
				filepath.Join(frontendRoot, "src", "types", object.ListFileBase+".ts"),
				filepath.Join(frontendRoot, "src", "schemas", object.ListFileBase+".ts"),
			)
		}
		for _, path := range paths {
			if _, err := os.Stat(path); err == nil {
				conflicts = append(conflicts, fmt.Sprintf("frontend file for %s conflicts with %s", object.Name, path))
			} else if !os.IsNotExist(err) {
				return fmt.Errorf("check frontend file %s: %w", path, err)
			}
		}
	}

	routePath := filepath.Join(frontendRoot, "src", "router", "routeManifest.ts")
	routeData, err := os.ReadFile(routePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read frontend route manifest %s: %w", routePath, err)
	}
	if err == nil {
		namePattern := regexp.MustCompile(`name:\s*["']([^"']+)["']`)
		pathPattern := regexp.MustCompile(`path:\s*["']([^"']+)["']`)
		names := make(map[string]struct{})
		paths := make(map[string]struct{})
		for _, match := range namePattern.FindAllSubmatch(routeData, -1) {
			names[string(match[1])] = struct{}{}
		}
		for _, match := range pathPattern.FindAllSubmatch(routeData, -1) {
			paths[string(match[1])] = struct{}{}
		}
		for _, object := range objects {
			generatedRoutes := make([]struct {
				Name string
				Path string
			}, 0, 3)
			if object.Frontend.List.Enabled {
				generatedRoutes = append(generatedRoutes, struct {
					Name string
					Path string
				}{Name: object.Frontend.Routes.ListName, Path: object.Frontend.Routes.ListPath})
			}
			if object.Frontend.Form.Enabled && object.CRUD.Create {
				generatedRoutes = append(generatedRoutes, struct {
					Name string
					Path string
				}{Name: object.Frontend.Routes.CreateName, Path: object.Frontend.Routes.CreatePath})
			}
			if object.Frontend.Form.Enabled && object.CRUD.Detail && object.CRUD.Update {
				generatedRoutes = append(generatedRoutes, struct {
					Name string
					Path string
				}{Name: object.Frontend.Routes.EditName, Path: object.Frontend.Routes.EditPath})
			}
			for _, route := range generatedRoutes {
				if _, exists := names[route.Name]; exists {
					conflicts = append(conflicts, fmt.Sprintf("frontend route name %q for %s conflicts with %s", route.Name, object.Name, routePath))
				}
				if _, exists := paths[route.Path]; exists {
					conflicts = append(conflicts, fmt.Sprintf("frontend route path %q for %s conflicts with %s", route.Path, object.Name, routePath))
				}
			}
		}
	}

	localePath := filepath.Join(frontendRoot, "src", "locales", "ru.json")
	localeData, err := os.ReadFile(localePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read frontend locale %s: %w", localePath, err)
	}
	if err == nil {
		var locale map[string]json.RawMessage
		if err := json.Unmarshal(localeData, &locale); err != nil {
			return fmt.Errorf("parse frontend locale %s: %w", localePath, err)
		}
		for _, object := range objects {
			if _, exists := locale[object.Name]; exists && (object.Frontend.List.Enabled || object.Frontend.Form.Enabled) {
				conflicts = append(conflicts, fmt.Sprintf("frontend locale key %q for %s conflicts with %s", object.Name, object.Name, localePath))
			}
		}
	}

	if len(conflicts) == 0 {
		return nil
	}
	sort.Strings(conflicts)
	return fmt.Errorf(
		"generated frontend conflicts with hand-written files, routes, or locale keys:\n- %s; remove the conflicting manual artifact or set CODEGEN_ALLOW_MANUAL_COLLISIONS=true only during a deliberate ownership migration",
		strings.Join(conflicts, "\n- "),
	)
}

type manualBackendSymbols struct {
	ModelTypes      map[string]string
	ServiceSymbols  map[string]string
	ServiceNames    map[string]string
	HTTPSymbols     map[string]string
	HTTPRoutes      map[string]string
	RouteNames      map[string]string
	PermissionNames map[string]string
}

func validateManualBackendCollisions(serverRoot string, objects []ObjectView) error {
	symbols, err := scanManualBackendSymbols(serverRoot)
	if err != nil {
		return err
	}

	conflicts := make([]string, 0)
	addConflict := func(kind string, name string, source string, object ObjectView) {
		if source == "" {
			return
		}
		conflicts = append(conflicts, fmt.Sprintf(
			"%s %q for %s conflicts with %s",
			kind,
			name,
			object.Name,
			source,
		))
	}

	for _, object := range objects {
		modelTypes := []string{object.Name, object.Name + "Key"}
		if object.CRUD.Update {
			modelTypes = append(modelTypes, "Update"+object.Name+"Request")
		}
		if object.HasCustomList {
			modelTypes = append(modelTypes, object.ListModelName)
		}
		for _, name := range modelTypes {
			addConflict("model type", name, symbols.ModelTypes[name], object)
		}

		for _, name := range []string{
			object.Name + "Service",
			"New" + object.Name + "Service",
			"Register" + object.Name + "Service",
		} {
			addConflict("service symbol", name, symbols.ServiceSymbols[name], object)
		}
		addConflict("registered service name", object.ServiceName, symbols.ServiceNames[object.ServiceName], object)

		httpSymbols := []string{object.Camel + "Routes"}
		if object.CompositeKey && (object.CRUD.Detail || object.CRUD.Update || object.CRUD.Delete) {
			httpSymbols = append(httpSymbols, object.Camel+"KeyBinder", object.Camel+"KeyFromRequest")
		}
		if object.CompositeKey && object.CRUD.Update {
			httpSymbols = append(httpSymbols, object.Camel+"UpdateBinder")
		}
		for _, name := range httpSymbols {
			addConflict("HTTP symbol", name, symbols.HTTPSymbols[name], object)
		}

		for _, route := range generatedHTTPRoutes(object) {
			addConflict("HTTP route", route, symbols.HTTPRoutes[route], object)
		}
		for _, code := range generatedActionCodes(object) {
			addConflict("route name", code, symbols.RouteNames[code], object)
			if object.PermissionsEnabled {
				addConflict("permission name", code, symbols.PermissionNames[code], object)
			}
		}
	}

	if len(conflicts) == 0 {
		return nil
	}
	sort.Strings(conflicts)
	return fmt.Errorf(
		"generated backend conflicts with hand-written registrations or declarations:\n- %s; remove the conflicting schema/manual declaration or set CODEGEN_ALLOW_MANUAL_COLLISIONS=true only during a deliberate ownership migration",
		strings.Join(conflicts, "\n- "),
	)
}

func generatedActionCodes(object ObjectView) []string {
	actions := make([]string, 0, 5)
	if object.CRUD.Create {
		actions = append(actions, "create")
	}
	if object.CRUD.List {
		actions = append(actions, "list")
	}
	if object.CRUD.Detail {
		actions = append(actions, "detail")
	}
	if object.CRUD.Update {
		actions = append(actions, "update")
	}
	if object.CRUD.Delete {
		actions = append(actions, "delete")
	}

	codes := make([]string, 0, len(actions))
	for _, action := range actions {
		codes = append(codes, object.PermissionPrefix+"."+action)
	}
	return codes
}

func generatedHTTPRoutes(object ObjectView) []string {
	routes := make([]string, 0, 5)
	if object.CRUD.Create {
		routes = append(routes, "POST "+object.Route)
	}
	if object.CRUD.List {
		routes = append(routes, "GET "+object.Route)
	}
	if object.CRUD.Detail {
		routes = append(routes, "GET "+object.ItemRoute)
	}
	if object.CRUD.Update {
		routes = append(routes, "PATCH "+object.ItemRoute)
	}
	if object.CRUD.Delete {
		routes = append(routes, "DELETE "+object.ItemRoute)
	}
	return routes
}

func scanManualBackendSymbols(serverRoot string) (manualBackendSymbols, error) {
	result := manualBackendSymbols{
		ModelTypes:      make(map[string]string),
		ServiceSymbols:  make(map[string]string),
		ServiceNames:    make(map[string]string),
		HTTPSymbols:     make(map[string]string),
		HTTPRoutes:      make(map[string]string),
		RouteNames:      make(map[string]string),
		PermissionNames: make(map[string]string),
	}

	if err := scanGoDirectory(
		filepath.Join(serverRoot, "internal", "models"),
		func(path string, file *ast.File) {
			for _, declaration := range file.Decls {
				general, ok := declaration.(*ast.GenDecl)
				if !ok || general.Tok != token.TYPE {
					continue
				}
				for _, spec := range general.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if ok {
						result.ModelTypes[typeSpec.Name.Name] = path
					}
				}
			}
		},
	); err != nil {
		return manualBackendSymbols{}, err
	}

	if err := scanGoDirectory(
		filepath.Join(serverRoot, "internal", "services"),
		func(path string, file *ast.File) {
			collectTopLevelSymbols(file, path, result.ServiceSymbols)
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || selectorName(call.Fun) != "MustRegisterService" || len(call.Args) == 0 {
					return true
				}
				if value, ok := stringLiteral(call.Args[0]); ok {
					result.ServiceNames[value] = path
				}
				return true
			})
		},
	); err != nil {
		return manualBackendSymbols{}, err
	}

	if err := scanGoDirectory(
		filepath.Join(serverRoot, "internal", "httpapi"),
		func(path string, file *ast.File) {
			collectTopLevelSymbols(file, path, result.HTTPSymbols)
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || len(call.Args) == 0 {
					return true
				}
				name := selectorName(call.Fun)
				value, isString := stringLiteral(call.Args[0])
				if !isString {
					return true
				}
				switch name {
				case "GET", "POST", "PATCH", "DELETE", "PUT":
					result.HTTPRoutes[name+" "+value] = path
				case "WithName":
					result.RouteNames[value] = path
				case "WithPermission":
					result.PermissionNames[value] = path
				}
				return true
			})
		},
	); err != nil {
		return manualBackendSymbols{}, err
	}

	return result, nil
}

func scanGoDirectory(dir string, inspect func(string, *ast.File)) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read Go source directory %s: %w", dir, err)
	}

	fileSet := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") ||
			strings.HasSuffix(name, "_test.go") ||
			strings.HasSuffix(name, ".gen.go") ||
			strings.HasSuffix(name, "_gen.go") {
			continue
		}
		path := filepath.Join(dir, name)
		file, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			return fmt.Errorf("parse Go source %s: %w", path, err)
		}
		inspect(path, file)
	}

	return nil
}

func collectTopLevelSymbols(file *ast.File, path string, destination map[string]string) {
	for _, declaration := range file.Decls {
		switch item := declaration.(type) {
		case *ast.FuncDecl:
			destination[item.Name.Name] = path
		case *ast.GenDecl:
			if item.Tok != token.TYPE {
				continue
			}
			for _, spec := range item.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					destination[typeSpec.Name.Name] = path
				}
			}
		}
	}
}

func selectorName(expression ast.Expr) string {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	return selector.Sel.Name
}

func stringLiteral(expression ast.Expr) (string, bool) {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(literal.Value)
	if err != nil {
		return "", false
	}
	return value, true
}
