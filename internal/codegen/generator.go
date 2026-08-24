package codegen

import (
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

func Generate(cfg Config) error {
	objects, registers, err := prepareProject(cfg)
	if err != nil {
		return err
	}

	renderer := NewRenderer(cfg)
	for _, object := range objects {
		if err := renderer.RenderObject(object); err != nil {
			return fmt.Errorf("render object %s: %w", object.Name, err)
		}
	}
	if cfg.RegistersEnabled && len(registers) > 0 {
		if err := renderer.RenderRegisters(registers); err != nil {
			return fmt.Errorf("render registers: %w", err)
		}
	}

	if cfg.BackendEnabled && cfg.RegistriesEnabled {
		if err := renderer.RenderRoutes(objects); err != nil {
			return fmt.Errorf("render routes: %w", err)
		}
		if err := renderer.RenderServices(objects); err != nil {
			return fmt.Errorf("render services: %w", err)
		}
	}
	if cfg.FrontendEnabled && cfg.RegistriesEnabled {
		if err := renderer.RenderFrontendRoutes(objects); err != nil {
			return fmt.Errorf("render frontend routes: %w", err)
		}
		if err := renderer.RenderFrontendLocale(objects); err != nil {
			return fmt.Errorf("render frontend locale: %w", err)
		}
	}

	return nil
}

func Validate(cfg Config) error {
	_, _, err := prepareProject(cfg)
	return err
}

func prepareProject(cfg Config) ([]ObjectView, []RegisterView, error) {
	objects, err := prepareObjects(cfg)
	if err != nil {
		return nil, nil, err
	}
	if !cfg.RegistersEnabled {
		return objects, nil, nil
	}

	specs, err := LoadRegisters(cfg.RegisterSchemaDir)
	if err != nil {
		return nil, nil, err
	}
	registers := make([]RegisterView, 0, len(specs))
	for _, spec := range specs {
		view, err := BuildRegisterView(spec)
		if err != nil {
			return nil, nil, fmt.Errorf("build register %s: %w", spec.Name, err)
		}
		view.RuntimeVersion = cfg.RegisterRuntimeVersion
		registers = append(registers, view)
	}
	if err := validateRegisterViews(objects, registers); err != nil {
		return nil, nil, err
	}
	return objects, registers, nil
}

func prepareObjects(cfg Config) ([]ObjectView, error) {
	specs, err := LoadObjects(cfg.SchemaDir)
	if err != nil {
		return nil, err
	}
	if len(specs) == 0 {
		fmt.Printf("no active yaml schema files found in %s; generated registries will be empty\n", cfg.SchemaDir)
	}

	objects := make([]ObjectView, 0, len(specs))
	for _, spec := range specs {
		view, err := BuildObjectView(cfg, spec)
		if err != nil {
			return nil, fmt.Errorf("build object %s: %w", spec.Name, err)
		}
		objects = append(objects, view)
	}

	if err := validateObjectViews(objects); err != nil {
		return nil, err
	}
	if cfg.BackendEnabled && !cfg.AllowManualCollisions && len(objects) > 0 {
		if err := validateManualBackendCollisions(cfg.ServerRoot, objects); err != nil {
			return nil, err
		}
	}
	if cfg.FrontendEnabled && !cfg.AllowManualCollisions && len(objects) > 0 {
		if err := validateManualFrontendCollisions(cfg.FrontendRoot, objects); err != nil {
			return nil, err
		}
	}

	return objects, nil
}

func LoadObjects(schemaDir string) ([]ObjectSpec, error) {
	entries, err := os.ReadDir(schemaDir)
	if err != nil {
		return nil, fmt.Errorf("read schema dir %s: %w", schemaDir, err)
	}

	objects := make([]ObjectSpec, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		path := filepath.Join(schemaDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}

		var object ObjectSpec
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		decoder.KnownFields(true)
		if err := decoder.Decode(&object); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		var extra any
		if err := decoder.Decode(&extra); err == nil {
			return nil, fmt.Errorf("parse %s: multiple YAML documents are not supported", path)
		} else if !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		object.SourceFile = path

		if err := object.Validate(); err != nil {
			return nil, fmt.Errorf("validate %s: %w", path, err)
		}

		objects = append(objects, object)
	}

	sort.Slice(objects, func(i int, j int) bool {
		return objects[i].Name < objects[j].Name
	})

	return objects, nil
}

func validateObjectViews(objects []ObjectView) error {
	type owner struct {
		object string
		source string
	}

	registries := map[string]map[string]owner{
		"generated file base":     {},
		"model name":              {},
		"service name":            {},
		"API route":               {},
		"route/permission prefix": {},
		"base relation":           {},
		"application route":       {},
		"migration name":          {},
		"frontend route name":     {},
		"frontend route path":     {},
	}

	add := func(kind string, value string, object ObjectView) error {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil
		}
		if previous, exists := registries[kind][value]; exists {
			return fmt.Errorf(
				"duplicate %s %q for %s (%s) and %s (%s)",
				kind,
				value,
				previous.object,
				sourceLabel(previous.source),
				object.Name,
				sourceLabel(object.SourceFile),
			)
		}
		registries[kind][value] = owner{object: object.Name, source: object.SourceFile}
		return nil
	}

	for _, object := range objects {
		checks := [][2]string{
			{"generated file base", object.FileBase},
			{"model name", object.Name},
			{"service name", object.ServiceName},
			{"API route", object.Route},
			{"route/permission prefix", object.PermissionPrefix},
			{"base relation", object.Relation},
		}
		if object.HasCustomList {
			checks = append(checks,
				[2]string{"model name", object.ListModelName},
				[2]string{"generated file base", object.ListFileBase},
			)
		}
		if object.ApplicationRoute.Enabled {
			checks = append(checks, [2]string{"application route", object.ApplicationRoute.Name})
		}
		if object.Migration.Enabled {
			checks = append(checks, [2]string{"migration name", object.Migration.Name})
		}
		if object.Frontend.List.Enabled {
			checks = append(checks,
				[2]string{"frontend route name", object.Frontend.Routes.ListName},
				[2]string{"frontend route path", object.Frontend.Routes.ListPath},
			)
		}
		if object.Frontend.Form.Enabled {
			checks = append(checks,
				[2]string{"frontend route name", object.Frontend.Routes.CreateName},
				[2]string{"frontend route name", object.Frontend.Routes.EditName},
				[2]string{"frontend route path", object.Frontend.Routes.CreatePath},
				[2]string{"frontend route path", object.Frontend.Routes.EditPath},
			)
		}
		for _, check := range checks {
			if err := add(check[0], check[1], object); err != nil {
				return err
			}
		}
	}

	return nil
}

