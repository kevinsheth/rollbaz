package redact

import (
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
)

var accessTokenQueryPattern = regexp.MustCompile(`([?&]access_token=)[^&\s]+`)

var sensitiveKeyWords = []string{"token", "authorization", "secret", "password", "api_key", "apikey"}

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

func Value(value any, token string) any {
	switch typed := value.(type) {
	case map[string]any:
		return redactMap(typed, token)
	case []any:
		return redactSlice(typed, token)
	case string:
		return String(typed, token)
	default:
		return redactStructured(value, token)
	}
}

func redactMap(value map[string]any, token string) map[string]any {
	clean := make(map[string]any, len(value))
	for key, nested := range value {
		if isSensitiveKey(key) {
			clean[key] = "[REDACTED]"
			continue
		}
		clean[key] = Value(nested, token)
	}

	return clean
}

func redactSlice(value []any, token string) []any {
	clean := make([]any, len(value))
	for index := range value {
		clean[index] = Value(value[index], token)
	}

	return clean
}

func redactStructured(value any, token string) any {
	if value == nil {
		return nil
	}

	kind := reflect.TypeOf(value).Kind()
	if kind != reflect.Struct && kind != reflect.Pointer {
		return value
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var decoded any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return value
	}

	return Value(decoded, token)
}

func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, keyword := range sensitiveKeyWords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}

	return false
}
