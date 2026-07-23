package ssh

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/fsj00/ops-mcp/internal/certutil"
	"github.com/fsj00/ops-mcp/internal/config"
	"github.com/fsj00/ops-mcp/internal/model"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
)

// ExecRequest is the JS-facing SSH exec payload.
type ExecRequest struct {
	Host    string            `json:"host"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Workdir string            `json:"workdir"`
	Env     map[string]string `json:"env"`
}

// ExecResult is the JS-facing SSH exec result.
type ExecResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// Connector executes remote commands over SSH.
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

// Exec runs a command on the named host.
func (c *Connector) Exec(ctx context.Context, req ExecRequest) (*ExecResult, error) {
	if req.Host == "" {
		return nil, model.NewAppError(model.ErrInvalidParams, "ssh.exec: host is required")
	}
	if req.Command == "" {
		return nil, model.NewAppError(model.ErrInvalidParams, "ssh.exec: command is required")
	}

	host, err := c.cfg.GetHost(req.Host)
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, err.Error())
	}

	client, err := dialSSH(ctx, host)
	if err != nil {
		c.log.Warn("ssh dial failed",
			zap.String("host", req.Host),
			zap.String("address", host.Address.Host),
			zap.Error(err),
		)
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("ssh dial %s: %v", req.Host, err))
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("ssh session: %v", err))
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	cmd := buildCommand(req)
	c.log.Debug("ssh exec",
		zap.String("host", req.Host),
		zap.String("command", req.Command),
		zap.Int("args_len", len(req.Args)),
	)

	done := make(chan error, 1)
	go func() {
		done <- session.Run(cmd)
	}()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGKILL)
		return nil, model.NewAppError(model.ErrPluginTimeout, ctx.Err().Error())
	case err := <-done:
		result := &ExecResult{
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			ExitCode: 0,
		}
		if err != nil {
			if ee, ok := err.(*ssh.ExitError); ok {
				result.ExitCode = ee.ExitStatus()
				return result, nil
			}
			return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("ssh run: %v", err))
		}
		return result, nil
	}
}

func dialSSH(ctx context.Context, host model.Host) (*ssh.Client, error) {
	auth, err := buildAuth(host)
	if err != nil {
		return nil, err
	}
	cfg := &ssh.ClientConfig{
		User:            host.Auth.Username,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // MVP: skip host key verify
		Timeout:         15 * time.Second,
	}
	addr := net.JoinHostPort(host.Address.Host, strconv.Itoa(host.Address.Port))

	d := net.Dialer{Timeout: 15 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return ssh.NewClient(c, chans, reqs), nil
}

func buildAuth(host model.Host) ([]ssh.AuthMethod, error) {
	switch host.Auth.Type {
	case "password":
		if host.Auth.Password == "" {
			return nil, fmt.Errorf("password auth requires password")
		}
		return []ssh.AuthMethod{ssh.Password(host.Auth.Password)}, nil
	case "private_key":
		pem, err := certutil.ResolveMaterial(host.Auth.PrivateKey, host.Auth.PrivateKeyFile, "private_key auth")
		if err != nil {
			return nil, err
		}
		if len(pem) == 0 {
			return nil, fmt.Errorf("private_key auth requires private_key or private_key_file")
		}
		signer, err := ssh.ParsePrivateKey(pem)
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
	default:
		return nil, fmt.Errorf("unsupported auth type %q", host.Auth.Type)
	}
}

func buildCommand(req ExecRequest) string {
	parts := make([]string, 0, 1+len(req.Args))
	parts = append(parts, shellQuote(req.Command))
	for _, a := range req.Args {
		parts = append(parts, shellQuote(a))
	}
	cmd := strings.Join(parts, " ")
	if req.Workdir != "" {
		cmd = "cd " + shellQuote(req.Workdir) + " && " + cmd
	}
	if len(req.Env) > 0 {
		exports := make([]string, 0, len(req.Env))
		for k, v := range req.Env {
			exports = append(exports, k+"="+shellQuote(v))
		}
		cmd = strings.Join(exports, " ") + " " + cmd
	}
	return cmd
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