func sourceLabel(source string) string {
	if strings.TrimSpace(source) == "" {
		return "unknown source"
	}
	return source
}

func NewRenderer(cfg Config) Renderer {
	return Renderer{Config: cfg}
}

type Renderer struct {
	Config Config
}

func (r Renderer) RenderObject(model ObjectView) error {
	if r.Config.BackendEnabled && !r.Config.AllowManualCollisions {
		if err := r.ensureNoManualBackendCollision(model); err != nil {
			return err
		}
	}

	if r.Config.FrontendEnabled && !r.Config.AllowManualCollisions {
		if err := r.ensureNoManualFrontendCollision(model); err != nil {
			return err
		}
	}
	if r.Config.FrontendEnabled {
		if err := r.migrateLegacyFrontendScaffolds(model); err != nil {
			return err
		}
	}

	jobs := make([]RenderJob, 0, 8)
	if r.Config.BackendEnabled {
		jobs = append(jobs,
			RenderJob{
				TemplatePath: filepath.Join(r.Config.TemplateDir, "go", "model.go.tmpl"),
				TargetPath:   filepath.Join(r.Config.ServerRoot, "internal", "models", model.FileBase+".gen.go"),
				Format:       FormatGo,
			},
			RenderJob{
				TemplatePath: filepath.Join(r.Config.TemplateDir, "go", "service.go.tmpl"),
				TargetPath:   filepath.Join(r.Config.ServerRoot, "internal", "services", model.FileBase+".gen.go"),
				Format:       FormatGo,
			},
			RenderJob{
				TemplatePath: filepath.Join(r.Config.TemplateDir, "go", "httpapi.go.tmpl"),
				TargetPath:   filepath.Join(r.Config.ServerRoot, "internal", "httpapi", model.FileBase+".gen.go"),
				Format:       FormatGo,
			},
		)
	}
	if r.Config.APITestEnabled && model.Test.Enabled {
		jobs = append(jobs, RenderJob{
			TemplatePath: filepath.Join(r.Config.TemplateDir, "go", "apitest.go.tmpl"),
			TargetPath:   filepath.Join(r.Config.ServerRoot, "internal", "apitest", model.Snake+"_test.gen.go"),
			Format:       FormatGo,
		})
	}
	if r.Config.FrontendEnabled {
		jobs = append(jobs,
			RenderJob{
				TemplatePath: filepath.Join(r.Config.TemplateDir, "vue", "types.ts.tmpl"),
				TargetPath:   filepath.Join(r.Config.FrontendRoot, "src", "types", model.FileBase+".gen.ts"),
				Format:       FormatNone,
			},
			RenderJob{
				TemplatePath: filepath.Join(r.Config.TemplateDir, "vue", "schemas.ts.tmpl"),
				TargetPath:   filepath.Join(r.Config.FrontendRoot, "src", "schemas", model.FileBase+".gen.ts"),
				Format:       FormatNone,
			},
		)
		if model.HasCustomList {
			jobs = append(jobs,
				RenderJob{
					TemplatePath: filepath.Join(r.Config.TemplateDir, "vue", "listTypes.ts.tmpl"),
					TargetPath:   filepath.Join(r.Config.FrontendRoot, "src", "types", model.ListFileBase+".gen.ts"),
					Format:       FormatNone,
				},
				RenderJob{
					TemplatePath: filepath.Join(r.Config.TemplateDir, "vue", "listSchemas.ts.tmpl"),
					TargetPath:   filepath.Join(r.Config.FrontendRoot, "src", "schemas", model.ListFileBase+".gen.ts"),
					Format:       FormatNone,
				},
			)
		}
		jobs = append(jobs,
			RenderJob{
				TemplatePath: filepath.Join(r.Config.TemplateDir, "vue", "api.ts.tmpl"),
				TargetPath:   filepath.Join(r.Config.FrontendRoot, "src", "api", model.FileBase+".gen.ts"),
				Format:       FormatNone,
			},
			RenderJob{
				TemplatePath: filepath.Join(r.Config.TemplateDir, "vue", "useSchemas.ts.tmpl"),
				TargetPath:   filepath.Join(r.Config.FrontendRoot, "src", "composables", "schemas", "use"+model.Name+"Schemas.gen.ts"),
				Format:       FormatNone,
			},
		)
		if model.Frontend.List.Enabled {
			jobs = append(jobs,
				RenderJob{
					TemplatePath: filepath.Join(r.Config.TemplateDir, "vue", "collection.ts.tmpl"),
					TargetPath:   filepath.Join(r.Config.FrontendRoot, "src", "collections", model.FileBase+".gen.ts"),
					Format:       FormatNone,
				},
				RenderJob{
					TemplatePath: filepath.Join(r.Config.TemplateDir, "vue", "list.vue.tmpl"),
					TargetPath:   filepath.Join(r.Config.FrontendRoot, "src", "views", model.Camel, model.Name+"List.vue"),
					Format:       FormatNone,
					Scaffold:     true,
				},
			)
		}
		if model.Frontend.Form.Enabled {
			jobs = append(jobs,
				RenderJob{
					TemplatePath: filepath.Join(r.Config.TemplateDir, "vue", "formContract.ts.tmpl"),
					TargetPath:   filepath.Join(r.Config.FrontendRoot, "src", "forms", model.FileBase+".gen.ts"),
					Format:       FormatNone,
				},
				RenderJob{
					TemplatePath: filepath.Join(r.Config.TemplateDir, "vue", "form.vue.tmpl"),
					TargetPath:   filepath.Join(r.Config.FrontendRoot, "src", "components", model.Camel, model.Name+"Form.vue"),
					Format:       FormatNone,
					Scaffold:     true,
				},
				RenderJob{
					TemplatePath: filepath.Join(r.Config.TemplateDir, "vue", "editPage.vue.tmpl"),
					TargetPath:   filepath.Join(r.Config.FrontendRoot, "src", "views", model.Camel, model.Name+"EditPage.vue"),
					Format:       FormatNone,
					Scaffold:     true,
				},
			)
		}
	}

	for _, job := range jobs {
		if err := r.renderFile(job, model); err != nil {
			return err
		}
	}

	if !r.Config.MigrationsEnabled || !model.Migration.Enabled {
		return nil
	}
	return r.RenderMigration(model)
}

