package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fsj00/ops-mcp/internal/connector/ssh"
	"github.com/fsj00/ops-mcp/internal/model"
	"go.uber.org/zap"
)

// PSRequest lists containers.
type PSRequest struct {
	All  bool   `json:"all"`
	Host string `json:"host"`
}

// ContainerInfo is a container summary.
type ContainerInfo struct {
	ID      string   `json:"id"`
	Names   []string `json:"names"`
	Image   string   `json:"image"`
	Status  string   `json:"status"`
	State   string   `json:"state"`
	Ports   string   `json:"ports"`
	Created string   `json:"created,omitempty"`
}

// LogsRequest reads container logs.
type LogsRequest struct {
	Container  string `json:"container"`
	Tail       string `json:"tail"`
	Since      string `json:"since"`
	Timestamps bool   `json:"timestamps"`
	Host       string `json:"host"`
}

// LogsResult holds log text.
type LogsResult struct {
	Logs string `json:"logs"`
}

// InfoRequest is docker info.
type InfoRequest struct {
	Host string `json:"host"`
}

// InfoResult holds docker info output.
type InfoResult struct {
	Raw    string                 `json:"raw,omitempty"`
	Info   map[string]interface{} `json:"info,omitempty"`
	Format string                 `json:"format,omitempty"` // "json" | "text"
}

// StatsRequest is docker stats --no-stream.
type StatsRequest struct {
	Host      string `json:"host"`
	Container string `json:"container"` // optional; empty = all running
}

// StatsEntry is one stats row.
type StatsEntry struct {
	Container string `json:"container"`
	Name      string `json:"name,omitempty"`
	CPUPerc   string `json:"cpu_perc,omitempty"`
	MemUsage  string `json:"mem_usage,omitempty"`
	MemPerc   string `json:"mem_perc,omitempty"`
	NetIO     string `json:"net_io,omitempty"`
	BlockIO   string `json:"block_io,omitempty"`
	PIDs      string `json:"pids,omitempty"`
}

// InspectRequest is docker inspect.
type InspectRequest struct {
	Host   string `json:"host"`
	Target string `json:"target"` // container or image id/name
}

// InspectResult holds inspect JSON.
type InspectResult struct {
	Objects []interface{} `json:"objects"`
}

// TopRequest is docker top.
type TopRequest struct {
	Host      string `json:"host"`
	Container string `json:"container"`
	PsArgs    string `json:"ps_args"` // optional args passed to ps
}

// TopResult holds process table text.
type TopResult struct {
	Output string `json:"output"`
}

// HistoryRequest is docker history.
type HistoryRequest struct {
	Host  string `json:"host"`
	Image string `json:"image"`
}

// HistoryEntry is one image history layer.
type HistoryEntry struct {
	ID        string `json:"id,omitempty"`
	Created   string `json:"created,omitempty"`
	CreatedBy string `json:"created_by,omitempty"`
	Size      string `json:"size,omitempty"`
	Comment   string `json:"comment,omitempty"`
	Tags      string `json:"tags,omitempty"`
}

// Connector talks to Docker via remote SSH (host required).
type Connector struct {
	ssh *ssh.Connector
	log *zap.Logger
}

func New(sshConn *ssh.Connector, log *zap.Logger) *Connector {
	if log == nil {
		log = zap.NewNop()
	}
	return &Connector{ssh: sshConn, log: log}
}

// PS lists containers on the named host.
func (c *Connector) PS(ctx context.Context, req PSRequest) ([]ContainerInfo, error) {
	if err := requireHost(req.Host); err != nil {
		return nil, err
	}
	args := []string{"ps", "--format", "{{json .}}"}
	if req.All {
		args = []string{"ps", "-a", "--format", "{{json .}}"}
	}
	stdout, err := c.runDocker(ctx, req.Host, args)
	if err != nil {
		return nil, err
	}
	return parseDockerPSJSON(stdout)
}

// Logs reads container logs on the named host.
func (c *Connector) Logs(ctx context.Context, req LogsRequest) (*LogsResult, error) {
	if err := requireHost(req.Host); err != nil {
		return nil, err
	}
	if req.Container == "" {
		return nil, model.NewAppError(model.ErrInvalidParams, "docker.logs: container is required")
	}
	args := []string{"logs"}
	if req.Timestamps {
		args = append(args, "-t")
	}
	if req.Since != "" {
		args = append(args, "--since", req.Since)
	}
	tail := req.Tail
	if tail == "" {
		tail = "100"
	}
	args = append(args, "--tail", tail, req.Container)

	stdout, err := c.runDocker(ctx, req.Host, args)
	if err != nil {
		return nil, err
	}
	return &LogsResult{Logs: stdout}, nil
}

// Info runs docker info on the named host.
func (c *Connector) Info(ctx context.Context, req InfoRequest) (*InfoResult, error) {
	if err := requireHost(req.Host); err != nil {
		return nil, err
	}
	stdout, err := c.runDocker(ctx, req.Host, []string{"info", "--format", "{{json .}}"})
	if err != nil {
		return nil, err
	}
	var info map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &info); err != nil {
		return &InfoResult{Raw: stdout, Format: "text"}, nil
	}
	return &InfoResult{Info: info, Format: "json"}, nil
}

