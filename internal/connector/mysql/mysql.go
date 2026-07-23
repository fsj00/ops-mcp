package mysql

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/fsj00/ops-mcp/internal/config"
	"github.com/fsj00/ops-mcp/internal/connector/dbutil"
	"github.com/fsj00/ops-mcp/internal/connector/sqlguard"
	"github.com/fsj00/ops-mcp/internal/model"
	mysqldriver "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
)

// QueryRequest is the JS-facing mysql.query payload.
type QueryRequest struct {
	Database string        `json:"database"`
	SQL      string        `json:"sql"`
	Args     []interface{} `json:"args"`
}

// VersionRequest is the JS-facing mysql.version payload.
type VersionRequest struct {
	Database string `json:"database"`
}

// Connector runs MySQL read-only operations.
type Connector struct {
	cfg *config.Manager
	log *zap.Logger
}

func New(cfg *config.Manager, log *zap.Logger) *Connector {
	if log == nil {
		log = zap.NewNop()
	}
	return &Connector{cfg: cfg, log: log}
}

// Query executes a single SELECT (enforced via sqlguard) against a named database.
func (c *Connector) Query(ctx context.Context, req QueryRequest) (*dbutil.QueryResult, error) {
	dbCfg, err := c.lookup(req.Database)
	if err != nil {
		return nil, err
	}
	safeSQL, err := sqlguard.EnsureSelect(req.SQL, dbCfg.Limit)
	if err != nil {
		return nil, model.NewAppError(model.ErrInvalidParams, err.Error())
	}

	c.log.Info("mysql query",
		zap.String("database", req.Database),
		zap.String("sql", safeSQL),
	)

	db, err := dbutil.OpenPing(ctx, "mysql", dsn(dbCfg))
	if err != nil {
		c.log.Warn("mysql dial failed",
			zap.String("database", req.Database),
			zap.String("host", dbCfg.Connection.Host),
			zap.Error(err),
		)
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("mysql dial %s: %v", req.Database, err))
	}
	defer db.Close()

	res, err := dbutil.Query(ctx, db, safeSQL, req.Args)
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("mysql query: %v", err))
	}
	return res, nil
}

// Version returns the MySQL server version string.
func (c *Connector) Version(ctx context.Context, req VersionRequest) (*dbutil.VersionResult, error) {
	dbCfg, err := c.lookup(req.Database)
	if err != nil {
		return nil, err
	}
	const sqlText = "SELECT VERSION()"
	c.log.Info("mysql version",
		zap.String("database", req.Database),
		zap.String("sql", sqlText),
	)

	db, err := dbutil.OpenPing(ctx, "mysql", dsn(dbCfg))
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("mysql dial %s: %v", req.Database, err))
	}
	defer db.Close()

	res, err := dbutil.QueryVersion(ctx, db, sqlText)
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, err.Error())
	}
	return res, nil
}

func (c *Connector) lookup(name string) (model.Database, error) {
	if name == "" {
		return model.Database{}, model.NewAppError(model.ErrInvalidParams, "mysql: database is required")
	}
	dbCfg, err := c.cfg.GetDatabase(name)
	if err != nil {
		return model.Database{}, model.NewAppError(model.ErrConnectorError, err.Error())
	}
	if dbCfg.Type != "mysql" {
		return model.Database{}, model.NewAppError(model.ErrInvalidParams,
			fmt.Sprintf("database %q type is %q, expected mysql", name, dbCfg.Type))
	}
	return dbCfg, nil
}

func dsn(db model.Database) string {
	conn := db.Connection
	port := conn.Port
	if port <= 0 {
		port = 3306
	}
	cfg := mysqldriver.NewConfig()
	cfg.User = conn.Username
	cfg.Passwd = conn.Password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(conn.Host, strconv.Itoa(port))
	cfg.DBName = conn.Database
	cfg.ParseTime = true
	cfg.Loc = nil
	return cfg.FormatDSN()
}