func (r Renderer) migrateLegacyFrontendScaffolds(model ObjectView) error {
	pairs := [][2]string{
		{
			filepath.Join(r.Config.FrontendRoot, "src", "components", model.Camel, model.Name+"Form.gen.vue"),
			filepath.Join(r.Config.FrontendRoot, "src", "components", model.Camel, model.Name+"Form.vue"),
		},
		{
			filepath.Join(r.Config.FrontendRoot, "src", "views", model.Camel, model.Name+"EditPage.gen.vue"),
			filepath.Join(r.Config.FrontendRoot, "src", "views", model.Camel, model.Name+"EditPage.vue"),
		},
		{
			filepath.Join(r.Config.FrontendRoot, "src", "views", model.Camel, model.Name+"List.gen.vue"),
			filepath.Join(r.Config.FrontendRoot, "src", "views", model.Camel, model.Name+"List.vue"),
		},
	}

	for _, pair := range pairs {
		legacyPath := pair[0]
		manualPath := pair[1]
		_, legacyErr := os.Stat(legacyPath)
		if errors.Is(legacyErr, fs.ErrNotExist) {
			continue
		}
		if legacyErr != nil {
			return fmt.Errorf("check legacy frontend scaffold %s: %w", legacyPath, legacyErr)
		}

		_, manualErr := os.Stat(manualPath)
		manualExists := manualErr == nil
		if manualErr != nil && !errors.Is(manualErr, fs.ErrNotExist) {
			return fmt.Errorf("check manual frontend scaffold %s: %w", manualPath, manualErr)
		}

		if r.Config.Check {
			return fmt.Errorf("check failed: legacy generated frontend scaffold %s still exists; run codegen generate to migrate it to %s", legacyPath, manualPath)
		}
		if manualExists {
			return fmt.Errorf("legacy generated frontend scaffold %s and manual scaffold %s both exist; keep the manual file and remove or archive the legacy .gen.vue file before generating", legacyPath, manualPath)
		}

		if err := os.MkdirAll(filepath.Dir(manualPath), 0o755); err != nil {
			return fmt.Errorf("create manual frontend scaffold directory for %s: %w", manualPath, err)
		}
		if err := os.Rename(legacyPath, manualPath); err != nil {
			return fmt.Errorf("migrate legacy frontend scaffold %s to %s: %w", legacyPath, manualPath, err)
		}
		fmt.Printf("migrated legacy generated scaffold %s -> %s\n", legacyPath, manualPath)
	}

	return nil
}