// Stats runs docker stats --no-stream on the named host.
func (c *Connector) Stats(ctx context.Context, req StatsRequest) ([]StatsEntry, error) {
	if err := requireHost(req.Host); err != nil {
		return nil, err
	}
	args := []string{"stats", "--no-stream", "--format", "{{json .}}"}
	if req.Container != "" {
		args = append(args, req.Container)
	}
	stdout, err := c.runDocker(ctx, req.Host, args)
	if err != nil {
		return nil, err
	}
	return parseDockerStatsJSON(stdout)
}

// Inspect runs docker inspect on the named host.
func (c *Connector) Inspect(ctx context.Context, req InspectRequest) (*InspectResult, error) {
	if err := requireHost(req.Host); err != nil {
		return nil, err
	}
	if req.Target == "" {
		return nil, model.NewAppError(model.ErrInvalidParams, "docker.inspect: target is required")
	}
	stdout, err := c.runDocker(ctx, req.Host, []string{"inspect", req.Target})
	if err != nil {
		return nil, err
	}
	var objects []interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &objects); err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("parse docker inspect: %v", err))
	}
	return &InspectResult{Objects: objects}, nil
}

// Top runs docker top on the named host.
func (c *Connector) Top(ctx context.Context, req TopRequest) (*TopResult, error) {
	if err := requireHost(req.Host); err != nil {
		return nil, err
	}
	if req.Container == "" {
		return nil, model.NewAppError(model.ErrInvalidParams, "docker.top: container is required")
	}
	args := []string{"top", req.Container}
	if req.PsArgs != "" {
		args = append(args, req.PsArgs)
	}
	stdout, err := c.runDocker(ctx, req.Host, args)
	if err != nil {
		return nil, err
	}
	return &TopResult{Output: stdout}, nil
}

// History runs docker history on the named host.
func (c *Connector) History(ctx context.Context, req HistoryRequest) ([]HistoryEntry, error) {
	if err := requireHost(req.Host); err != nil {
		return nil, err
	}
	if req.Image == "" {
		return nil, model.NewAppError(model.ErrInvalidParams, "docker.history: image is required")
	}
	args := []string{"history", "--format", "{{json .}}", "--no-trunc", req.Image}
	stdout, err := c.runDocker(ctx, req.Host, args)
	if err != nil {
		return nil, err
	}
	return parseDockerHistoryJSON(stdout)
}

func requireHost(host string) error {
	if strings.TrimSpace(host) == "" {
		return model.NewAppError(model.ErrInvalidParams, "docker: host is required")
	}
	return nil
}

func (c *Connector) runDocker(ctx context.Context, host string, args []string) (string, error) {
	if c.ssh == nil {
		return "", model.NewAppError(model.ErrConnectorError, "ssh connector not configured")
	}
	res, err := c.ssh.Exec(ctx, ssh.ExecRequest{
		Host:    host,
		Command: "docker",
		Args:    args,
	})
	if err != nil {
		return "", err
	}
	if res.ExitCode != 0 {
		return "", model.NewAppError(model.ErrConnectorError,
			fmt.Sprintf("docker exit %d: %s", res.ExitCode, strings.TrimSpace(res.Stderr)))
	}
	return res.Stdout, nil
}

func parseDockerPSJSON(stdout string) ([]ContainerInfo, error) {
	out := []ContainerInfo{}
	scanner := newLineScanner(stdout)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("parse docker ps: %v", err))
		}
		info := ContainerInfo{
			ID:      strField(raw, "ID", "Id"),
			Image:   strField(raw, "Image"),
			Status:  strField(raw, "Status"),
			State:   strField(raw, "State"),
			Ports:   strField(raw, "Ports"),
			Created: strField(raw, "CreatedAt", "Created"),
		}
		if name := strField(raw, "Names", "Name"); name != "" {
			info.Names = strings.Split(name, ",")
		}
		out = append(out, info)
	}
	if err := scanner.Err(); err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, err.Error())
	}
	return out, nil
}

func parseDockerStatsJSON(stdout string) ([]StatsEntry, error) {
	out := []StatsEntry{}
	scanner := newLineScanner(stdout)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("parse docker stats: %v", err))
		}
		out = append(out, StatsEntry{
			Container: strField(raw, "Container", "ID", "Id"),
			Name:      strField(raw, "Name", "Names"),
			CPUPerc:   strField(raw, "CPUPerc", "CPUPercentage"),
			MemUsage:  strField(raw, "MemUsage", "MemoryUsage"),
			MemPerc:   strField(raw, "MemPerc", "MemoryPercentage"),
			NetIO:     strField(raw, "NetIO", "NetIO"),
			BlockIO:   strField(raw, "BlockIO", "BlockIO"),
			PIDs:      strField(raw, "PIDs", "Pids"),
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, err.Error())
	}
	return out, nil
}

func parseDockerHistoryJSON(stdout string) ([]HistoryEntry, error) {
	out := []HistoryEntry{}
	scanner := newLineScanner(stdout)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("parse docker history: %v", err))
		}
		out = append(out, HistoryEntry{
			ID:        strField(raw, "ID", "Id"),
			Created:   strField(raw, "CreatedAt", "Created"),
			CreatedBy: strField(raw, "CreatedBy", "CreatedBy"),
			Size:      strField(raw, "Size"),
			Comment:   strField(raw, "Comment"),
			Tags:      strField(raw, "Tags"),
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, err.Error())
	}
	return out, nil
}

func newLineScanner(stdout string) *bufio.Scanner {
	scanner := bufio.NewScanner(strings.NewReader(stdout))
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	return scanner
}

func strField(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case string:
				return t
			default:
				return fmt.Sprint(t)
			}
		}
	}
	return ""
}
