package codegen

import (
	"fmt"
	"go/ast"
	"go/token"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

type accessMethodSignature struct {
	Name       string
	Input      string
	Source     string
	HasContext bool
	ParamCount int
}

func buildAccessDecoders(cfg Config, spec ObjectSpec, object ObjectView, result *AccessView) error {
	if !cfg.BackendEnabled {
		return nil
	}
	manual, err := scanAccessServiceMethods(cfg.ServerRoot, cfg.GoModule, object.Name+"Service")
	if err != nil && cfg.BackendEnabled {
		return err
	}
	// These are the concrete signatures emitted by service.go.tmpl, not
	// guesses from table names. Additional handwritten method names are read
	// from Go declarations and need an explicit operation in serviceInputs.
	methods := map[string]accessMethodSignature{}
	operations := map[string]string{}
	if object.CRUD.Create {
		methods["Create"] = accessMethodSignature{Name: "Create", Input: "modelbind.ModelInput[*models." + object.Name + "]", HasContext: true, ParamCount: 2}
		operations["Create"] = "insert"
	}
	if object.CRUD.Update {
		methods["Update"] = accessMethodSignature{Name: "Update", Input: "webapp.UpdateByKeysInput[*models." + object.Name + "Key,*models." + object.Name + "]", HasContext: true, ParamCount: 2}
		operations["Update"] = "update"
	}
	for name, owned := range map[string]bool{"Create": object.ManualServiceCRUD.Create, "Update": object.ManualServiceCRUD.Update} {
		if !owned || !cfg.BackendEnabled {
			continue
		}
		signature, found := manual[name]
		if !found {
			return accessError(spec, "handwritten method %sService.%s was not found; its actual signature is required for policy input integration", object.Name, name)
		}
		methods[name] = signature
	}
	for _, name := range sortedAccessFields(spec.Access.ServiceInputs) {
		input := spec.Access.ServiceInputs[name]
		if !token.IsIdentifier(name) || !ast.IsExported(name) {
			return accessError(spec, "serviceInputs.%s must name an exported service method", name)
		}
		if input.Operation != "insert" && input.Operation != "update" {
			return accessError(spec, "serviceInputs.%s.operation must be insert or update", name)
		}
		if previous, ok := operations[name]; ok && previous != input.Operation {
			return accessError(spec, "serviceInputs.%s.operation conflicts with generated %s operation", name, previous)
		}
		if _, ok := methods[name]; !ok {
			signature, found := manual[name]
			if !found {
				return accessError(spec, "serviceInputs.%s does not name an existing generated or handwritten method", name)
			}
			methods[name] = signature
		}
		operations[name] = input.Operation
	}
	for _, name := range sortedAccessFields(manual) {
		signature := manual[name]
		if !ast.IsExported(name) || name == "Detail" || name == "Delete" || name == "List" {
			continue
		}
		if _, configured := methods[name]; !configured && protectedAccessInput(signature.Input, object.Name) {
			return accessError(spec, "handwritten method %sService.%s (%s) needs serviceInputs.%s.operation; a write operation cannot be inferred from its name", object.Name, name, signature.Source, name)
		}
	}
	imports := map[string]string{}
	for _, name := range sortedAccessFields(methods) {
		method := methods[name]
		if !method.HasContext || method.ParamCount != 2 {
			return accessError(spec, "method %sService.%s (%s) must have (context.Context, input) parameters; provide a handwritten service registration for unsupported signatures", object.Name, name, method.Source)
		}
		operation := operations[name]
		config := spec.Access.ServiceInputs[name]
		shape, supported := accessShape(method.Input, object.Name)
		var expression string
		if config.Decoder != nil {
			decoder := config.Decoder
			if !token.IsIdentifier(decoder.Name) || decoder.Name == "_" {
				return accessError(spec, "serviceInputs.%s.decoder.name must be a Go identifier naming a webapp.ServiceInputDecoder", name)
			}
			expression = decoder.Name
			if decoder.Import != "" {
				if !ast.IsExported(decoder.Name) {
					return accessError(spec, "serviceInputs.%s.decoder.name must be exported when decoder.import is supplied", name)
				}
				if strings.TrimSpace(decoder.Import) != decoder.Import || strings.ContainsAny(decoder.Import, "\"`\r\n\t ") || decoder.Import == cfg.GoModule+"/internal/services" {
					return accessError(spec, "serviceInputs.%s.decoder.import must be an importable package outside internal/services (omit it for a local function)", name)
				}
				// Use stable synthetic aliases so function names cannot collide
				// with existing service imports or each other.
				alias := fmt.Sprintf("accessdecoder%d", len(imports)+1)
				for existing, imported := range imports {
					if imported == decoder.Import {
						alias = existing
					}
				}
				imports[alias] = decoder.Import
				expression = alias + "." + decoder.Name
			}
		} else {
			if !supported {
				return accessError(spec, "method %sService.%s input %s is unsupported; set serviceInputs.%s.decoder to a handwritten ServiceInputDecoder and use a handwritten HTTP binding", object.Name, name, method.Input, name)
			}
			if shape.Kind == "keys" && operation != "update" {
				return accessError(spec, "method %s: UpdateByKeysInput requires the update operation", name)
			}
			// The attached libraries decode struct values, but preparation and
			// validation require pointer models. Decode/prepare POINTER inputs
			// through the public APIs, then adapt to the exact service signature.
			modelValue := !strings.HasPrefix(shape.Model, "*")
			keyValue := shape.Kind == "keys" && !strings.HasPrefix(shape.Key, "*")
			if modelValue || keyValue {
				adapter := AccessDecoderAdapterView{
					Name: object.Camel + name + "AccessJSONDecoder", Kind: shape.Kind,
					Model: shape.Model, Key: shape.Key, ModelValue: modelValue,
					KeyValue: keyValue, ForInsert: operation == "insert",
				}
				result.DecoderAdapters = append(result.DecoderAdapters, adapter)
				result.AdapterModelbind = result.AdapterModelbind || shape.Kind != "model"
				expression = adapter.Name + "()"
			} else {
				switch shape.Kind {
				case "keys":
					expression = fmt.Sprintf("webapp.UpdateByKeysJSONDecoder[%s, %s]()", shape.Key, shape.Model)
				case "input":
					expression = fmt.Sprintf("webapp.ModelInputJSONDecoder[%s](%t)", shape.Model, operation == "insert")
				case "model":
					expression = fmt.Sprintf("webapp.ModelJSONDecoder[%s](%t)", shape.Model, operation == "insert")
				}
			}
		}
		if strings.Contains(expression, "models.") {
			result.NeedsModelsImport = true
		}
		result.Decoders = append(result.Decoders, AccessDecoderView{Method: method.Name, Expression: expression})
		if name == "Create" && object.GeneratedHTTPCRUD.Create {
			if config.Decoder != nil || !supported || shape.Kind == "keys" {
				return accessError(spec, "Create input requires httpRoutes.manualMethods: [create] and a handwritten policy-aware HTTP binder")
			}
			binder := "InsertInputBinder"
			if shape.Kind == "model" {
				binder = "InsertModelBinder"
			}
			result.CreateBinder = fmt.Sprintf("webapp.%s[%s]()", binder, shape.Model)
			if !strings.HasPrefix(shape.Model, "*") {
				result.CreateValue, result.CreateKind, result.CreateModelType = true, shape.Kind, shape.Model
				result.CreateBinder = object.Camel + "AccessCreateBinder()"
				result.HTTPModelbind = shape.Kind == "input"
			}
		}
		if name == "Update" && object.GeneratedHTTPCRUD.Update {
			if config.Decoder != nil || !supported || shape.Kind != "keys" {
				return accessError(spec, "Update input %s cannot accept the generated path-key/body contract; set httpRoutes.manualMethods: [update] and supply a handwritten policy-aware binder", method.Input)
			}
			result.UpdateKeyType, result.UpdateModelType = shape.Key, shape.Model
			result.UpdateKeyValue = !strings.HasPrefix(shape.Key, "*")
			result.UpdateModelValue = !strings.HasPrefix(shape.Model, "*")
			result.HTTPModelbind = true
		}
	}
	for _, alias := range sortedAccessFields(imports) {
		result.DecoderImports = append(result.DecoderImports, AccessImportView{Name: alias, Path: imports[alias]})
	}
	return nil
}

func scanAccessServiceMethods(root, module, service string) (map[string]accessMethodSignature, error) {
	methods := map[string]accessMethodSignature{}
	var conflict error
	err := scanGoDirectory(filepath.Join(root, "internal/services"), func(filename string, file *ast.File) {
		aliases := map[string]string{}
		for _, imp := range file.Imports {
			importPath, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				continue
			}
			alias := path.Base(importPath)
			if imp.Name != nil {
				alias = imp.Name.Name
			}
			switch importPath {
			case "context":
				aliases[alias] = "context"
			case "github.com/dronm/modelbind":
				aliases[alias] = "modelbind"
			case "github.com/dronm/webapp":
				aliases[alias] = "webapp"
			case module + "/internal/models":
				aliases[alias] = "models"
			default:
				aliases[alias] = "package(" + importPath + ")"
			}
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) != 1 || receiverTypeName(fn.Recv.List[0].Type) != service {
				continue
			}
			params := []string{}
			for _, field := range fn.Type.Params.List {
				count := len(field.Names)
				if count == 0 {
					count = 1
				}
				for i := 0; i < count; i++ {
					params = append(params, canonicalAccessType(field.Type, aliases))
				}
			}
			signature := accessMethodSignature{Name: fn.Name.Name, Source: filename, ParamCount: len(params)}
			if len(params) > 0 {
				signature.HasContext = params[0] == "context.Context"
			}
			if len(params) == 2 {
				signature.Input = params[1]
			}
			if old, found := methods[signature.Name]; found {
				conflict = fmt.Errorf("access: duplicate handwritten method %s.%s in %s and %s", service, signature.Name, old.Source, filename)
			}
			methods[signature.Name] = signature
		}
	})
	if err != nil {
		return nil, err
	}
	return methods, conflict
}