func (r Renderer) ensureNoManualFrontendCollision(model ObjectView) error {
	paths := []string{
		filepath.Join(r.Config.FrontendRoot, "src", "types", model.FileBase+".ts"),
		filepath.Join(r.Config.FrontendRoot, "src", "schemas", model.FileBase+".ts"),
		filepath.Join(r.Config.FrontendRoot, "src", "api", model.FileBase+".ts"),
		filepath.Join(r.Config.FrontendRoot, "src", "composables", "schemas", "use"+model.Name+"Schemas.ts"),
	}
	if model.HasCustomList {
		paths = append(paths,
			filepath.Join(r.Config.FrontendRoot, "src", "types", model.ListFileBase+".ts"),
			filepath.Join(r.Config.FrontendRoot, "src", "schemas", model.ListFileBase+".ts"),
		)
	}

	collisions := make([]string, 0, len(paths))
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			collisions = append(collisions, path)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("check manual frontend file %s: %w", path, err)
		}
	}
	if len(collisions) == 0 {
		return nil
	}
	return fmt.Errorf(
		"refusing to generate %s because hand-written frontend files already exist: %s; remove the schema, migrate the files to generated ownership, or set CODEGEN_ALLOW_MANUAL_COLLISIONS=true explicitly",
		model.Name,
		strings.Join(collisions, ", "),
	)
}

