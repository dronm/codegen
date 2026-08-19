package codegen

import (
	"strings"
	"unicode"
)

var goInitialisms = map[string]string{
	"ai":    "AI",
	"api":   "API",
	"http":  "HTTP",
	"https": "HTTPS",
	"id":    "ID",
	"ip":    "IP",
	"json":  "JSON",
	"sql":   "SQL",
	"tcp":   "TCP",
	"udp":   "UDP",
	"uri":   "URI",
	"url":   "URL",
	"uuid":  "UUID",
	"xml":   "XML",
}

func PascalCase(value string) string {
	words := splitWords(value)
	for i, word := range words {
		words[i] = pascalWord(word)
	}

	return strings.Join(words, "")
}

func CamelCase(value string) string {
	words := splitWords(value)
	if len(words) == 0 {
		return ""
	}
	words[0] = strings.ToLower(words[0])
	for i := 1; i < len(words); i++ {
		words[i] = pascalWord(words[i])
	}

	return strings.Join(words, "")
}

func pascalWord(word string) string {
	if word == "" {
		return ""
	}
	if initialism, exists := goInitialisms[strings.ToLower(word)]; exists {
		return initialism
	}
	runes := []rune(word)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func SnakeCase(value string) string {
	return strings.Join(splitWords(value), "_")
}

func KebabCase(value string) string {
	return strings.Join(splitWords(value), "-")
}

func HumanName(value string) string {
	return strings.Join(splitWords(value), " ")
}

func splitWords(value string) []string {
	if value == "" {
		return nil
	}

	value = strings.ReplaceAll(value, "-", "_")
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '_' || r == ' ' || r == '.' || r == '/'
	})

	words := make([]string, 0)
	for _, part := range parts {
		if part == "" {
			continue
		}
		words = append(words, splitCamelPart(part)...)
	}

	return words
}

func splitCamelPart(part string) []string {
	runes := []rune(part)
	if len(runes) == 0 {
		return nil
	}

	words := make([]string, 0)
	start := 0
	for i := 1; i < len(runes); i++ {
		prev := runes[i-1]
		cur := runes[i]
		var next rune
		if i+1 < len(runes) {
			next = runes[i+1]
		}

		boundary := unicode.IsLower(prev) && unicode.IsUpper(cur)
		boundary = boundary || (unicode.IsUpper(prev) && unicode.IsUpper(cur) && next != 0 && unicode.IsLower(next))
		boundary = boundary || (unicode.IsLetter(prev) && unicode.IsDigit(cur))
		boundary = boundary || (unicode.IsDigit(prev) && unicode.IsLetter(cur))

		if boundary {
			words = append(words, strings.ToLower(string(runes[start:i])))
			start = i
		}
	}

	words = append(words, strings.ToLower(string(runes[start:])))
	return words
}

func PluralSnake(value string) string {
	snake := SnakeCase(value)
	if strings.HasSuffix(snake, "s") {
		return snake
	}
	if strings.HasSuffix(snake, "y") {
		return strings.TrimSuffix(snake, "y") + "ies"
	}

	return snake + "s"
}
