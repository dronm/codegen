package codegen

import (
	"fmt"
	"regexp"
	"strings"
)

var accessIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func accessIdentifier(value string) bool {
	return accessIdentifierPattern.MatchString(value)
}

// Only identifiers are accepted; this is not an SQL expression parser. The
// caller supplies the qualifiers exposed by generator-owned relation metadata.
func validateAccessColumnReference(column string, qualifiers ...string) error {
	parts := strings.Split(column, ".")
	if len(parts) < 1 || len(parts) > 3 {
		return fmt.Errorf("row target %q must be a column or qualified column, not SQL", column)
	}
	for _, part := range parts {
		if !accessIdentifier(part) {
			return fmt.Errorf("row target %q must contain only identifiers; SQL expressions are not supported", column)
		}
	}
	if len(parts) == 1 {
		return nil
	}
	qualifier := strings.Join(parts[:len(parts)-1], ".")
	for _, known := range qualifiers {
		if qualifier == known {
			return nil
		}
	}
	return fmt.Errorf("row target %q uses qualifier %q not exposed by this relation (known: %s)", column, qualifier, strings.Join(qualifiers, ", "))
}

// canonicalAccessWriteColumn returns the spelling used for modelbind field-ID
// lookup. Policy targets are unquoted SQL identifiers, not SQL expressions.
func canonicalAccessWriteColumn(column string) (string, error) {
	if !accessIdentifier(column) {
		return "", fmt.Errorf("%q must be an unqualified column", column)
	}
	return strings.ToLower(column), nil
}
