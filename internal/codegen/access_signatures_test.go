package codegen

import (
	"go/parser"
	"testing"
)

func TestAccessCanonicalTypesPreservePointerAndValueArguments(t *testing.T) {
	aliases := map[string]string{"mb": "modelbind", "wa": "webapp", "m": "models", "ctx": "context", "other": "package(example.org/other)"}
	for source, expected := range map[string]string{
		"mb.ModelInput[*m.Order]":                    "modelbind.ModelInput[*models.Order]",
		"mb.ModelInput[m.Order]":                     "modelbind.ModelInput[models.Order]",
		"wa.UpdateByKeysInput[m.OrderKey, *m.Order]": "webapp.UpdateByKeysInput[models.OrderKey,*models.Order]",
		"wa.UpdateByKeysInput[*m.OrderKey, m.Order]": "webapp.UpdateByKeysInput[*models.OrderKey,models.Order]",
		"*m.Order":    "*models.Order",
		"m.Order":     "models.Order",
		"ctx.Context": "context.Context",
		"other.Order": "package(example.org/other).Order",
		"[]m.Order":   "<unsupported>",
	} {
		expression, err := parser.ParseExpr(source)
		if err != nil {
			t.Fatal(err)
		}
		if actual := canonicalAccessType(expression, aliases); actual != expected {
			t.Errorf("%s => %s, want %s", source, actual, expected)
		}
	}
}

func TestAccessDecoderShapesRejectUnrelatedOrUnknownInputs(t *testing.T) {
	for _, tc := range []struct{ source, kind, model, key string }{
		{"models.Order", "model", "models.Order", ""},
		{"*models.Order", "model", "*models.Order", ""},
		{"modelbind.ModelInput[models.Order]", "input", "models.Order", ""},
		{"modelbind.ModelInput[*models.Order]", "input", "*models.Order", ""},
		{"webapp.UpdateByKeysInput[models.OrderKey,*models.Order]", "keys", "*models.Order", "models.OrderKey"},
		{"webapp.UpdateByKeysInput[*models.OrderKey,models.Order]", "keys", "models.Order", "*models.OrderKey"},
	} {
		shape, ok := accessShape(tc.source, "Order")
		if !ok || shape.Kind != tc.kind || shape.Model != tc.model || shape.Key != tc.key {
			t.Errorf("%s: %#v %v", tc.source, shape, ok)
		}
	}
	for _, source := range []string{"models.Other", "[]models.Order", "CustomInput", "modelbind.ModelInput[*models.Other]", "webapp.UpdateByKeysInput[models.OtherKey,*models.Order]", "*modelbind.ModelInput[*models.Order]"} {
		if _, ok := accessShape(source, "Order"); ok {
			t.Errorf("accepted unsupported signature %s", source)
		}
	}
}
