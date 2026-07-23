package command

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/fsj00/ops-mcp/internal/config"
	"github.com/fsj00/ops-mcp/internal/model"
	"go.uber.org/zap"
)

// DefaultMaxOutputBytes caps stdout/stderr to avoid memory blowups.
const DefaultMaxOutputBytes = 1 << 20 // 1 MiB

// ExecRequest is the JS-facing local command exec payload.
type ExecRequest struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Workdir string            `json:"workdir"`
	Env     map[string]string `json:"env"`
}

// ExecResult is the JS-facing local command exec result.
type ExecResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// Connector executes whitelisted local processes (no shell).
type Connector struct {
	cfg            *config.Manager
	log            *zap.Logger
	maxOutputBytes int
}

func New(cfg *config.Manager, log *zap.Logger) *Connector {
	if log == nil {
		log = zap.NewNop()
	}
	return &Connector{
		cfg:            cfg,
		log:            log,
		maxOutputBytes: DefaultMaxOutputBytes,
	}
}

// Exec runs a whitelisted command by logical name.
func (c *Connector) Exec(ctx context.Context, req ExecRequest) (*ExecResult, error) {
	if req.Command == "" {
		return nil, model.NewAppError(model.ErrInvalidParams, "command.exec: command is required")
	}
	if c.cfg == nil {
		return nil, model.NewAppError(model.ErrConnectorError, "command.exec: config manager not configured")
	}

	entry, err := c.cfg.GetCommand(req.Command)
	if err != nil {
		return nil, model.NewAppError(model.ErrInvalidParams, err.Error())
	}
	if entry.Path == "" {
		return nil, model.NewAppError(model.ErrConnectorError,
			fmt.Sprintf("command.exec %s: no available path on this host", req.Command))
	}

	cmd := exec.CommandContext(ctx, entry.Path, req.Args...)
	if req.Workdir != "" {
		cmd.Dir = req.Workdir
	}
	if len(req.Env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range req.Env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}
	// New process group so cancel can kill children when possible.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	c.log.Debug("command exec",
		zap.String("command", req.Command),
		zap.String("path", entry.Path),
		zap.Int("args_len", len(req.Args)),
		zap.Int("env_len", len(req.Env)),
	)

	err = cmd.Run()
	result := &ExecResult{
		Stdout:   truncateOutput(stdout.String(), c.maxOutputBytes),
		Stderr:   truncateOutput(stderr.String(), c.maxOutputBytes),
		ExitCode: 0,
	}

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || ctx.Err() != nil {
			killProcessGroup(cmd)
			return nil, model.NewAppError(model.ErrPluginTimeout, ctx.Err().Error())
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			result.ExitCode = ee.ExitCode()
			return result, nil
		}
		c.log.Warn("command exec failed",
			zap.String("command", req.Command),
			zap.String("path", entry.Path),
			zap.Error(err),
		)
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("command.exec %s: %v", req.Command, err))
	}
	return result, nil
}

func truncateOutput(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "\n...[truncated]"
}

func killProcessGroup(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil || pgid <= 0 {
		_ = cmd.Process.Kill()
		return
	}
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
}
