package main

import (
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/fsj00/ops-mcp/internal/api"
	"github.com/fsj00/ops-mcp/internal/config"
	cmdconn "github.com/fsj00/ops-mcp/internal/connector/command"
	dockerconn "github.com/fsj00/ops-mcp/internal/connector/docker"
	httpconn "github.com/fsj00/ops-mcp/internal/connector/http"
	kafkaconn "github.com/fsj00/ops-mcp/internal/connector/kafka"
	mysqlconn "github.com/fsj00/ops-mcp/internal/connector/mysql"
	postgresconn "github.com/fsj00/ops-mcp/internal/connector/postgres"
	redisconn "github.com/fsj00/ops-mcp/internal/connector/redis"
	snmpconn "github.com/fsj00/ops-mcp/internal/connector/snmp"
	sshconn "github.com/fsj00/ops-mcp/internal/connector/ssh"
	tcpconn "github.com/fsj00/ops-mcp/internal/connector/tcp"
	udpconn "github.com/fsj00/ops-mcp/internal/connector/udp"
	"github.com/fsj00/ops-mcp/internal/executor"
	"github.com/fsj00/ops-mcp/internal/mcp"
	"github.com/fsj00/ops-mcp/internal/openapi"
	"github.com/fsj00/ops-mcp/internal/plugin"
	"github.com/fsj00/ops-mcp/internal/runtime"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	var configPath string

	root := &cobra.Command{
		Use:   "ops-mcp",
		Short: "Remote MCP Ops Server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(configPath)
		},
	}
	root.Flags().StringVar(&configPath, "config", "./config/ops-mcp.yaml", "path to ops-mcp.yaml")

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	app := cfg.App()

	log, err := newLogger(app.Log.Level, app.Log.Encoding)
	if err != nil {
		return err
	}
	defer log.Sync() //nolint:errcheck
	cfg.SetLogger(log)

	pm := plugin.NewManager(app.Plugins.Dir, log)
	sshC := sshconn.New(cfg, log)
	dockerC := dockerconn.New(sshC, log)
	mysqlC := mysqlconn.New(cfg, log)
	postgresC := postgresconn.New(cfg, log)
	redisC := redisconn.New(cfg, log)
	kafkaC := kafkaconn.New(cfg, log)
	httpC := httpconn.New(cfg, log)
	commandC := cmdconn.New(cfg, log)
	snmpC := snmpconn.New(cfg, log)
	tcpC := tcpconn.New(log)
	udpC := udpconn.New(log)
	apiReg := openapi.NewRegistry(cfg, httpC, log)
	apiReg.SetReservedNamesProvider(pm.Names)
	pm.SetReservedNamesProvider(apiReg.Names)
	if _, err := pm.Load(); err != nil {
		return fmt.Errorf("load plugins: %w", err)
	}
	if _, err := apiReg.Load(); err != nil {
		return fmt.Errorf("load openapi tools: %w", err)
	}
	rt := runtime.New(runtime.Dependencies{
		SSH:      sshC,
		Docker:   dockerC,
		MySQL:    mysqlC,
		Postgres: postgresC,
		Redis:    redisC,
		Kafka:    kafkaC,
		HTTP:     httpC,
		Command:  commandC,
		SNMP:     snmpC,
		TCP:      tcpC,
		UDP:      udpC,
		APIs:     apiReg,
		Cfg:      cfg,
		Log:      log,
	})
	exec := executor.New(rt, cfg.DefaultPluginTimeout())
	mcpServer := mcp.NewServer(pm, exec, log)
	mcpServer.SetAPITools(apiReg)
	httpServer := api.New(cfg, pm, mcpServer, log)

	addr := net.JoinHostPort(app.Server.Host, strconv.Itoa(app.Server.Port))
	authEnabled := app.Server.Auth.Token != ""
	log.Info("ops-mcp starting",
		zap.String("addr", addr),
		zap.String("plugins", app.Plugins.Dir),
		zap.Int("plugin_count", pm.Count()),
		zap.Int("openapi_tool_count", apiReg.Count()),
		zap.Int("tools_count", mcpServer.ToolsCount()),
		zap.Bool("auth_enabled", authEnabled),
	)
	return httpServer.Router().Run(addr)
}

func newLogger(level, encoding string) (*zap.Logger, error) {
	var lvl zapcore.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = zapcore.InfoLevel
	}
	cfg := zap.Config{
		Level:            zap.NewAtomicLevelAt(lvl),
		Development:      false,
		Encoding:         encoding,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
	}
	if encoding == "console" {
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	return cfg.Build()
}
