package codegen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// EnumBackendSpec controls Go ownership independently of frontend generation.
type EnumBackendSpec struct {
	Generation string `yaml:"generation"` // managed (default) or manual
	Register   *bool  `yaml:"register"`   // defaults to true only for managed types
}

// EnumMigrationSpec explicitly opts a shared PostgreSQL type into an initial
// CREATE TYPE migration. It never rewrites history or alters existing columns.
type EnumMigrationSpec struct {
	Enabled bool   `yaml:"enabled"`
	Name    string `yaml:"name"`
}

type BackendEnumValueView struct {
	Value string
	Const string
}

type BackendEnumView struct {
	Name             string
	GoType           string
	FileBase         string
	SQLType          string
	Managed          bool
	Register         bool
	Values           []BackendEnumValueView
	MigrationEnabled bool
	MigrationName    string
}

type BackendEnumsView struct {
	Enums []BackendEnumView
}

func buildBackendEnum(enum EnumSpec) BackendEnumView {
	managed := enum.Backend.Generation != "manual"
	view := BackendEnumView{
		Name: enum.Name, GoType: enum.GoType, FileBase: CamelCase(enum.GoType),
		SQLType: enum.SQLType, Managed: managed,
		Register:         boolOrDefault(enum.Backend.Register, managed),
		MigrationEnabled: enum.Migration.Enabled, MigrationName: enum.Migration.Name,
	}
	if view.MigrationName == "" && enum.SQLType != "" {
		view.MigrationName = "create_enum_" + strings.ReplaceAll(enum.SQLType, ".", "_")
	}
	for _, item := range enum.Values {
		name := item.GoConst
		if name == "" {
			words := strings.FieldsFunc(item.Value, func(r rune) bool {
				return !unicode.IsLetter(r) && !unicode.IsDigit(r)
			})
			name = enum.GoType + PascalCase(strings.Join(words, "_"))
		}
		view.Values = append(view.Values, BackendEnumValueView{Value: item.Value, Const: name})
	}
	return view
}

func validateBackendEnumDefinition(enum EnumSpec) error {
	if mode := enum.Backend.Generation; mode != "" && mode != "managed" && mode != "manual" {
		return fmt.Errorf("enum %q: backend.generation must be managed or manual", enum.Name)
	}
	view := buildBackendEnum(enum)
	if view.Managed {
		if types.Universe.Lookup(enum.GoType) != nil || enum.GoType == "init" {
			return fmt.Errorf("enum %q: managed goType %q shadows a predeclared Go identifier", enum.Name, enum.GoType)
		}
		seen := map[string]bool{enum.GoType: true, enum.GoType + "Values": true, "RegisterGeneratedModelbindEnums": true}
		for _, value := range view.Values {
			if !token.IsIdentifier(value.Const) || value.Const == "_" || value.Const == "init" || types.Universe.Lookup(value.Const) != nil {
				return fmt.Errorf("enum %q value %q: invalid generated Go constant %q; supply values[].goConst", enum.Name, value.Value, value.Const)
			}
			if seen[value.Const] {
				return fmt.Errorf("enum %q value %q: duplicate generated Go identifier %q; supply a unique values[].goConst", enum.Name, value.Value, value.Const)
			}
			seen[value.Const] = true
		}
	}
	for _, value := range enum.Values {
		if value.GoConst != "" && (!token.IsIdentifier(value.GoConst) || value.GoConst == "_") {
			return fmt.Errorf("enum %q value %q: goConst must be a Go identifier", enum.Name, value.Value)
		}
	}
	if enum.Migration.Name != "" && !isSQLIdentifier(enum.Migration.Name) {
		return fmt.Errorf("enum %q: migration.name must be an SQL-style identifier", enum.Name)
	}
	if enum.Migration.Enabled {
		if len(strings.Split(enum.SQLType, ".")) != 2 {
			return fmt.Errorf("enum %q: migration.enabled requires a schema-qualified sqlType, such as public.material_status_types", enum.Name)
		}
		for _, part := range strings.Split(enum.SQLType, ".") {
			if len(part) > 63 {
				return fmt.Errorf("enum %q: PostgreSQL type identifiers must not exceed 63 bytes", enum.Name)
			}
		}
		for _, value := range enum.Values {
			if len(value.Value) > 63 || strings.ContainsRune(value.Value, '\x00') {
				return fmt.Errorf("enum %q value %q: PostgreSQL enum labels must not contain NUL or exceed 63 bytes", enum.Name, value.Value)
			}
		}
	}
	return nil
}

