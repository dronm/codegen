package codegen

import (
	"go/ast"
	"strings"
)

type accessInputShape struct {
	Kind  string
	Model string
	Key   string
}

func protectedAccessInput(input, model string) bool {
	_, ok := accessShape(input, model)
	return ok || strings.Contains(input, "modelbind.ModelInput[") || strings.Contains(input, "webapp.UpdateByKeysInput[")
}

func accessShape(input, model string) (accessInputShape, bool) {
	isModel := func(value string) bool { return value == "models."+model || value == "*models."+model }
	if isModel(input) {
		return accessInputShape{Kind: "model", Model: input}, true
	}
	if strings.HasPrefix(input, "modelbind.ModelInput[") && strings.HasSuffix(input, "]") {
		inner := strings.TrimSuffix(strings.TrimPrefix(input, "modelbind.ModelInput["), "]")
		if isModel(inner) {
			return accessInputShape{Kind: "input", Model: inner}, true
		}
	}
	if strings.HasPrefix(input, "webapp.UpdateByKeysInput[") && strings.HasSuffix(input, "]") {
		inner := strings.TrimSuffix(strings.TrimPrefix(input, "webapp.UpdateByKeysInput["), "]")
		parts := strings.Split(inner, ",")
		if len(parts) == 2 && (parts[0] == "models."+model+"Key" || parts[0] == "*models."+model+"Key") && isModel(parts[1]) {
			return accessInputShape{Kind: "keys", Key: parts[0], Model: parts[1]}, true
		}
	}
	return accessInputShape{}, false
}

func canonicalAccessType(expr ast.Expr, aliases map[string]string) string {
	switch node := expr.(type) {
	case *ast.Ident:
		return node.Name
	case *ast.StarExpr:
		return "*" + canonicalAccessType(node.X, aliases)
	case *ast.ParenExpr:
		return canonicalAccessType(node.X, aliases)
	case *ast.SelectorExpr:
		if pkg, ok := node.X.(*ast.Ident); ok {
			if name, exists := aliases[pkg.Name]; exists {
				return name + "." + node.Sel.Name
			}
		}
	case *ast.IndexExpr:
		return canonicalAccessType(node.X, aliases) + "[" + canonicalAccessType(node.Index, aliases) + "]"
	case *ast.IndexListExpr:
		parts := make([]string, 0, len(node.Indices))
		for _, item := range node.Indices {
			parts = append(parts, canonicalAccessType(item, aliases))
		}
		return canonicalAccessType(node.X, aliases) + "[" + strings.Join(parts, ",") + "]"
	}
	return "<unsupported>"
}
