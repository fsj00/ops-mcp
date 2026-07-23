package redis

import (
	"fmt"

	"github.com/fsj00/ops-mcp/internal/config"
)

// ClampLimit requires a positive requested limit and caps it at maxLimit
// (default config.DefaultQueryLimit when maxLimit <= 0).
func ClampLimit(requested, maxLimit int) (int, error) {
	if requested <= 0 {
		return 0, fmt.Errorf("limit is required and must be > 0 (e.g. 1000)")
	}
	if maxLimit <= 0 {
		maxLimit = config.DefaultQueryLimit
	}
	if requested > maxLimit {
		return maxLimit, nil
	}
	return requested, nil
}
