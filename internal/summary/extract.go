package summary

import "encoding/json"

var preferredErrorPaths = [][]string{
	{"trace", "exception", "description"},
	{"trace", "exception", "message"},
	{"trace_chain", "0", "exception", "description"},
	{"trace_chain", "0", "exception", "message"},
	{"body", "trace_chain", "0", "exception", "description"},
	{"body", "trace_chain", "0", "exception", "message"},
	{"exception", "description"},
	{"exception", "message"},
	{"message", "body"},
	{"message"},
	{"body", "message"},
	{"body"},
}

func MainError(body json.RawMessage, data json.RawMessage) string {
	if message := fromRawJSON(data); message != "" {
		return message
	}

	if message := fromRawJSON(body); message != "" {
		return message
	}

	return "unknown"
}

func fromRawJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}

	for _, path := range preferredErrorPaths {
		if message := stringAtPath(value, path); message != "" {
			return message
		}
	}

	if message, ok := value.(string); ok {
		return message
	}

	return ""
}

func stringAtPath(value any, path []string) string {
	current := value

	for _, segment := range path {
		switch typed := current.(type) {
		case map[string]any:
			next, ok := typed[segment]
			if !ok {
				return ""
			}
			current = next
		case []any:
			indexValue := toIndex(segment)
			if indexValue < 0 || indexValue >= len(typed) {
				return ""
			}
			current = typed[indexValue]
		default:
			return ""
		}
	}

	message, ok := current.(string)
	if !ok {
		return ""
	}

	return message
}

func toIndex(value string) int {
	if len(value) == 0 {
		return -1
	}

	index := 0
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return -1
		}
		index = (index * 10) + int(ch-'0')
	}

	return index
}
