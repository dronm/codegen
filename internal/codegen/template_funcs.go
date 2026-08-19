package codegen

import (
	"strconv"
	"strings"
)

func TemplateFuncs() map[string]any {
	return map[string]any{
		"joinNames":                  joinNames,
		"joinKeyNames":               joinKeyNames,
		"joinFrontendNames":          joinFrontendNames,
		"joinFrontendKeyNames":       joinFrontendKeyNames,
		"frontendKeyValueType":       frontendKeyValueType,
		"frontendKeyValueExpression": frontendKeyValueExpression,
		"frontendValibot":            frontendValibot,
		"frontendSubmitExpression":   frontendSubmitExpression,
		"frontendGridColumnsClass":   frontendGridColumnsClass,
		"frontendColumnSpanClass":    frontendColumnSpanClass,
		"firstFrontendKeyName":       firstFrontendKeyName,
		"contains":                   strings.Contains,
		"sub":                        func(a int, b int) int { return a - b },
		"add":                        func(a int, b int) int { return a + b },
		"sqlQuote":                   sqlQuote,
		"sqlNullableText":            sqlNullableText,
		"goQuote":                    strconv.Quote,
	}
}

func frontendGridColumnsClass(columns int) string {
	switch columns {
	case 1:
		return "grid grid-cols-1 gap-4"
	case 3:
		return "grid grid-cols-1 gap-4 md:grid-cols-3"
	case 4:
		return "grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4"
	default:
		return "grid grid-cols-1 gap-4 md:grid-cols-2"
	}
}

func frontendColumnSpanClass(span int) string {
	switch span {
	case 2:
		return " md:col-span-2"
	case 3:
		return " md:col-span-3"
	case 4:
		return " md:col-span-4"
	default:
		return ""
	}
}

func firstFrontendKeyName(keys []KeyView) string {
	if len(keys) == 0 {
		return "id"
	}
	return keys[0].TSName
}

func joinFrontendNames(fields []FieldView, quote bool) string {
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		name := field.TSName
		if quote {
			name = strconv.Quote(name)
		}
		parts = append(parts, name)
	}
	return strings.Join(parts, " | ")
}

func joinFrontendKeyNames(keys []KeyView, quote bool) string {
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		name := key.TSName
		if quote {
			name = strconv.Quote(name)
		}
		parts = append(parts, name)
	}
	return strings.Join(parts, " | ")
}

func frontendKeyValueType(keys []KeyView) string {
	if len(keys) == 1 {
		return keys[0].TSType
	}
	return "string"
}

func frontendKeyValueExpression(keys []KeyView) string {
	if len(keys) == 1 {
		return "key." + keys[0].TSName
	}
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, "${key."+key.TSName+"}")
	}
	return "`" + strings.Join(parts, "/") + "`"
}

func frontendValibot(expression string, optional bool) string {
	if optional {
		return "v.optional(" + expression + ")"
	}
	return expression
}

func joinNames(fields []FieldView, quote bool) string {
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		name := field.Name
		if quote {
			name = "\"" + name + "\""
		}
		parts = append(parts, name)
	}

	return strings.Join(parts, " | ")
}

func joinKeyNames(keys []KeyView, quote bool) string {
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		name := key.Name
		if quote {
			name = "\"" + name + "\""
		}
		parts = append(parts, name)
	}

	return strings.Join(parts, " | ")
}

func sqlQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func sqlNullableText(value *string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return "NULL"
	}
	return sqlQuote(strings.TrimSpace(*value))
}