func (r Renderer) ensureNoManualBackendCollision(model ObjectView) error {
	paths := []string{
		filepath.Join(r.Config.ServerRoot, "internal", "models", model.FileBase+".go"),
		filepath.Join(r.Config.ServerRoot, "internal", "services", model.FileBase+".go"),
		filepath.Join(r.Config.ServerRoot, "internal", "httpapi", model.FileBase+".go"),
	}

	collisions := make([]string, 0, len(paths))
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			collisions = append(collisions, path)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("check manual backend file %s: %w", path, err)
		}
	}

	if len(collisions) == 0 {
		return nil
	}

	return fmt.Errorf(
		"refusing to generate %s because hand-written backend files already exist: %s; remove the schema, migrate the files to generated ownership, or set CODEGEN_ALLOW_MANUAL_COLLISIONS=true explicitly",
		model.Name,
		strings.Join(collisions, ", "),
	)
}

func (r Renderer) RenderRoutes(objects []ObjectView) error {
	job := RenderJob{
		TemplatePath: filepath.Join(r.Config.TemplateDir, "go", "routes_gen.go.tmpl"),
		TargetPath:   filepath.Join(r.Config.ServerRoot, "internal", "httpapi", "routes_gen.go"),
		Format:       FormatGo,
	}

	return r.renderFile(job, RoutesView{Objects: objects})
}

func (r Renderer) RenderServices(objects []ObjectView) error {
	job := RenderJob{
		TemplatePath: filepath.Join(r.Config.TemplateDir, "go", "services_gen.go.tmpl"),
		TargetPath:   filepath.Join(r.Config.ServerRoot, "internal", "services", "services_gen.go"),
		Format:       FormatGo,
	}

	return r.renderFile(job, RoutesView{Objects: objects})
}

func (r Renderer) RenderFrontendRoutes(objects []ObjectView) error {
	frontendObjects := make([]ObjectView, 0, len(objects))
	for _, object := range objects {
		if object.Frontend.List.Enabled || object.Frontend.Form.Enabled {
			frontendObjects = append(frontendObjects, object)
		}
	}
	job := RenderJob{
		TemplatePath: filepath.Join(r.Config.TemplateDir, "vue", "routeManifest.ts.tmpl"),
		TargetPath:   filepath.Join(r.Config.FrontendRoot, "src", "router", "routeManifest.gen.ts"),
		Format:       FormatNone,
	}
	return r.renderFile(job, RoutesView{Objects: frontendObjects})
}

func (r Renderer) RenderFrontendLocale(objects []ObjectView) error {
	frontendObjects := make([]ObjectView, 0, len(objects))
	for _, object := range objects {
		if object.Frontend.List.Enabled || object.Frontend.Form.Enabled {
			frontendObjects = append(frontendObjects, object)
		}
	}
	job := RenderJob{
		TemplatePath: filepath.Join(r.Config.TemplateDir, "vue", "ru.json.tmpl"),
		TargetPath:   filepath.Join(r.Config.FrontendRoot, "src", "locales", "ru.gen.json"),
		Format:       FormatNone,
	}
	return r.renderFile(job, RoutesView{Objects: frontendObjects})
}

func (r Renderer) RenderRegisters(registers []RegisterView) error {
	if r.Config.MigrationsEnabled {
		needsRuntime := false
		for _, register := range registers {
			if register.MigrationEnabled {
				needsRuntime = true
				break
			}
		}
		if needsRuntime {
			if err := r.RenderRegisterRuntimeMigration(BuildRegisterRuntimeView(r.Config)); err != nil {
				return fmt.Errorf("render common runtime: %w", err)
			}
		}
	}

	for _, register := range registers {
		if r.Config.BackendEnabled {
			if err := r.RenderRegisterGo(register); err != nil {
				return fmt.Errorf("render %s Go helpers: %w", register.Name, err)
			}
		}
		if r.Config.MigrationsEnabled && register.MigrationEnabled {
			if err := r.RenderRegisterMigration(register); err != nil {
				return fmt.Errorf("render %s migration: %w", register.Name, err)
			}
		}
	}

	if r.Config.BackendEnabled {
		if err := r.RenderRegisterRegistry(registers); err != nil {
			return fmt.Errorf("render Go registry: %w", err)
		}
	}
	return nil
}

