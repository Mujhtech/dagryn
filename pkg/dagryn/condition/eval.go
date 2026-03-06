package condition

import (
	"fmt"
	"strconv"
	"strings"
)

// Context provides the variables available for condition evaluation.
type Context struct {
	Branch      string // git branch name
	Event       string // "push", "pull_request", "cli", "api", "dashboard"
	EventAction string // "opened", "synchronize", etc.
	PRNumber    int    // 0 if not a PR
	Trigger     string // "ci", "cli", "api", "dashboard"
}

// Evaluate evaluates a condition expression against the given context.
// An empty expression always returns true.
// Supported syntax:
//   - Equality: branch == 'main'
//   - Inequality: branch != 'main'
//   - AND: event == 'push' && branch == 'main'
//   - OR: event == 'push' || event == 'pull_request'
//   - Variables: branch, event, event_action, pr_number, trigger
func Evaluate(expr string, ctx *Context) (bool, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return true, nil
	}

	// Split on || (OR groups) - any group matching means true
	orGroups := splitOnOperator(expr, "||")

	for _, group := range orGroups {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}

		// Split on && (AND groups) - all must match
		andParts := splitOnOperator(group, "&&")

		allMatch := true
		for _, part := range andParts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}

			match, err := evaluateComparison(part, ctx)
			if err != nil {
				return false, err
			}
			if !match {
				allMatch = false
				break
			}
		}

		if allMatch {
			return true, nil
		}
	}

	return false, nil
}

// splitOnOperator splits an expression on the given operator, respecting quoted strings.
func splitOnOperator(expr, op string) []string {
	var parts []string
	inQuote := false
	quoteChar := byte(0)
	start := 0

	for i := 0; i < len(expr); i++ {
		if !inQuote && (expr[i] == '\'' || expr[i] == '"') {
			inQuote = true
			quoteChar = expr[i]
			continue
		}
		if inQuote && expr[i] == quoteChar {
			inQuote = false
			continue
		}
		if !inQuote && i+len(op) <= len(expr) && expr[i:i+len(op)] == op {
			parts = append(parts, expr[start:i])
			start = i + len(op)
			i += len(op) - 1
		}
	}

	parts = append(parts, expr[start:])
	return parts
}

// evaluateComparison evaluates a single comparison like "branch == 'main'" or "branch != 'dev'".
func evaluateComparison(expr string, ctx *Context) (bool, error) {
	// Try != first (before ==) to avoid misparse
	if lhs, rhs, ok := splitComparison(expr, "!="); ok {
		lhsVal, err := resolveValue(lhs, ctx)
		if err != nil {
			return false, err
		}
		rhsVal, err := resolveValue(rhs, ctx)
		if err != nil {
			return false, err
		}
		return lhsVal != rhsVal, nil
	}

	if lhs, rhs, ok := splitComparison(expr, "=="); ok {
		lhsVal, err := resolveValue(lhs, ctx)
		if err != nil {
			return false, err
		}
		rhsVal, err := resolveValue(rhs, ctx)
		if err != nil {
			return false, err
		}
		return lhsVal == rhsVal, nil
	}

	return false, fmt.Errorf("invalid condition expression: %q (expected 'var == value' or 'var != value')", expr)
}

// splitComparison splits an expression on the given operator.
func splitComparison(expr, op string) (lhs, rhs string, ok bool) {
	idx := strings.Index(expr, op)
	if idx < 0 {
		return "", "", false
	}
	return strings.TrimSpace(expr[:idx]), strings.TrimSpace(expr[idx+len(op):]), true
}

// resolveValue resolves a token to its string value.
// Quoted strings ('value' or "value") are returned as-is.
// Unquoted tokens are treated as variable names.
func resolveValue(token string, ctx *Context) (string, error) {
	token = strings.TrimSpace(token)

	// Check for quoted string
	if (strings.HasPrefix(token, "'") && strings.HasSuffix(token, "'")) ||
		(strings.HasPrefix(token, "\"") && strings.HasSuffix(token, "\"")) {
		return token[1 : len(token)-1], nil
	}

	// Resolve as variable
	switch token {
	case "branch":
		return ctx.Branch, nil
	case "event":
		return ctx.Event, nil
	case "event_action":
		return ctx.EventAction, nil
	case "pr_number":
		return strconv.Itoa(ctx.PRNumber), nil
	case "trigger":
		return ctx.Trigger, nil
	default:
		return "", fmt.Errorf("unknown variable %q in condition", token)
	}
}
