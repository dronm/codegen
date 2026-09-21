package codegen

import (
	"bytes"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"text/template"
)

// These syntax tests deliberately use metadata maps, not substitute declarations
// for modelbind/webapp. They can run with the standard library alone. Native
// generated-code compilation is covered by TestAccessGeneratedApplicationIntegration.
func TestAccessTemplateSyntaxMatrix(t *testing.T) {
	for _, composite := range []bool{false, true} {
		for _, keyValue := range []bool{false, true} {
			for _, modelValue := range []bool{false, true} {
				t.Run(fmt.Sprintf("composite=%t/keyValue=%t/modelValue=%t", composite, keyValue, modelValue), func(t *testing.T) {
					keys := []map[string]any{{"Name": "id", "JSONName": "id", "PathName": "orderID", "Pascal": "ID", "IsNumeric": true}}
					if composite {
						keys = append(keys, map[string]any{"Name": "code", "JSONName": "code", "PathName": "itemCode", "Pascal": "Code", "IsNumeric": false})
					}
					keyType, modelType := "*models.OrderKey", "*models.Order"
					if keyValue {
						keyType = "models.OrderKey"
					}
					if modelValue {
						modelType = "models.Order"
					}
					access := map[string]any{
						"Enabled": true, "HTTPModelbind": true, "CreateValue": modelValue,
						"CreateKind": "input", "CreateModelType": modelType,
						"UpdateKeyType": keyType, "UpdateModelType": modelType,
						"UpdateKeyValue": keyValue, "UpdateModelValue": modelValue,
						"AdapterModelbind": true,
						"DecoderAdapters": []map[string]any{
							{"Name": "orderUpdateAccessJSONDecoder", "Kind": "keys", "Key": keyType, "Model": modelType, "KeyValue": keyValue, "ModelValue": modelValue},
							{"Name": "orderCreateAccessJSONDecoder", "Kind": "input", "Model": "models.Order", "ModelValue": true, "ForInsert": true},
							{"Name": "orderWholeAccessJSONDecoder", "Kind": "model", "Model": "models.Order", "ModelValue": true, "ForInsert": false},
						},
						"Models": []map[string]any{{"Name": "Order", "Resource": "Order"}, {"Name": "OrderKey", "Resource": "Order"}},
					}
					data := map[string]any{"GoModule": "example.com/project", "Name": "Order", "Camel": "order", "Keys": keys, "Access": access, "GeneratedHTTPCRUD": map[string]any{"Update": true}}
					for _, name := range []string{"access_http.go.tmpl", "access_service.go.tmpl", "access_model.go.tmpl"} {
						text := renderAccessSyntaxTemplate(t, name, data)
						if name == "access_http.go.tmpl" && strings.Contains(text, "input.Validate(") {
							t.Fatal("body was validated outside preparation")
						}
					}
				})
			}
		}
	}
	// Create-only and full-model adapters exercise conditional imports/branches.
	for _, kind := range []string{"input", "model"} {
		data := map[string]any{"GoModule": "example.com/project", "Name": "Order", "Camel": "order", "GeneratedHTTPCRUD": map[string]any{}, "Access": map[string]any{
			"CreateValue": true, "CreateKind": kind, "CreateModelType": "models.Order", "HTTPModelbind": kind == "input",
			"AdapterModelbind": kind == "input", "DecoderAdapters": []map[string]any{{"Name": "orderCreateAccessJSONDecoder", "Kind": kind, "Model": "models.Order", "ModelValue": true, "ForInsert": true}},
		}}
		for _, name := range []string{"access_http.go.tmpl", "access_service.go.tmpl"} {
			renderAccessSyntaxTemplate(t, name, data)
		}
	}
	for _, writes := range []bool{false, true} {
		fields := []map[string]any{{"Field": "owner", "Column": "customer_id"}}
		model := map[string]any{"Name": "Order", "Resource": "Order", "Rows": fields}
		if writes {
			model["Writes"] = fields
		}
		renderAccessSyntaxTemplate(t, "access_models.go.tmpl", map[string]any{"GoModule": "example.com/project", "Models": []map[string]any{model}})
	}
}

func renderAccessSyntaxTemplate(t *testing.T, name string, data any) string {
	t.Helper()
	path := filepath.Join("templates", "go", name)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tmpl, err := template.New(name).Funcs(template.FuncMap{"goQuote": strconv.Quote}).Parse(string(content))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatal(err)
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		t.Fatalf("%s: %v\n%s", name, err, buf.Bytes())
	}
	if _, err := parser.ParseFile(token.NewFileSet(), name, formatted, parser.AllErrors); err != nil {
		t.Fatal(err)
	}
	return string(formatted)
}
