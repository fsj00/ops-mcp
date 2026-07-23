package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/fsj00/ops-mcp/internal/config"
	"github.com/fsj00/ops-mcp/internal/mcp"
	"github.com/fsj00/ops-mcp/internal/plugin"
	"github.com/fsj00/ops-mcp/web"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Server wires HTTP routes.
type Server struct {
	cfg     *config.Manager
	plugins *plugin.Manager
	mcp     *mcp.Server
	log     *zap.Logger
}

func New(cfg *config.Manager, plugins *plugin.Manager, mcpServer *mcp.Server, log *zap.Logger) *Server {
	if log == nil {
		log = zap.NewNop()
	}
	return &Server{cfg: cfg, plugins: plugins, mcp: mcpServer, log: log}
}

// Router returns the Gin engine.
func (s *Server) Router() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(s.accessLog())
	r.Use(s.requireAuth())

	r.POST("/mcp", s.handleMCP)
	r.GET("/mcp", s.handleMCPGet)
	r.DELETE("/mcp", s.handleMCPDelete)

	r.GET("/api/plugins", s.listPlugins)
	r.GET("/api/tools", s.listTools)
	r.GET("/api/hosts", s.listHosts)
	r.GET("/api/databases", s.listDatabases)
	r.GET("/api/redis", s.listRedis)
	r.GET("/api/kafka", s.listKafka)
	r.GET("/api/apis", s.listAPIs)
	r.GET("/api/commands", s.listCommands)
	r.GET("/api/snmp", s.listSNMP)
	r.POST("/api/reload", s.reload)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/", s.serveIndex)
	r.GET("/index.html", s.serveIndex)

	return r
}

func (s *Server) serveIndex(c *gin.Context) {
	data, err := web.FS.ReadFile("index.html")
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", data)
}

func (s *Server) accessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		s.log.Debug("http",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
		)
	}
}

func (s *Server) handleMCP(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "read body"})
		return
	}
	if len(body) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty body"})
		return
	}

	sessionID := c.GetHeader("Mcp-Session-Id")
	resp, err := s.mcp.Handle(c.Request.Context(), body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Detect initialize to mint session id (Streamable HTTP header only).
	var peek struct {
		Method string `json:"method"`
	}
	_ = json.Unmarshal(body, &peek)
	if peek.Method == "initialize" {
		if sessionID == "" {
			sessionID = uuid.NewString()
		}
		c.Header("Mcp-Session-Id", sessionID)
	} else if sessionID != "" {
		c.Header("Mcp-Session-Id", sessionID)
	}

	c.Header("Content-Type", "application/json")
	if resp == nil {
		c.Status(http.StatusAccepted)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (s *Server) handleMCPGet(c *gin.Context) {
	// Streamable HTTP optional SSE stream; MVP returns 405 Method Not Allowed style hint.
	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusMethodNotAllowed, gin.H{
		"error": "SSE stream not required for ops-mcp MVP; use POST /mcp",
	})
}

func (s *Server) handleMCPDelete(c *gin.Context) {
	c.Status(http.StatusOK)
}

func (s *Server) listPlugins(c *gin.Context) {
	list := s.plugins.List()
	out := make([]gin.H, 0, len(list))
	for _, p := range list {
		out = append(out, gin.H{
			"name":        p.Name,
			"version":     p.Version,
			"description": p.Description,
			"type":        p.Type,
			"target":      p.Target,
			"runtime":     p.Runtime,
			"timeout":     p.Timeout,
			"path":        p.Path,
		})
	}
	c.JSON(http.StatusOK, gin.H{"plugins": out})
}

func (s *Server) listTools(c *gin.Context) {
	c.JSON(http.StatusOK, s.mcp.ToolsList())
}

func (s *Server) listHosts(c *gin.Context) {
	list := s.cfg.ListHostSummaries()
	c.JSON(http.StatusOK, gin.H{
		"hosts": list,
		"count": len(list),
	})
}

func (s *Server) listDatabases(c *gin.Context) {
	list := s.cfg.ListDatabaseSummaries()
	c.JSON(http.StatusOK, gin.H{
		"databases": list,
		"count":     len(list),
	})
}

func (s *Server) listRedis(c *gin.Context) {
	list := s.cfg.ListRedisSummaries()
	c.JSON(http.StatusOK, gin.H{
		"redis": list,
		"count": len(list),
	})
}

func (s *Server) listKafka(c *gin.Context) {
	list := s.cfg.ListKafkaSummaries()
	c.JSON(http.StatusOK, gin.H{
		"kafka": list,
		"count": len(list),
	})
}

func (s *Server) listAPIs(c *gin.Context) {
	if reg := s.mcp.APITools(); reg != nil {
		summaries := reg.ListAPISummaries()
		c.JSON(http.StatusOK, gin.H{
			"apis":  summaries,
			"count": len(summaries),
		})
		return
	}
	summaries := s.cfg.ListAPISummaries()
	c.JSON(http.StatusOK, gin.H{
		"apis":  summaries,
		"count": len(summaries),
	})
}

func (s *Server) listCommands(c *gin.Context) {
	list := s.cfg.ListCommandSummaries()
	c.JSON(http.StatusOK, gin.H{
		"commands": list,
		"count":    len(list),
	})
}

func (s *Server) listSNMP(c *gin.Context) {
	opts := config.SNMPDeviceListOptions{
		Labels: parseLabelQuery(append([]string{c.Query("labels")}, c.QueryArray("label")...)...),
	}
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			opts.Limit = n
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			opts.Offset = n
		}
	}
	list := s.cfg.ListSNMPDeviceSummaries(opts)
	c.JSON(http.StatusOK, gin.H{
		"devices": list,
		"count":   len(list),
	})
}

// parseLabelQuery accepts repeated ?label=k=v and/or comma-separated ?labels=k=v,k2=v2.
func parseLabelQuery(parts ...string) map[string]string {
	out := map[string]string{}
	for _, part := range parts {
		for _, pair := range strings.Split(part, ",") {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			k, v, ok := strings.Cut(pair, "=")
			if !ok || k == "" {
				continue
			}
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

type reloadRequest struct {
	Plugins bool `json:"plugins"`
	Config  bool `json:"config"`
}

func (s *Server) reload(c *gin.Context) {
	var req reloadRequest
	_ = c.ShouldBindJSON(&req)
	// Default: reload plugins.
	if !req.Plugins && !req.Config {
		req.Plugins = true
	}
	if req.Config {
		if err := s.cfg.ReloadConfig(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if reg := s.mcp.APITools(); reg != nil {
			if _, err := reg.Load(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
	}
	pluginsCount := s.plugins.Count()
	if req.Plugins {
		n, err := s.plugins.Load()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		pluginsCount = n
	}
	c.JSON(http.StatusOK, gin.H{
		"reloaded":      true,
		"plugins_count": pluginsCount,
		"tools_count":   s.mcp.ToolsCount(),
	})
}
