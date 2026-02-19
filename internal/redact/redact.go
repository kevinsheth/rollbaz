package redact

import (
	"regexp"
	"strings"
)

var accessTokenQueryPattern = regexp.MustCompile(`([?&]access_token=)[^&\s]+`)

func String(value string, token string) string {
	if value == "" {
		return value
	}

	redacted := accessTokenQueryPattern.ReplaceAllString(value, `${1}[REDACTED]`)
	if token == "" {
		return redacted
	}

	return strings.ReplaceAll(redacted, token, "[REDACTED]")
}
