package codegen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Compile the generated route, service and projection against real grouped-
// filtering dependencies, then exercise their HTTP contract and captured SQL.
// Only the database I/O seam is replaced; no library APIs are stubbed.
func TestCollectionGeneratedApplicationIntegration(t *testing.T) {
	webappDir := os.Getenv("CODEGEN_COLLECTION_WEBAPP_DIR")
	modelbindDir := os.Getenv("CODEGEN_COLLECTION_MODELBIND_DIR")
	if webappDir == "" && modelbindDir == "" {
		t.Skip("set CODEGEN_COLLECTION_WEBAPP_DIR and CODEGEN_COLLECTION_MODELBIND_DIR to run native grouped collection integration tests")
	}
	if webappDir == "" || modelbindDir == "" {
		t.Fatal("both grouped-filter-capable library directories are required")
	}
	cfg := accessTestConfig(t)
	mod := "module example.com/project\n\ngo 1.25.0\n\n" +
		"// These versions are local-replacement placeholders, not release requirements.\n" +
		"require (\n\tgithub.com/dronm/webapp v0.0.0\n\tgithub.com/dronm/modelbind v0.0.0\n)\n"
	for _, dependency := range []struct{ module, directory string }{
		{"github.com/dronm/webapp", webappDir},
		{"github.com/dronm/modelbind", modelbindDir},
		{"github.com/dronm/ds/v4", os.Getenv("CODEGEN_COLLECTION_DS_DIR")},
		{"github.com/dronm/session", os.Getenv("CODEGEN_COLLECTION_SESSION_DIR")},
	} {
		if dependency.directory == "" {
			continue
		}
		absolute, err := filepath.Abs(dependency.directory)
		if err != nil {
			t.Fatal(err)
		}
		actual, err := modulePathFromGoMod(filepath.Join(absolute, "go.mod"))
		if err != nil || actual != dependency.module {
			t.Fatalf("%s: expected module %s; got %s: %v", absolute, dependency.module, actual, err)
		}
		mod += fmt.Sprintf("\nreplace %s => %s\n", dependency.module, strconv.Quote(absolute))
	}
	writeAccessTestFile(t, cfg.ServerRoot, "go.mod", mod)
	writeAccessTestFile(t, cfg.ServerRoot, "schema/cursorItem.yaml", `name: CursorItem
table: {name: cursor_items}
route: /cursor-items
sessionRequired: false
permissions: {enabled: false}
crud: {list: true}
collection: {groupedFilters: true}
keys: [{name: item_code, type: int}]
fields:
  - {name: item_code, type: int, primaryKey: true}
  - {name: priority, type: int}
list:
  model: CursorItemList
  table: {name: cursor_items}
  fields:
    - {name: item_code, type: int}
    - {name: priority, type: int}
`)
	fixture, err := os.ReadFile("testdata/collection/runtime_test.go.txt")
	if err != nil {
		t.Fatal(err)
	}
	writeAccessTestFile(t, cfg.ServerRoot, "internal/httpapi/collection_runtime_test.go", string(fixture))
	if err := Generate(cfg); err != nil {
		t.Fatal(err)
	}
	if err := Generate(cfg); err != nil {
		t.Fatal(err)
	}
	check := cfg
	check.Check = true
	if err := Generate(check); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"test", "-mod=mod", "-count=1", "./..."}, {"vet", "-mod=mod", "./..."}} {
		command := exec.Command("go", args...)
		command.Dir = cfg.ServerRoot
		command.Env = append(os.Environ(), "GOWORK=off")
		output, err := command.CombinedOutput()
		t.Logf("go %s\n%s", strings.Join(args, " "), output)
		if err != nil {
			t.Fatalf("native grouped collection integration failed: %v\n%s", err, output)
		}
	}
}
