package kafka

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fsj00/ops-mcp/internal/model"
)

func paramString(params map[string]interface{}, key string) string {
	if params == nil {
		return ""
	}
	v, ok := params[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case fmt.Stringer:
		return strings.TrimSpace(t.String())
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func paramInt(params map[string]interface{}, key string) (int, bool, error) {
	if params == nil {
		return 0, false, nil
	}
	v, ok := params[key]
	if !ok || v == nil {
		return 0, false, nil
	}
	switch t := v.(type) {
	case int:
		return t, true, nil
	case int32:
		return int(t), true, nil
	case int64:
		return int(t), true, nil
	case float64:
		return int(t), true, nil
	case float32:
		return int(t), true, nil
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return 0, false, nil
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			return 0, false, model.NewAppError(model.ErrInvalidParams, fmt.Sprintf("kafka: %s must be an integer", key))
		}
		return n, true, nil
	default:
		return 0, false, model.NewAppError(model.ErrInvalidParams, fmt.Sprintf("kafka: %s must be an integer", key))
	}
}

func paramBool(params map[string]interface{}, key string) bool {
	if params == nil {
		return false
	}
	v, ok := params[key]
	if !ok || v == nil {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(strings.TrimSpace(t), "true")
	default:
		return false
	}
}

func requireString(params map[string]interface{}, key string) (string, error) {
	s := paramString(params, key)
	if s == "" {
		return "", model.NewAppError(model.ErrInvalidParams, fmt.Sprintf("kafka: %s is required", key))
	}
	return s, nil
}

func applyLimit(n, cfgLimit, reqLimit int) (limit int, truncated bool) {
	limit = cfgLimit
	if limit <= 0 {
		limit = 1000
	}
	if reqLimit > 0 && reqLimit < limit {
		limit = reqLimit
	}
	if n > limit {
		return limit, true
	}
	return n, false
}
