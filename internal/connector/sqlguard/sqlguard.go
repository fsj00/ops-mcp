// Package sqlguard validates SQL with pingcap/parser and enforces a row limit.
package sqlguard

import (
	"fmt"
	"strings"

	"github.com/pingcap/parser"
	"github.com/pingcap/parser/ast"
	_ "github.com/pingcap/parser/test_driver" // registers ValueExpr factory for parser.New
)

// DefaultLimit matches config.DefaultQueryLimit.
const DefaultLimit = 1000

// EnsureSelect parses sql, requires a single SELECT (or UNION), and wraps it
// with an outer LIMIT so large result sets cannot overwhelm the database.
func EnsureSelect(sql string, maxLimit int) (string, error) {
	if maxLimit <= 0 {
		maxLimit = DefaultLimit
	}
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return "", fmt.Errorf("sql is required")
	}
	trimmed = strings.TrimRight(trimmed, ";")
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return "", fmt.Errorf("sql is required")
	}

	p := parser.New()
	stmts, _, err := p.Parse(trimmed, "", "")
	if err != nil {
		return "", fmt.Errorf("parse sql: %w", err)
	}
	if len(stmts) == 0 {
		return "", fmt.Errorf("empty sql")
	}
	if len(stmts) > 1 {
		return "", fmt.Errorf("only a single SQL statement is allowed")
	}

	switch stmts[0].(type) {
	case *ast.SelectStmt, *ast.SetOprStmt:
		// allowed
	default:
		return "", fmt.Errorf("only SELECT statements are allowed")
	}

	return fmt.Sprintf("SELECT * FROM (%s) AS _ops_mcp_q LIMIT %d", trimmed, maxLimit), nil
}
