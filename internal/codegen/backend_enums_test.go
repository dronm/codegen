package codegen

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func backendEnumConfig(t *testing.T) Config {
	t.Helper()
	cfg := enumConfig(t)
	cfg.FrontendEnabled = false
	cfg.MigrationsDir = filepath.Join(cfg.ServerRoot, "migrations")
	return cfg
}

func backendEnumObject(t *testing.T, cfg Config) ObjectView {
	t.Helper()
	view, err := BuildObjectView(cfg, enumObjectSpec())
	if err != nil {
		t.Fatal(err)
	}
	return view
}

func writeBackendEnumTestFile(t *testing.T, name, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBackendEnumRenderingAndCheck(t *testing.T) {
	cfg := backendEnumConfig(t)
	view := backendEnumObject(t, cfg)
	renderer := NewRenderer(cfg)
	if err := renderer.RenderBackendEnums([]ObjectView{view}); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(cfg.ServerRoot, "internal/models/materialStatusType.enum.gen.go")
	data := readTestFile(t, filename)
	for _, want := range []string{
		backendEnumGeneratedHeader,
		"type MaterialStatusType string",
		`MaterialStatusTypeAtWork        MaterialStatusType = "at_work"`,
		`MaterialStatusTypeOnMaintenance MaterialStatusType = "on_maintenance"`,
		"func MaterialStatusTypeValues() []string", "func (v MaterialStatusType) IsValid() bool", "func (v MaterialStatusType) String() string",
	} {
		if !strings.Contains(data, want) {
			t.Fatalf("missing %q in generated enum:\n%s", want, data)
		}
	}
	registry := readTestFile(t, filepath.Join(cfg.ServerRoot, "internal/models/enums_registry.gen.go"))
	for _, want := range []string{`"github.com/dronm/modelbind/metadata"`, "func init()", "RegisterGeneratedModelbindEnums()", `name: "material_status_type"`, "metadata.Enums[definition.name] = definition.values", "slices.Equal"} {
		// gofmt may align struct fields with whitespace.
		if !strings.Contains(strings.Join(strings.Fields(registry), " "), strings.Join(strings.Fields(want), " ")) {
			t.Fatalf("missing %q in registry:\n%s", want, registry)
		}
	}
	check := cfg
	check.Check = true
	if err := NewRenderer(check).RenderBackendEnums([]ObjectView{view}); err != nil {
		t.Fatal(err)
	}
	writeBackendEnumTestFile(t, filename, data+"// edited\n")
	if err := NewRenderer(check).RenderBackendEnums([]ObjectView{view}); err == nil || !strings.Contains(err.Error(), "outdated") {
		t.Fatalf("changed enum must fail check: %v", err)
	}
}

func TestBackendEnumSharedOnceAndProjections(t *testing.T) {
	cfg := backendEnumConfig(t)
	spec := enumObjectSpec()
	spec.List.Fields[4].Nullable = true
	spec.List.Fields[4].Required = false
	spec.Detail.Fields[4].Nullable = true
	spec.Detail.Fields[4].Required = false
	spec.Fields[3].Nullable = true
	spec.Fields[3].Required = false
	first, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatal(err)
	}
	second := first
	second.Name, second.FileBase, second.Camel = "OtherStatus", "otherStatus", "otherStatus"
	second.ListModelName, second.DetailModelName = "OtherStatusList", "OtherStatusDetail"
	if err := NewRenderer(cfg).RenderBackendEnums([]ObjectView{second, first}); err != nil {
		t.Fatal(err)
	}
	manifest, _, err := readBackendEnumManifest(cfg.ServerRoot)
	if err != nil || len(manifest.Files) != 2 {
		t.Fatalf("shared type and registry must be generated once: %+v, %v", manifest, err)
	}
	for _, fields := range [][]FieldView{first.Fields, first.ListFields, first.DetailFields} {
		for _, field := range fields {
			if field.Name == "status" && (field.GoType != "*MaterialStatusType" || !strings.Contains(field.StructTag, `enum:"material_status_type"`)) {
				t.Fatalf("nullable projection lost backend enum: %+v", field)
			}
		}
	}
}