func backendEnumSymbols(view BackendEnumView) []string {
	if !view.Managed {
		return nil
	}
	symbols := []string{view.GoType, view.GoType + "Values"}
	for _, value := range view.Values {
		symbols = append(symbols, value.Const)
	}
	return symbols
}

func validateBackendEnumRegistry(registry map[string]EnumSpec) error {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	owners := map[string]string{"symbol:RegisterGeneratedModelbindEnums": "Codegen registry"}
	for _, name := range names {
		view := buildBackendEnum(registry[name])
		identities := []string{}
		if view.Managed {
			identities = append(identities, "file:"+strings.ToLower(view.FileBase))
			for _, symbol := range backendEnumSymbols(view) {
				identities = append(identities, "symbol:"+symbol)
			}
		}
		// A database type must have exactly one enum owner. Different registry
		// names referring to an existing shared type should use manual ownership.
		if view.MigrationEnabled {
			identities = append(identities, "sqlType:"+strings.ToLower(view.SQLType), "migration:"+view.MigrationName)
		}
		for _, identity := range identities {
			if previous, exists := owners[identity]; exists && previous != name {
				return fmt.Errorf("enums %q and %q conflict on generated backend identity %q", previous, name, identity)
			}
			owners[identity] = name
		}
	}
	return nil
}

func backendEnumsForObject(spec ObjectSpec) []BackendEnumView {
	byName := make(map[string]BackendEnumView)
	for _, enum := range spec.Enums {
		byName[enum.Name] = buildBackendEnum(enum)
	}
	groups := [][]FieldSpec{spec.Fields}
	if spec.List != nil {
		groups = append(groups, spec.List.Fields)
	}
	if spec.Detail != nil {
		groups = append(groups, spec.Detail.Fields)
	}
	for _, fields := range groups {
		for _, field := range fields {
			if field.enumDefinition != nil {
				byName[field.Enum] = buildBackendEnum(*field.enumDefinition)
			}
		}
	}
	return sortedBackendEnums(byName)
}

func sortedBackendEnums(byName map[string]BackendEnumView) []BackendEnumView {
	enums := make([]BackendEnumView, 0, len(byName))
	for _, enum := range byName {
		enums = append(enums, enum)
	}
	sort.Slice(enums, func(i, j int) bool { return enums[i].Name < enums[j].Name })
	return enums
}

func collectBackendEnums(objects []ObjectView) ([]BackendEnumView, error) {
	byName := make(map[string]BackendEnumView)
	for _, object := range objects {
		for _, enum := range object.BackendEnums {
			if old, exists := byName[enum.Name]; exists && !reflect.DeepEqual(old, enum) {
				return nil, fmt.Errorf("conflicting backend enum %q in object %s", enum.Name, object.Name)
			}
			byName[enum.Name] = enum
		}
	}
	return sortedBackendEnums(byName), nil
}

