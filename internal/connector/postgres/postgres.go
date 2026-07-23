package postgres

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/fsj00/ops-mcp/internal/config"
	"github.com/fsj00/ops-mcp/internal/connector/dbutil"
	"github.com/fsj00/ops-mcp/internal/connector/sqlguard"
	"github.com/fsj00/ops-mcp/internal/model"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

// QueryRequest is the JS-facing postgres.query payload.
type QueryRequest struct {
	Database string        `json:"database"`
	SQL      string        `json:"sql"`
	Args     []interface{} `json:"args"`
}

// VersionRequest is the JS-facing postgres.version payload.
type VersionRequest struct {
	Database string `json:"database"`
}

// Connector runs PostgreSQL read-only operations.
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

	c.log.Info("postgres query",
		zap.String("database", req.Database),
		zap.String("sql", safeSQL),
	)

	db, err := dbutil.OpenPing(ctx, "postgres", dsn(dbCfg))
	if err != nil {
		c.log.Warn("postgres dial failed",
			zap.String("database", req.Database),
			zap.String("host", dbCfg.Connection.Host),
			zap.Error(err),
		)
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("postgres dial %s: %v", req.Database, err))
	}
	defer db.Close()

	res, err := dbutil.Query(ctx, db, safeSQL, req.Args)
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("postgres query: %v", err))
	}
	return res, nil
}

// Version returns the PostgreSQL server version string.
func (c *Connector) Version(ctx context.Context, req VersionRequest) (*dbutil.VersionResult, error) {
	dbCfg, err := c.lookup(req.Database)
	if err != nil {
		return nil, err
	}
	const sqlText = "SELECT version()"
	c.log.Info("postgres version",
		zap.String("database", req.Database),
		zap.String("sql", sqlText),
	)

	db, err := dbutil.OpenPing(ctx, "postgres", dsn(dbCfg))
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("postgres dial %s: %v", req.Database, err))
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
		return model.Database{}, model.NewAppError(model.ErrInvalidParams, "postgres: database is required")
	}
	dbCfg, err := c.cfg.GetDatabase(name)
	if err != nil {
		return model.Database{}, model.NewAppError(model.ErrConnectorError, err.Error())
	}
	if dbCfg.Type != "postgresql" {
		return model.Database{}, model.NewAppError(model.ErrInvalidParams,
			fmt.Sprintf("database %q type is %q, expected postgresql", name, dbCfg.Type))
	}
	return dbCfg, nil
}

func dsn(db model.Database) string {
	conn := db.Connection
	sslmode := conn.SSLMode
	if sslmode == "" {
		sslmode = "disable"
	}
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(conn.Username, conn.Password),
		Host:   netHost(conn.Host, conn.Port),
		Path:   "/" + conn.Database,
	}
	q := url.Values{}
	q.Set("sslmode", sslmode)
	u.RawQuery = q.Encode()
	return u.String()
}

func netHost(host string, port int) string {
	if port <= 0 {
		port = 5432
	}
	return host + ":" + strconv.Itoa(port)
}