func TestBackendEnumManualOwnershipAndRegistrationOptOut(t *testing.T) {
	cfg := backendEnumConfig(t)
	spec := enumObjectSpec()
	spec.Enums[0].Backend.Generation = "manual"
	view, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatal(err)
	}
	manual := filepath.Join(cfg.ServerRoot, "internal/models/status.go")
	original := "package models\n\ntype MaterialStatusType string\n"
	writeBackendEnumTestFile(t, manual, original)
	if err := NewRenderer(cfg).RenderBackendEnums([]ObjectView{view}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cfg.ServerRoot, backendEnumManifestPath)); !os.IsNotExist(err) {
		t.Fatal("manual enums must not create managed artifacts by default")
	}
	if readTestFile(t, manual) != original {
		t.Fatal("manual enum changed")
	}
	yes := true
	spec.Enums[0].Backend.Register = &yes
	view, err = BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := NewRenderer(cfg).RenderBackendEnums([]ObjectView{view}); err != nil {
		t.Fatal(err)
	}
	manifest, _, _ := readBackendEnumManifest(cfg.ServerRoot)
	if len(manifest.Files) != 1 || manifest.Files[0].Path != "internal/models/enums_registry.gen.go" {
		t.Fatalf("explicit registration should generate registry only: %+v", manifest)
	}
}

func TestBackendEnumCollisionValidation(t *testing.T) {
	cases := []struct{ name, source string }{
		{"type", "type MaterialStatusType string"},
		{"helper", "func MaterialStatusTypeValues() []string { return nil }"},
		{"constant", "const MaterialStatusTypeAtWork = 1"},
		{"registry", "func RegisterGeneratedModelbindEnums() {}"},
		{"method", "func (v MaterialStatusType) IsValid() bool { return true }"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := backendEnumConfig(t)
			cfg.AllowManualCollisions = true // must not bypass enum protection
			view := backendEnumObject(t, cfg)
			writeBackendEnumTestFile(t, filepath.Join(cfg.ServerRoot, "internal/models/manual.go"), "package models\n"+tc.source+"\n")
			if err := NewRenderer(cfg).RenderBackendEnums([]ObjectView{view}); err == nil || !strings.Contains(err.Error(), "conflict") && !strings.Contains(err.Error(), "already exists") {
				t.Fatalf("expected collision before writes, got %v", err)
			}
			if _, err := os.Stat(filepath.Join(cfg.ServerRoot, backendEnumManifestPath)); !os.IsNotExist(err) {
				t.Fatal("failed validation wrote output")
			}
		})
	}
}