func validateBackendEnumObjects(cfg Config, objects []ObjectView) error {
	enums, err := collectBackendEnums(objects)
	if err != nil {
		return err
	}
	if len(enums) == 0 {
		return nil
	}
	if cfg.MigrationsEnabled {
		if err := validateBackendEnumMigrationOrder(cfg, objects, enums); err != nil {
			return err
		}
	}
	if !cfg.BackendEnabled {
		return nil
	}
	expected := make(map[string]string)
	for _, enum := range enums {
		for _, name := range backendEnumSymbols(enum) {
			if old, exists := expected[name]; exists {
				return fmt.Errorf("backend enums %q and %q both declare Go identifier %q", old, enum.Name, name)
			}
			expected[name] = enum.Name
		}
		if enum.Register {
			expected["RegisterGeneratedModelbindEnums"] = "generated enum registry"
		}
	}
	if len(expected) == 0 {
		return nil
	}
	imports := map[string]bool{"wmodels": len(objects) > 0}
	for _, enum := range enums {
		if enum.Register {
			imports["fmt"], imports["slices"], imports["metadata"] = true, true, true
		}
	}
	for _, object := range objects {
		imports["time"] = imports["time"] || object.NeedsTimeImport
		imports["modelbind"] = imports["modelbind"] || object.NeedsModelbindImport
	}
	for name, used := range imports {
		if enum, exists := expected[name]; used && exists {
			return fmt.Errorf("backend enum %q identifier %q conflicts with a generated models-package import", enum, name)
		}
	}
	for _, object := range objects {
		for _, name := range []string{object.Name, object.Name + "Key", "Update" + object.Name + "Request", object.ListModelName, object.DetailModelName, object.Camel + "Relation", object.Camel + "ListRelation", object.Camel + "DetailRelation"} {
			if enum, exists := expected[name]; exists {
				return fmt.Errorf("backend enum %q identifier %q conflicts with generated model %s", enum, name, object.Name)
			}
		}
	}
	// Enum collisions are always fatal, even when allowManualCollisions is set:
	// emitting two declarations of the same type can never compile.
	return validateExistingBackendEnumSymbols(cfg, objects, enums, expected)
}

func validateExistingBackendEnumSymbols(cfg Config, objects []ObjectView, enums []BackendEnumView, expected map[string]string) error {
	ignored := make(map[string]bool)
	for _, object := range objects {
		ignored[object.FileBase+".gen.go"] = true // will be replaced by this run
	}
	for _, enum := range enums {
		if enum.Managed {
			ignored[enum.FileBase+".enum.gen.go"] = true
		}
	}
	ignored["enums_registry.gen.go"] = true
	manifest, _, err := readBackendEnumManifest(cfg.ServerRoot)
	if err != nil {
		return err
	}
	for _, file := range manifest.Files {
		ignored[filepath.Base(file.Path)] = true // stale owned files are removed later
	}
	dir := filepath.Join(cfg.ServerRoot, "internal", "models")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") || ignored[entry.Name()] {
			continue
		}
		filename := filepath.Join(dir, entry.Name())
		file, err := parser.ParseFile(token.NewFileSet(), filename, nil, 0)
		if err != nil {
			return fmt.Errorf("inspect backend enum collisions in %s: %w", filename, err)
		}
		for _, decl := range file.Decls {
			names := []string{}
			switch decl := decl.(type) {
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					switch spec := spec.(type) {
					case *ast.ImportSpec:
						if spec.Name != nil {
							names = append(names, spec.Name.Name)
						} else {
							importPath, _ := strconv.Unquote(spec.Path.Value)
							names = append(names, filepath.Base(importPath))
						}
					case *ast.TypeSpec:
						names = append(names, spec.Name.Name)
					case *ast.ValueSpec:
						for _, name := range spec.Names {
							names = append(names, name.Name)
						}
					}
				}
			case *ast.FuncDecl:
				if decl.Recv == nil {
					names = append(names, decl.Name.Name)
				} else if decl.Name.Name == "IsValid" || decl.Name.Name == "String" {
					for _, receiver := range decl.Recv.List {
						typ := receiver.Type
						if pointer, ok := typ.(*ast.StarExpr); ok {
							typ = pointer.X
						}
						if typ, ok := typ.(*ast.Ident); ok {
							for _, enum := range enums {
								if enum.Managed && enum.GoType == typ.Name {
									return fmt.Errorf("backend enum %q method %s.%s conflicts with %s; use backend.generation: manual or remove the duplicate method", enum.Name, typ.Name, decl.Name.Name, filename)
								}
							}
						}
					}
				}
			}
			for _, name := range names {
				if enum, exists := expected[name]; exists {
					return fmt.Errorf("backend enum %q identifier %q already exists in %s; use backend.generation: manual for an existing enum, or deliberately remove the manual declaration before generating", enum, name, filename)
				}
			}
		}
	}
	return nil
}