func (r Renderer) RenderRegisterRuntimeMigration(runtime RegisterRuntimeView) error {
	versionDir := fmt.Sprintf("v%d", runtime.Version)
	return r.renderNamedMigration(
		runtime.MigrationName,
		filepath.Join(r.Config.TemplateDir, "register", "runtime", versionDir, "bootstrap.up.sql.tmpl"),
		filepath.Join(r.Config.TemplateDir, "register", "runtime", versionDir, "bootstrap.down.sql.tmpl"),
		runtime,
	)
}

func (r Renderer) RenderRegisterMigration(register RegisterView) error {
	return r.renderScaffoldMigration(
		register.MigrationName,
		filepath.Join(r.Config.TemplateDir, "register", "create.up.sql.tmpl"),
		filepath.Join(r.Config.TemplateDir, "register", "create.down.sql.tmpl"),
		register,
	)
}

func (r Renderer) renderScaffoldMigration(
	migrationName string,
	upTemplatePath string,
	downTemplatePath string,
	data any,
) error {
	pair, existed, err := r.findMigrationPair(migrationName)
	if err != nil {
		return err
	}
	createdMigration := false
	if !existed {
		if r.Config.Check {
			return fmt.Errorf("check failed: migration %s does not exist", migrationName)
		}
		pair, err = r.createMigration(migrationName)
		if err != nil {
			return err
		}
		createdMigration = true
	}

	if existed && r.Config.Check {
		return nil
	}
	if existed && !r.Config.MigrationsOverwrite {
		fmt.Printf("migration exists, kept immutable scaffold %s and %s\n", pair.UpPath, pair.DownPath)
		return nil
	}

	jobs := []RenderJob{
		{TemplatePath: upTemplatePath, TargetPath: pair.UpPath, Format: FormatNone},
		{TemplatePath: downTemplatePath, TargetPath: pair.DownPath, Format: FormatNone},
	}
	for _, job := range jobs {
		if err := r.renderFile(job, data); err != nil {
			if createdMigration {
				_ = os.Remove(pair.UpPath)
				_ = os.Remove(pair.DownPath)
			}
			return err
		}
	}
	return nil
}

func (r Renderer) RenderRegisterGo(register RegisterView) error {
	return r.renderFile(RenderJob{
		TemplatePath: filepath.Join(r.Config.TemplateDir, "register", "register.go.tmpl"),
		TargetPath:   filepath.Join(r.Config.ServerRoot, "internal", "registers", register.FileBase+".gen.go"),
		Format:       FormatGo,
	}, register)
}

func (r Renderer) RenderRegisterRegistry(registers []RegisterView) error {
	return r.renderFile(RenderJob{
		TemplatePath: filepath.Join(r.Config.TemplateDir, "register", "registry.go.tmpl"),
		TargetPath:   filepath.Join(r.Config.ServerRoot, "internal", "registers", "registry.gen.go"),
		Format:       FormatGo,
	}, RegistersView{Registers: registers})
}

func (r Renderer) RenderMigration(model ObjectView) error {
	return r.renderNamedMigration(
		model.Migration.Name,
		filepath.Join(r.Config.TemplateDir, "sql", "create.up.sql.tmpl"),
		filepath.Join(r.Config.TemplateDir, "sql", "create.down.sql.tmpl"),
		model,
	)
}

func (r Renderer) renderNamedMigration(
	migrationName string,
	upTemplatePath string,
	downTemplatePath string,
	data any,
) error {
	pair, existed, err := r.findMigrationPair(migrationName)
	if err != nil {
		return err
	}
	createdMigration := false

	if !existed {
		if r.Config.Check {
			return fmt.Errorf("check failed: migration %s does not exist", migrationName)
		}

		pair, err = r.createMigration(migrationName)
		if err != nil {
			return err
		}
		createdMigration = true
	}

	if existed && !createdMigration && !r.Config.MigrationsOverwrite && !r.Config.Check {
		fmt.Printf("migration exists, skipped %s and %s\n", pair.UpPath, pair.DownPath)
		return nil
	}

	jobs := []RenderJob{
		{
			TemplatePath: upTemplatePath,
			TargetPath:   pair.UpPath,
			Format:       FormatNone,
		},
		{
			TemplatePath: downTemplatePath,
			TargetPath:   pair.DownPath,
			Format:       FormatNone,
		},
	}

	for _, job := range jobs {
		if err := r.renderFile(job, data); err != nil {
			if createdMigration {
				_ = os.Remove(pair.UpPath)
				_ = os.Remove(pair.DownPath)
			}
			return err
		}
	}

	return nil
}