func TestBackendEnumNameValidation(t *testing.T) {
	cases := []struct {
		name string
		edit func(*EnumSpec)
		want string
	}{
		{"mode", func(e *EnumSpec) { e.Backend.Generation = "invalid" }, "backend.generation"},
		{"builtin", func(e *EnumSpec) { e.GoType = "string" }, "predeclared"},
		{"constant collision", func(e *EnumSpec) { e.Values[1].Value = "at-work" }, "duplicate generated"},
		{"helper collision", func(e *EnumSpec) { e.Values[0].Value = "values" }, "duplicate generated"},
		{"invalid name", func(e *EnumSpec) { e.Values[0].GoConst = "not-a-name" }, "Go constant"},
		{"sql qualification", func(e *EnumSpec) { e.SQLType = "status_types"; e.Migration.Enabled = true }, "schema-qualified"},
		{"long label", func(e *EnumSpec) { e.Values[0].Value = strings.Repeat("я", 32); e.Migration.Enabled = true }, "63 bytes"},
		{"null label", func(e *EnumSpec) {
			e.Values[0].Value = "a\x00b"
			e.Values[0].GoConst = "HasNUL"
			e.Migration.Enabled = true
		}, "NUL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enum := enumObjectSpec().Enums[0]
			tc.edit(&enum)
			if err := validateBackendEnumDefinition(enum); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
	enum := enumObjectSpec().Enums[0]
	enum.Values[1].Value = "at-work"
	enum.Values[1].GoConst = "MaterialStatusTypeAtWorkLegacy"
	if err := validateEnumDefinition(enum); err != nil {
		t.Fatalf("explicit constant should disambiguate values: %v", err)
	}
	second := enum
	second.Name, second.FrontendName = "another_status", "AnotherStatus"
	if err := validateBackendEnumRegistry(map[string]EnumSpec{enum.Name: enum, second.Name: second}); err == nil {
		t.Fatal("two managed enums must not share Go identifiers")
	}
}

func TestBackendEnumStaleCleanupAndManualPreservation(t *testing.T) {
	cfg := backendEnumConfig(t)
	view := backendEnumObject(t, cfg)
	renderer := NewRenderer(cfg)
	if err := renderer.RenderBackendEnums([]ObjectView{view}); err != nil {
		t.Fatal(err)
	}
	manual := filepath.Join(cfg.ServerRoot, "internal/models/keep.go")
	writeBackendEnumTestFile(t, manual, "package models\nconst Keep = true\n")
	check := cfg
	check.Check = true
	if err := NewRenderer(check).RenderBackendEnums(nil); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("stale files must fail check: %v", err)
	}
	if err := renderer.RenderBackendEnums(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(manual); err != nil {
		t.Fatal("manual file deleted")
	}
	if err := NewRenderer(check).RenderBackendEnums(nil); err != nil {
		t.Fatal(err)
	}
}

func TestBackendEnumRefusesModifiedStaleAndUnownedTargets(t *testing.T) {
	cfg := backendEnumConfig(t)
	view := backendEnumObject(t, cfg)
	renderer := NewRenderer(cfg)
	filename := filepath.Join(cfg.ServerRoot, "internal/models/materialStatusType.enum.gen.go")
	writeBackendEnumTestFile(t, filename, "package models\n// manual\n")
	if err := renderer.RenderBackendEnums([]ObjectView{view}); err == nil || !strings.Contains(err.Error(), "developer-owned") {
		t.Fatalf("unowned target should be protected: %v", err)
	}
	if err := os.Remove(filename); err != nil {
		t.Fatal(err)
	}
	if err := renderer.RenderBackendEnums([]ObjectView{view}); err != nil {
		t.Fatal(err)
	}
	data := readTestFile(t, filename)
	writeBackendEnumTestFile(t, filename, data+"// manual changes\n")
	if err := renderer.RenderBackendEnums(nil); err == nil || !strings.Contains(err.Error(), "modified stale") {
		t.Fatalf("modified stale file should be protected: %v", err)
	}
}

func TestBackendEnumManifestAndSymlinkSafety(t *testing.T) {
	for _, name := range []string{"../outside.enum.gen.go", "internal/models/../../outside.enum.gen.go", "internal/models/manual.go", "internal/models/nested/x.enum.gen.go", "internal\\models\\x.enum.gen.go"} {
		if safeBackendEnumArtifactPath(name) {
			t.Fatalf("unsafe artifact path accepted: %q", name)
		}
	}
	cfg := backendEnumConfig(t)
	view := backendEnumObject(t, cfg)
	if err := os.MkdirAll(filepath.Join(cfg.ServerRoot, "internal"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(cfg.ServerRoot, "internal/models")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := NewRenderer(cfg).RenderBackendEnums([]ObjectView{view}); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlinked output should be refused: %v", err)
	}
}

func TestBackendEnumNoOutputsForLegacyFields(t *testing.T) {
	cfg := backendEnumConfig(t)
	spec := ObjectSpec{
		Name: "Legacy", Table: TableSpec{Name: "legacy"}, Route: "/legacy",
		Keys:   []KeySpec{{Name: "id", Type: "int"}},
		Fields: []FieldSpec{{Name: "id", Type: "int", PrimaryKey: true}, {Name: "role", Type: "enum", Enum: "role_id", GoType: "RoleID"}},
		CRUD:   CRUDSpec{Detail: true},
	}
	view, err := BuildObjectView(cfg, spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := NewRenderer(cfg).RenderBackendEnums([]ObjectView{view}); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(cfg.ServerRoot)
	if len(entries) != 0 {
		t.Fatalf("legacy backend fields should not create enum declarations: %v", entries)
	}
}

func TestBackendEnumMigrationsOrderingAndImmutability(t *testing.T) {
	cfg := backendEnumConfig(t)
	cfg.MigrationsEnabled, cfg.MigrationsOverwrite = true, true
	view := backendEnumObject(t, cfg)
	view.Migration.Enabled, view.Migration.Name = true, "create_material_statuses"
	view.BackendEnums[0].MigrationEnabled = true
	renderer := NewRenderer(cfg)
	if err := renderer.RenderBackendEnumMigrations([]ObjectView{view}); err != nil {
		t.Fatal(err)
	}
	enum := view.BackendEnums[0]
	pair, exists, err := renderer.findMigrationPair(enum.MigrationName)
	if err != nil || !exists {
		t.Fatalf("missing migration: %v", err)
	}
	original := readTestFile(t, pair.UpPath)
	if !strings.Contains(original, "CREATE TYPE public.material_status_types AS ENUM") || !strings.Contains(original, "'at_work'") {
		t.Fatal(original)
	}
	down := readTestFile(t, pair.DownPath)
	if !strings.Contains(down, "DROP TYPE public.material_status_types RESTRICT;") || strings.Contains(down, "CASCADE;") {
		t.Fatal(down)
	}
	if err := renderer.RenderMigration(view); err != nil {
		t.Fatal(err)
	}
	if err := validateBackendEnumMigrationOrder(cfg, []ObjectView{view}, view.BackendEnums); err != nil {
		t.Fatal(err)
	}
	view.BackendEnums[0].Values = append(view.BackendEnums[0].Values, BackendEnumValueView{Value: "retired", Const: "MaterialStatusTypeRetired"})
	if err := renderer.RenderBackendEnumMigrations([]ObjectView{view}); err != nil {
		t.Fatal(err)
	}
	if readTestFile(t, pair.UpPath) != original {
		t.Fatal("existing CREATE TYPE migration was rewritten")
	}
	check := cfg
	check.Check = true
	if err := NewRenderer(check).RenderBackendEnumMigrations([]ObjectView{view}); err != nil {
		t.Fatalf("check must not compare historical enum values with current ones: %v", err)
	}
}

func TestBackendEnumMigrationsRefuseWrongOrder(t *testing.T) {
	cfg := backendEnumConfig(t)
	cfg.MigrationsEnabled = true
	view := backendEnumObject(t, cfg)
	view.Migration.Enabled, view.Migration.Name = true, "create_material_statuses"
	view.BackendEnums[0].MigrationEnabled = true
	renderer := NewRenderer(cfg)
	if err := renderer.RenderMigration(view); err != nil {
		t.Fatal(err)
	}
	if err := renderer.RenderBackendEnumMigrations([]ObjectView{view}); err == nil || !strings.Contains(err.Error(), "cannot append") {
		t.Fatalf("must not append CREATE TYPE after table: %v", err)
	}
	view.Migration.Enabled = false
	if err := renderer.RenderBackendEnumMigrations([]ObjectView{view}); err != nil {
		t.Fatal(err)
	}
	view.Migration.Enabled = true
	if err := renderer.RenderBackendEnumMigrations([]ObjectView{view}); err == nil || !strings.Contains(err.Error(), "must precede") {
		t.Fatalf("must reject existing incorrect order: %v", err)
	}
}

// Compile and execute the actual generated enum declarations without third-party
// dependencies. JSON uses the standard named-string representation.
func TestBackendEnumGeneratedTypeRuntime(t *testing.T) {
	cfg := backendEnumConfig(t)
	view := backendEnumObject(t, cfg)
	view.BackendEnums[0].Register = false
	if err := NewRenderer(cfg).RenderBackendEnums([]ObjectView{view}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(cfg.ServerRoot, "internal/models")
	writeBackendEnumTestFile(t, filepath.Join(dir, "go.mod"), "module example.com/enumfixture\n\ngo 1.23.0\n")
	writeBackendEnumTestFile(t, filepath.Join(dir, "runtime_test.go"), `package models
import (
	"encoding/json"
	"reflect"
	"testing"
)
func TestValuesAndJSON(t *testing.T) {
	values := MaterialStatusTypeValues()
	if !reflect.DeepEqual(values, []string{"at_work", "on_maintenance"}) { t.Fatal(values) }
	values[0] = "modified"
	if MaterialStatusTypeValues()[0] != "at_work" { t.Fatal("shared mutable slice") }
	if !MaterialStatusTypeAtWork.IsValid() || MaterialStatusType("invalid").IsValid() || MaterialStatusType("").IsValid() { t.Fatal("validation") }
	var data struct { Status MaterialStatusType; Optional *MaterialStatusType }
	if err := json.Unmarshal([]byte("{\"Status\":\"at_work\",\"Optional\":null}"), &data); err != nil { t.Fatal(err) }
	if data.Status != MaterialStatusTypeAtWork || data.Optional != nil { t.Fatal(data) }
	encoded, err := json.Marshal(data.Status)
	if err != nil || string(encoded) != "\"at_work\"" || data.Status.String() != "at_work" { t.Fatal(string(encoded), err) }
}
`)
	runBackendEnumFixture(t, dir)
}

// Exercise the generated initialization/merge logic against an in-memory registry
// by replacing only metadata.Enums and its import in an AST. This is a unit test,
// not an integration test of the third-party modelbind request validator.
func TestBackendEnumGeneratedRegistryRuntime(t *testing.T) {
	cfg := backendEnumConfig(t)
	view := backendEnumObject(t, cfg)
	if err := NewRenderer(cfg).RenderBackendEnums([]ObjectView{view}); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	filename := filepath.Join(cfg.ServerRoot, "internal/models/enums_registry.gen.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range file.Decls {
		if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.IMPORT {
			specs := gen.Specs[:0]
			for _, spec := range gen.Specs {
				if spec.(*ast.ImportSpec).Path.Value != `"github.com/dronm/modelbind/metadata"` {
					specs = append(specs, spec)
				}
			}
			gen.Specs = specs
		}
	}
	// Keep selector expressions unchanged; bind their base to a local fixture.
	var source bytes.Buffer
	if err := format.Node(&source, fset, file); err != nil {
		t.Fatal(err)
	}
	writeBackendEnumTestFile(t, filepath.Join(dir, "registry.go"), source.String()+"\nvar metadata = struct { Enums map[string][]string }{}\n")
	writeBackendEnumTestFile(t, filepath.Join(dir, "go.mod"), "module example.com/registryfixture\n\ngo 1.23.0\n")
	writeBackendEnumTestFile(t, filepath.Join(dir, "registry_test.go"), `package models
import "testing"
func TestRegistration(t *testing.T) {
	if len(metadata.Enums["material_status_type"]) != 2 { t.Fatal("init did not register enum") }
	metadata.Enums["manual"] = []string{"kept"}
	RegisterGeneratedModelbindEnums()
	if metadata.Enums["manual"][0] != "kept" { t.Fatal("manual registry erased") }
	metadata.Enums = nil
	RegisterGeneratedModelbindEnums()
	metadata.Enums["material_status_type"] = []string{"conflict"}
	defer func() {
		if recover() == nil { t.Fatal("conflict must fail") }
		if metadata.Enums["material_status_type"][0] != "conflict" { t.Fatal("conflict overwritten") }
	}()
	RegisterGeneratedModelbindEnums()
}
`)
	runBackendEnumFixture(t, dir)
}

func runBackendEnumFixture(t *testing.T, dir string) {
	t.Helper()
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("Go executable unavailable for generated-code compilation")
	}
	cmd := exec.Command(goBin, "test", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local", "GOPROXY=off", "GOFLAGS=")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated enum runtime fixture: %v\n%s", err, output)
	}
}

// This test uses the real YAML decoder and Generate/Validate/Check entry points.
func TestBackendEnumYAMLGenerationIntegration(t *testing.T) {
	cfg := backendEnumConfig(t)
	cfg.MigrationsEnabled = true
	data, err := os.ReadFile(filepath.Join("..", "..", "examples", "material_status_enums.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Replace(data, []byte("    values:\n"), []byte("    backend:\n      generation: managed\n      register: true\n    migration:\n      enabled: true\n    values:\n"), 1)
	writeBackendEnumTestFile(t, filepath.Join(cfg.SchemaDir, "materialStatus.yaml"), string(data))
	if err := Validate(cfg); err != nil {
		t.Fatal(err)
	}
	if err := Generate(cfg); err != nil {
		t.Fatal(err)
	}
	manifest, _, err := readBackendEnumManifest(cfg.ServerRoot)
	if err != nil || len(manifest.Files) != 2 {
		t.Fatalf("missing enum outputs: %+v, %v", manifest, err)
	}
	model := readTestFile(t, filepath.Join(cfg.ServerRoot, "internal/models/materialStatus.gen.go"))
	if !strings.Contains(model, "MaterialStatusType") || !strings.Contains(model, `enum:"material_status_type"`) {
		t.Fatal("generated model enum lost")
	}
	cfg.Check = true
	if err := Generate(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestBackendEnumManifestDeterministic(t *testing.T) {
	cfg := backendEnumConfig(t)
	view := backendEnumObject(t, cfg)
	renderer := NewRenderer(cfg)
	if err := renderer.RenderBackendEnums([]ObjectView{view}); err != nil {
		t.Fatal(err)
	}
	first := readTestFile(t, filepath.Join(cfg.ServerRoot, backendEnumManifestPath))
	if err := renderer.RenderBackendEnums([]ObjectView{view}); err != nil {
		t.Fatal(err)
	}
	second := readTestFile(t, filepath.Join(cfg.ServerRoot, backendEnumManifestPath))
	if first != second {
		t.Fatal("manifest is not deterministic")
	}
	var decoded backendEnumManifest
	if err := json.Unmarshal([]byte(first), &decoded); err != nil || !reflect.DeepEqual(decoded.Version, 1) {
		t.Fatal(err)
	}
}
