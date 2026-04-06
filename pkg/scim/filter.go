package scim

import (
	"fmt"
	"strings"
)

// Filter represents a parsed SCIM filter expression.
type Filter struct {
	Attribute string
	Operator  string
	Value     string
}

// ParseFilter parses a minimal SCIM filter expression.
// Supports: userName eq "value", externalId eq "value", displayName eq "value"
func ParseFilter(raw string) (*Filter, error) {
	if raw == "" {
		return nil, nil
	}

	parts := strings.SplitN(raw, " ", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid filter: expected 'attribute op value', got: %s", raw)
	}

	attr := parts[0]
	op := strings.ToLower(parts[1])
	val := parts[2]

	// Validate operator
	if op != "eq" {
		return nil, fmt.Errorf("unsupported filter operator: %s (only 'eq' is supported)", op)
	}

	// Validate attribute
	switch attr {
	case "userName", "externalId", "displayName", "emails.value":
		// OK
	default:
		return nil, fmt.Errorf("unsupported filter attribute: %s", attr)
	}

	// Strip quotes from value
	val = strings.Trim(val, "\"")
	if val == "" {
		return nil, fmt.Errorf("empty filter value")
	}

	return &Filter{
		Attribute: attr,
		Operator:  op,
		Value:     val,
	}, nil
}