func (r Renderer) createMigration(migrationName string) (MigrationPair, error) {
	if err := os.MkdirAll(r.Config.MigrationsDir, 0o755); err != nil {
		return MigrationPair{}, fmt.Errorf("create migrations dir %s: %w", r.Config.MigrationsDir, err)
	}

	if r.Config.MigrationCreateMode == "external" {
		return r.createMigrationExternal(migrationName)
	}
	return r.createMigrationInternal(migrationName)
}

func (r Renderer) createMigrationInternal(migrationName string) (MigrationPair, error) {
	next, err := r.nextMigrationSequence()
	if err != nil {
		return MigrationPair{}, err
	}

	prefix := fmt.Sprintf("%0*d", r.Config.MigrationSequenceWidth, next)
	base := filepath.Join(r.Config.MigrationsDir, prefix+"_"+migrationName)
	pair := MigrationPair{
		UpPath:   base + ".up." + r.Config.MigrationCreateExt,
		DownPath: base + ".down." + r.Config.MigrationCreateExt,
	}

	createdPaths := make([]string, 0, 2)
	for _, path := range []string{pair.UpPath, pair.DownPath} {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			for _, createdPath := range createdPaths {
				_ = os.Remove(createdPath)
			}
			return MigrationPair{}, fmt.Errorf("create migration file %s: %w", path, err)
		}
		if err := file.Close(); err != nil {
			_ = os.Remove(path)
			for _, createdPath := range createdPaths {
				_ = os.Remove(createdPath)
			}
			return MigrationPair{}, fmt.Errorf("close migration file %s: %w", path, err)
		}
		createdPaths = append(createdPaths, path)
	}

	return pair, nil
}

func (r Renderer) createMigrationExternal(migrationName string) (MigrationPair, error) {
	args := []string{
		"create",
		"-ext",
		r.Config.MigrationCreateExt,
		"-dir",
		r.Config.MigrationsDir,
	}
	if r.Config.MigrationCreateSeq {
		args = append(args, "-seq")
	}
	args = append(args, migrationName)

	cmd := exec.Command(r.Config.MigrationCreateBin, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return MigrationPair{}, fmt.Errorf("run %s %s: %w\n%s", r.Config.MigrationCreateBin, strings.Join(args, " "), err, string(output))
	}
	if len(output) > 0 {
		fmt.Print(string(output))
	}

	pair, existed, err := r.findMigrationPair(migrationName)
	if err != nil {
		return MigrationPair{}, err
	}
	if !existed {
		return MigrationPair{}, fmt.Errorf("migration command completed, but files for %s were not found", migrationName)
	}
	return pair, nil
}

func (r Renderer) nextMigrationSequence() (int, error) {
	entries, err := os.ReadDir(r.Config.MigrationsDir)
	if err != nil {
		return 0, fmt.Errorf("read migrations dir %s: %w", r.Config.MigrationsDir, err)
	}

	maxSequence := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		prefix, _, ok := strings.Cut(name, "_")
		if !ok {
			continue
		}
		sequence, err := strconv.Atoi(prefix)
		if err != nil {
			continue
		}
		if sequence > maxSequence {
			maxSequence = sequence
		}
	}

	return maxSequence + 1, nil
}

