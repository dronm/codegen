package codegen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// This test compiles and executes a generated application against REAL policy-
// enabled library source trees. No dependency shims or altered Go directives.
// Optional for ordinary generator development; mandatory for an access release.
func TestAccessGeneratedApplicationIntegration(t *testing.T) {
	webappDir, modelbindDir := os.Getenv("CODEGEN_ACCESS_WEBAPP_DIR"), os.Getenv("CODEGEN_ACCESS_MODELBIND_DIR")
	if webappDir == "" && modelbindDir == "" {
		t.Skip("set CODEGEN_ACCESS_WEBAPP_DIR and CODEGEN_ACCESS_MODELBIND_DIR to run native generated-application tests")
	}
	if webappDir == "" || modelbindDir == "" {
		t.Fatal("both policy-enabled library directories are required")
	}
	cfg := accessTestConfig(t)
	t.Log("generated integration application:", cfg.ServerRoot)
	mod := "module example.com/project\n\ngo 1.25.0\n\n" +
		"// v0.0.0 below is a LOCAL-REPLACEMENT placeholder, not a published policy release.\n" +
		"require (\n\tgithub.com/dronm/webapp v0.0.0\n\tgithub.com/dronm/modelbind v0.0.0\n)\n"
	for _, dep := range []struct{ module, dir string }{
		{"github.com/dronm/webapp", webappDir},
		{"github.com/dronm/modelbind", modelbindDir},
		{"github.com/dronm/ds/v4", os.Getenv("CODEGEN_ACCESS_DS_DIR")},
		{"github.com/dronm/session", os.Getenv("CODEGEN_ACCESS_SESSION_DIR")},
	} {
		if dep.dir == "" {
			continue
		}
		absolute, err := filepath.Abs(dep.dir)
		if err != nil {
			t.Fatal(err)
		}
		actual, err := modulePathFromGoMod(filepath.Join(absolute, "go.mod"))
		if err != nil || actual != dep.module {
			t.Fatalf("%s: expected module %s; got %s: %v", absolute, dep.module, actual, err)
		}
		mod += fmt.Sprintf("\nreplace %s => %s\n", dep.module, strconv.Quote(absolute))
	}
	writeAccessTestFile(t, cfg.ServerRoot, "go.mod", mod)
	copyFixture := func(source, target string) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("testdata/access", source))
		if err != nil {
			t.Fatal(err)
		}
		writeAccessTestFile(t, cfg.ServerRoot, target, string(data))
	}
	// Handwritten signatures must exist BEFORE generation for AST inspection.
	copyFixture("transport_service.go.txt", "internal/services/transport_manual.go")
	copyFixture("runtime_test.go.txt", "internal/httpapi/access_runtime_test.go")
	bootstrap, err := os.ReadFile(filepath.Join("..", "..", "examples", "access", "bootstrap.go.txt"))
	if err != nil {
		t.Fatal(err)
	}
	writeAccessTestFile(t, cfg.ServerRoot, "internal/accesssetup/bootstrap.go", string(bootstrap))

	order := accessTestSpec()
	order.Permissions.Enabled = boolPointer(false)
	transport := accessTestSpec()
	transport.Name, transport.Route, transport.Table.Name = "TransportOrder", "/transport-orders", "transport_orders"
	transport.List, transport.Detail = nil, nil
	transport.Access.Bindings.List, transport.Access.Bindings.Detail = nil, nil
	transport.Service.ManualMethods = []string{"create", "update"}
	transport.Access.ServiceInputs = map[string]AccessServiceInputSpec{
		"Submit":       {Operation: "insert"},
		"Patch":        {Operation: "update"},
		"Whole":        {Operation: "insert"},
		"WholeValue":   {Operation: "insert"},
		"ReplaceValue": {Operation: "update"},
	}
	transport.Permissions.Enabled = boolPointer(false)
	line := accessTestSpec()
	line.Name, line.Route, line.Table.Name = "OrderLine", "/order-lines", "order_lines"
	line.List, line.Detail = nil, nil
	line.Access.Bindings.List, line.Access.Bindings.Detail = nil, nil
	line.Keys = []KeySpec{{Name: "document_id", PathName: "document", Type: "int"}, {Name: "line_num", PathName: "line", Type: "int"}}
	line.Fields = append([]FieldSpec{
		{Name: "document_id", Type: "int", PrimaryKey: true, Required: true},
		{Name: "line_num", Type: "int", PrimaryKey: true, Required: true},
	}, line.Fields[1:]...)
	line.Permissions.Enabled = boolPointer(false)
	hidden := accessTestSpec()
	hidden.Name, hidden.Route, hidden.Table.Name = "HiddenOrder", "/hidden-orders", "hidden_orders"
	hidden.List, hidden.Detail = nil, nil
	hidden.Access.Bindings.List, hidden.Access.Bindings.Detail = nil, nil
	hidden.Fields = append(hidden.Fields[:1], hidden.Fields[2:]...)
	hidden.Permissions.Enabled = boolPointer(false)
	for _, spec := range []ObjectSpec{order, transport, line, hidden} {
		data, err := yaml.Marshal(spec)
		if err != nil {
			t.Fatal(err)
		}
		writeAccessTestFile(t, cfg.ServerRoot, "schema/"+CamelCase(spec.Name)+".yaml", string(data))
	}
	if err := Generate(cfg); err != nil {
		t.Fatal(err)
	}
	check := cfg
	check.Check = true
	if err := Generate(check); err != nil {
		t.Fatal(err)
	}
	// GOWORK=off prevents a caller's unrelated workspace from overriding the
	// explicit test dependencies. The real Go command chooses Go >= 1.25.
	for _, args := range [][]string{{"test", "-mod=mod", "-count=1", "./..."}, {"vet", "-mod=mod", "./..."}} {
		cmd := exec.Command("go", args...)
		cmd.Dir = cfg.ServerRoot
		cmd.Env = append(os.Environ(), "GOWORK=off")
		output, err := cmd.CombinedOutput()
		t.Logf("go %s\n%s", strings.Join(args, " "), output)
		if err != nil {
			t.Fatalf("native integration failed (no shim fallback): %v\n%s", err, output)
		}
	}
}
