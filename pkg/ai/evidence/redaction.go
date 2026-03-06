package evidence

import (
	"os"
	"regexp"
	"strings"
)

// Compiled regex patterns for secret detection.
var secretPatterns = []*regexp.Regexp{
	// Bearer tokens
	regexp.MustCompile(`(?i)(Bearer\s+)[A-Za-z0-9\-._~+/]+=*`),
	// AWS access key IDs
	regexp.MustCompile(`((?:AKIA|ASIA)[A-Z0-9]{16})`),
	// AWS secret key assignments
	regexp.MustCompile(`(?i)(aws_secret_access_key\s*[=:]\s*)[A-Za-z0-9/+=]{30,}`),
	// GitHub tokens
	regexp.MustCompile(`(ghp_[A-Za-z0-9]{36}|gho_[A-Za-z0-9]{36}|ghs_[A-Za-z0-9]{36}|ghr_[A-Za-z0-9]{36}|github_pat_[A-Za-z0-9_]{22,})`),
	// JWT tokens (three base64 segments separated by dots)
	regexp.MustCompile(`(eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,})`),
	// Stripe keys
	regexp.MustCompile(`((?:sk|pk|rk)_(?:live|test)_[A-Za-z0-9]{20,})`),
	// Generic key=value patterns for api_key, secret, token, password
	regexp.MustCompile(`(?i)((?:api_key|api_secret|secret_key|access_token|auth_token|password|credentials?)\s*[=:]\s*)[^\s"']+`),
	// Connection string assignments
	regexp.MustCompile(`(?i)((?:DATABASE_URL|REDIS_URL|MONGODB_URI|AMQP_URL|ELASTICSEARCH_URL)\s*[=:]\s*)\S+`),
}

// redactedPlaceholder is the replacement text for detected secrets.
const redactedPlaceholder = "[REDACTED]"

// sensitiveEnvPattern matches environment variable names that likely contain secrets.
var sensitiveEnvPattern = regexp.MustCompile(`(?i)(KEY|SECRET|TOKEN|PASSWORD|CREDENTIAL|AUTH)`)

// Redact applies compiled regex patterns to remove secrets from input.
func Redact(input string) string {
	result := input
	for _, pat := range secretPatterns {
		result = pat.ReplaceAllStringFunc(result, func(match string) string {
			// For patterns with capture groups that keep a prefix, preserve the prefix
			loc := pat.FindStringSubmatchIndex(match)
			if len(loc) >= 4 {
				// If there are two groups, keep the first group (prefix) and redact the rest
				prefix := match[loc[2]:loc[3]]
				if loc[3] < len(match) {
					return prefix + redactedPlaceholder
				}
				// The entire match IS the capture group (no prefix), redact it all
				return redactedPlaceholder
			}
			return redactedPlaceholder
		})
	}
	return result
}

// SensitiveEnvVarNames returns names of environment variables whose names
// match common secret patterns (KEY, SECRET, TOKEN, PASSWORD, CREDENTIAL, AUTH).
func SensitiveEnvVarNames() []string {
	var names []string
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) < 2 {
			continue
		}
		name := parts[0]
		if sensitiveEnvPattern.MatchString(name) {
			names = append(names, name)
		}
	}
	return names
}

// RedactEnvValues replaces occurrences of the values of the named env vars in input.
func RedactEnvValues(input string, envVarNames []string) string {
	result := input
	for _, name := range envVarNames {
		val := os.Getenv(name)
		if val == "" || len(val) < 4 {
			continue
		}
		result = strings.ReplaceAll(result, val, redactedPlaceholder)
	}
	return result
}

// RedactAll applies both pattern-based and env-value-based redaction.
func RedactAll(input string) string {
	result := Redact(input)
	result = RedactEnvValues(result, SensitiveEnvVarNames())
	return result
}