func (r Renderer) findMigrationPair(migrationName string) (MigrationPair, bool, error) {
	ext := strings.TrimPrefix(r.Config.MigrationCreateExt, ".")
	upSuffix := ".up." + ext
	downSuffix := ".down." + ext
	upPattern := filepath.Join(r.Config.MigrationsDir, "*_"+migrationName+upSuffix)
	downPattern := filepath.Join(r.Config.MigrationsDir, "*_"+migrationName+downSuffix)

	upFiles, err := filepath.Glob(upPattern)
	if err != nil {
		return MigrationPair{}, false, fmt.Errorf("glob migration pattern %s: %w", upPattern, err)
	}
	downFiles, err := filepath.Glob(downPattern)
	if err != nil {
		return MigrationPair{}, false, fmt.Errorf("glob migration pattern %s: %w", downPattern, err)
	}

	pairs := make(map[string]MigrationPair, len(upFiles)+len(downFiles))
	for _, upPath := range upFiles {
		basePath := strings.TrimSuffix(upPath, upSuffix)
		pair := pairs[basePath]
		pair.UpPath = upPath
		pairs[basePath] = pair
	}
	for _, downPath := range downFiles {
		basePath := strings.TrimSuffix(downPath, downSuffix)
		pair := pairs[basePath]
		pair.DownPath = downPath
		pairs[basePath] = pair
	}

	bases := make([]string, 0, len(pairs))
	for basePath := range pairs {
		bases = append(bases, basePath)
	}
	sort.Strings(bases)
	for _, basePath := range bases {
		pair := pairs[basePath]
		if pair.UpPath == "" {
			return MigrationPair{}, false, fmt.Errorf("incomplete migration %s: missing up file for %s", migrationName, pair.DownPath)
		}
		if pair.DownPath == "" {
			return MigrationPair{}, false, fmt.Errorf("incomplete migration %s: missing down file for %s", migrationName, pair.UpPath)
		}
	}
	if len(bases) == 0 {
		return MigrationPair{}, false, nil
	}

	return pairs[bases[len(bases)-1]], true, nil
}

type MigrationPair struct {
	UpPath   string
	DownPath string
}

func (r Renderer) renderFile(job RenderJob, data any) error {
	if job.Scaffold {
		_, err := os.Stat(job.TargetPath)
		if err == nil {
			if !r.Config.Check {
				fmt.Printf("kept manual scaffold %s\n", job.TargetPath)
			}
			return nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("check scaffold %s: %w", job.TargetPath, err)
		}
		if r.Config.Check {
			return fmt.Errorf("check failed: scaffold %s does not exist", job.TargetPath)
		}
	}

	content, err := r.executeTemplate(job.TemplatePath, data)
	if err != nil {
		return err
	}

	if job.Format == FormatGo {
		content, err = format.Source(content)
		if err != nil {
			return fmt.Errorf("format go template %s: %w\n%s", job.TemplatePath, err, string(content))
		}
	}

	if r.Config.Check {
		existing, err := os.ReadFile(job.TargetPath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("check failed: %s does not exist", job.TargetPath)
			}
			return fmt.Errorf("read existing %s: %w", job.TargetPath, err)
		}
		if !bytes.Equal(existing, content) {
			return fmt.Errorf("check failed: %s is outdated", job.TargetPath)
		}

		return nil
	}

	if err := os.MkdirAll(filepath.Dir(job.TargetPath), 0o755); err != nil {
		return fmt.Errorf("create dir for %s: %w", job.TargetPath, err)
	}
	if err := writeFileAtomic(job.TargetPath, content, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", job.TargetPath, err)
	}

	fmt.Printf("generated %s\n", job.TargetPath)
	return nil
}

func writeFileAtomic(path string, content []byte, mode fs.FileMode) error {
	dir := filepath.Dir(path)
	temporary, err := os.CreateTemp(dir, ".codegen-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	cleanup := func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}

	if err := temporary.Chmod(mode); err != nil {
		cleanup()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		cleanup()
		return err
	}
	if err := temporary.Close(); err != nil {
		_ = os.Remove(temporaryPath)
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		_ = os.Remove(temporaryPath)
		return err
	}

	return nil
}

func (r Renderer) executeTemplate(path string, data any) ([]byte, error) {
	content, templateLabel, err := r.readTemplate(path)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New(filepath.Base(path)).Funcs(TemplateFuncs()).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", templateLabel, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template %s: %w", templateLabel, err)
	}

	return buf.Bytes(), nil
}

func (r Renderer) readTemplate(path string) ([]byte, string, error) {
	if strings.TrimSpace(r.Config.TemplateDir) != "" {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, path, fmt.Errorf("read template %s: %w", path, err)
		}
		return content, path, nil
	}

	embeddedPath := filepath.ToSlash(filepath.Join("templates", path))
	content, err := defaultTemplates.ReadFile(embeddedPath)
	if err != nil {
		return nil, embeddedPath, fmt.Errorf("read embedded template %s: %w", embeddedPath, err)
	}
	return content, embeddedPath, nil
}

type RenderJob struct {
	TemplatePath string
	TargetPath   string
	Format       OutputFormat
	Scaffold     bool
}

type OutputFormat string

const (
	FormatNone OutputFormat = "none"
	FormatGo   OutputFormat = "go"
)
