package redis

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/fsj00/ops-mcp/internal/config"
	"github.com/fsj00/ops-mcp/internal/model"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Connector runs Redis read-only ops against named instances from redis.yaml.
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

// --- request / result types ---
// DB is the Redis logical database index; omit or 0 means db 0.

type PingRequest struct {
	Redis string `json:"redis"`
	DB    int    `json:"db"`
}

type PingResult struct {
	Result string `json:"result"`
}

type InfoRequest struct {
	Redis   string `json:"redis"`
	DB      int    `json:"db"`
	Section string `json:"section"`
}

type InfoResult struct {
	Info string `json:"info"`
}

type RoleRequest struct {
	Redis string `json:"redis"`
	DB    int    `json:"db"`
}

type RoleResult struct {
	Role []interface{} `json:"role"`
}

type DBSizeRequest struct {
	Redis string `json:"redis"`
	DB    int    `json:"db"`
}

type DBSizeResult struct {
	DBSize int64 `json:"dbsize"`
}

type ScanRequest struct {
	Redis  string `json:"redis"`
	DB     int    `json:"db"`
	Cursor uint64 `json:"cursor"`
	Match  string `json:"match"`
	Limit  int    `json:"limit"`
}

type ScanResult struct {
	Cursor    uint64   `json:"cursor"`
	Keys      []string `json:"keys"`
	Count     int      `json:"count"`
	Truncated bool     `json:"truncated"`
}

type KeyRequest struct {
	Redis string `json:"redis"`
	DB    int    `json:"db"`
	Key   string `json:"key"`
}

type TypeResult struct {
	Key  string `json:"key"`
	Type string `json:"type"`
}

type TTLResult struct {
	Key string `json:"key"`
	TTL int64  `json:"ttl"`
}

type ExistsResult struct {
	Key    string `json:"key"`
	Exists bool   `json:"exists"`
}

type GetResult struct {
	Key   string  `json:"key"`
	Value *string `json:"value"` // nil when key is missing
}

type MemoryUsageResult struct {
	Key   string `json:"key"`
	Bytes *int64 `json:"bytes"`
}

type ObjectEncodingResult struct {
	Key      string  `json:"key"`
	Encoding *string `json:"encoding"`
}

type SlowlogGetRequest struct {
	Redis string `json:"redis"`
	DB    int    `json:"db"`
	Count int    `json:"count"`
}

type SlowlogEntry struct {
	ID        int64         `json:"id"`
	Timestamp int64         `json:"timestamp"`
	Duration  int64         `json:"duration_us"`
	Command   []interface{} `json:"command"`
}

type SlowlogGetResult struct {
	Entries []SlowlogEntry `json:"entries"`
	Count   int            `json:"count"`
}

type ClientListRequest struct {
	Redis string `json:"redis"`
	DB    int    `json:"db"`
	Limit int    `json:"limit"`
}

type ClientListResult struct {
	Clients   []string `json:"clients"`
	Count     int      `json:"count"`
	Truncated bool     `json:"truncated"`
}

type ConfigGetRequest struct {
	Redis   string `json:"redis"`
	DB      int    `json:"db"`
	Pattern string `json:"pattern"`
}

type ConfigGetResult struct {
	Config map[string]string `json:"config"`
}

type CardinalityResult struct {
	Key   string `json:"key"`
	Count int64  `json:"count"`
}

type ZRangeSampleRequest struct {
	Redis string `json:"redis"`
	DB    int    `json:"db"`
	Key   string `json:"key"`
	Limit int    `json:"limit"`
}

type ZRangeSampleResult struct {
	Key       string                   `json:"key"`
	Members   []map[string]interface{} `json:"members"`
	Count     int                      `json:"count"`
	Truncated bool                     `json:"truncated"`
}

type LRangeRequest struct {
	Redis string `json:"redis"`
	DB    int    `json:"db"`
	Key   string `json:"key"`
	Start int64  `json:"start"`
	Limit int    `json:"limit"`
}

type LRangeResult struct {
	Key       string   `json:"key"`
	Start     int64    `json:"start"`
	Values    []string `json:"values"`
	Count     int      `json:"count"`
	Truncated bool     `json:"truncated"`
}

type HGetRequest struct {
	Redis string `json:"redis"`
	DB    int    `json:"db"`
	Key   string `json:"key"`
	Field string `json:"field"`
}

type HGetResult struct {
	Key   string  `json:"key"`
	Field string  `json:"field"`
	Value *string `json:"value"` // nil when field is missing
}

type HMGetRequest struct {
	Redis  string   `json:"redis"`
	DB     int      `json:"db"`
	Key    string   `json:"key"`
	Fields []string `json:"fields"`
}

type HMGetFieldValue struct {
	Field string  `json:"field"`
	Value *string `json:"value"`
}

type HMGetResult struct {
	Key    string            `json:"key"`
	Fields []HMGetFieldValue `json:"fields"`
	Count  int               `json:"count"`
}

type HScanRequest struct {
	Redis  string `json:"redis"`
	DB     int    `json:"db"`
	Key    string `json:"key"`
	Cursor uint64 `json:"cursor"`
	Match  string `json:"match"`
	Limit  int    `json:"limit"`
}

type HScanFieldValue struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

type HScanResult struct {
	Key       string            `json:"key"`
	Cursor    uint64            `json:"cursor"`
	Fields    []HScanFieldValue `json:"fields"`
	Count     int               `json:"count"`
	Truncated bool              `json:"truncated"`
}

// --- operations ---

func (c *Connector) Ping(ctx context.Context, req PingRequest) (*PingResult, error) {
	client, name, err := c.dial(ctx, req.Redis, req.DB)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	c.log.Info("redis ping", zap.String("redis", name))
	res, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("redis ping: %v", err))
	}
	return &PingResult{Result: res}, nil
}

func (c *Connector) Info(ctx context.Context, req InfoRequest) (*InfoResult, error) {
	client, name, err := c.dial(ctx, req.Redis, req.DB)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	c.log.Info("redis info", zap.String("redis", name), zap.String("section", req.Section))
	var info string
	if strings.TrimSpace(req.Section) == "" {
		info, err = client.Info(ctx).Result()
	} else {
		info, err = client.Info(ctx, req.Section).Result()
	}
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("redis info: %v", err))
	}
	return &InfoResult{Info: info}, nil
}

func (c *Connector) Role(ctx context.Context, req RoleRequest) (*RoleResult, error) {
	client, name, err := c.dial(ctx, req.Redis, req.DB)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	c.log.Info("redis role", zap.String("redis", name))
	raw, err := client.Do(ctx, "ROLE").Result()
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("redis role: %v", err))
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return &RoleResult{Role: []interface{}{raw}}, nil
	}
	return &RoleResult{Role: arr}, nil
}

func (c *Connector) DBSize(ctx context.Context, req DBSizeRequest) (*DBSizeResult, error) {
	client, name, err := c.dial(ctx, req.Redis, req.DB)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	c.log.Info("redis dbsize", zap.String("redis", name))
	n, err := client.DBSize(ctx).Result()
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("redis dbsize: %v", err))
	}
	return &DBSizeResult{DBSize: n}, nil
}

func (c *Connector) Scan(ctx context.Context, req ScanRequest) (*ScanResult, error) {
	cfg, err := c.lookup(req.Redis)
	if err != nil {
		return nil, err
	}
	limit, err := ClampLimit(req.Limit, cfg.Limit)
	if err != nil {
		return nil, model.NewAppError(model.ErrInvalidParams, "redis scan: "+err.Error())
	}
	client, name, err := c.dial(ctx, req.Redis, req.DB)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	match := req.Match
	if match == "" {
		match = "*"
	}
	c.log.Info("redis scan",
		zap.String("redis", name),
		zap.Uint64("cursor", req.Cursor),
		zap.String("match", match),
		zap.Int("limit", limit),
	)

	keys := make([]string, 0, limit)
	cursor := req.Cursor
	truncated := false
	for {
		batch, next, err := client.Scan(ctx, cursor, match, int64(limit)).Result()
		if err != nil {
			return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("redis scan: %v", err))
		}
		for _, k := range batch {
			if len(keys) >= limit {
				truncated = true
				break
			}
			keys = append(keys, k)
		}
		cursor = next
		if truncated || cursor == 0 || len(keys) >= limit {
			if len(keys) >= limit && cursor != 0 {
				truncated = true
			}
			break
		}
	}
	return &ScanResult{
		Cursor:    cursor,
		Keys:      keys,
		Count:     len(keys),
		Truncated: truncated,
	}, nil
}

func (c *Connector) Type(ctx context.Context, req KeyRequest) (*TypeResult, error) {
	if err := requireKey(req.Key); err != nil {
		return nil, err
	}
	client, name, err := c.dial(ctx, req.Redis, req.DB)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	c.log.Info("redis type", zap.String("redis", name), zap.String("key", req.Key))
	typ, err := client.Type(ctx, req.Key).Result()
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("redis type: %v", err))
	}
	return &TypeResult{Key: req.Key, Type: typ}, nil
}

func (c *Connector) TTL(ctx context.Context, req KeyRequest) (*TTLResult, error) {
	if err := requireKey(req.Key); err != nil {
		return nil, err
	}
	client, name, err := c.dial(ctx, req.Redis, req.DB)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	c.log.Info("redis ttl", zap.String("redis", name), zap.String("key", req.Key))
	// Use raw TTL integer: -2 missing, -1 no expiry, >=0 seconds remaining.
	raw, err := client.Do(ctx, "TTL", req.Key).Int64()
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("redis ttl: %v", err))
	}
	return &TTLResult{Key: req.Key, TTL: raw}, nil
}

func (c *Connector) Exists(ctx context.Context, req KeyRequest) (*ExistsResult, error) {
	if err := requireKey(req.Key); err != nil {
		return nil, err
	}
	client, name, err := c.dial(ctx, req.Redis, req.DB)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	c.log.Info("redis exists", zap.String("redis", name), zap.String("key", req.Key))
	n, err := client.Exists(ctx, req.Key).Result()
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("redis exists: %v", err))
	}
	return &ExistsResult{Key: req.Key, Exists: n > 0}, nil
}

// Get runs Redis GET on a string key. Missing key returns value=null; non-string types error.
func (c *Connector) Get(ctx context.Context, req KeyRequest) (*GetResult, error) {
	if err := requireKey(req.Key); err != nil {
		return nil, err
	}
	client, name, err := c.dial(ctx, req.Redis, req.DB)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	c.log.Info("redis get", zap.String("redis", name), zap.String("key", req.Key))
	val, err := client.Get(ctx, req.Key).Result()
	if err == goredis.Nil {
		return &GetResult{Key: req.Key, Value: nil}, nil
	}
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("redis get: %v", err))
	}
	return &GetResult{Key: req.Key, Value: &val}, nil
}

func (c *Connector) MemoryUsage(ctx context.Context, req KeyRequest) (*MemoryUsageResult, error) {
	if err := requireKey(req.Key); err != nil {
		return nil, err
	}
	client, name, err := c.dial(ctx, req.Redis, req.DB)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	c.log.Info("redis memory_usage", zap.String("redis", name), zap.String("key", req.Key))
	n, err := client.MemoryUsage(ctx, req.Key).Result()
	if err == goredis.Nil {
		return &MemoryUsageResult{Key: req.Key, Bytes: nil}, nil
	}
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("redis memory usage: %v", err))
	}
	return &MemoryUsageResult{Key: req.Key, Bytes: &n}, nil
}

func (c *Connector) ObjectEncoding(ctx context.Context, req KeyRequest) (*ObjectEncodingResult, error) {
	if err := requireKey(req.Key); err != nil {
		return nil, err
	}
	client, name, err := c.dial(ctx, req.Redis, req.DB)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	c.log.Info("redis object_encoding", zap.String("redis", name), zap.String("key", req.Key))
	enc, err := client.ObjectEncoding(ctx, req.Key).Result()
	if err == goredis.Nil {
		return &ObjectEncodingResult{Key: req.Key, Encoding: nil}, nil
	}
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("redis object encoding: %v", err))
	}
	return &ObjectEncodingResult{Key: req.Key, Encoding: &enc}, nil
}

func (c *Connector) SlowlogGet(ctx context.Context, req SlowlogGetRequest) (*SlowlogGetResult, error) {
	cfg, err := c.lookup(req.Redis)
	if err != nil {
		return nil, err
	}
	count, err := ClampLimit(req.Count, cfg.Limit)
	if err != nil {
		return nil, model.NewAppError(model.ErrInvalidParams, "redis slowlog_get: count "+err.Error())
	}
	client, name, err := c.dial(ctx, req.Redis, req.DB)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	c.log.Info("redis slowlog_get", zap.String("redis", name), zap.Int("count", count))
	entries, err := client.SlowLogGet(ctx, int64(count)).Result()
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("redis slowlog get: %v", err))
	}
	out := make([]SlowlogEntry, 0, len(entries))
	for _, e := range entries {
		cmd := make([]interface{}, len(e.Args))
		for i, a := range e.Args {
			cmd[i] = a
		}
		out = append(out, SlowlogEntry{
			ID:        e.ID,
			Timestamp: e.Time.Unix(),
			Duration:  e.Duration.Microseconds(),
			Command:   cmd,
		})
	}
	return &SlowlogGetResult{Entries: out, Count: len(out)}, nil
}

func (c *Connector) ClientList(ctx context.Context, req ClientListRequest) (*ClientListResult, error) {
	cfg, err := c.lookup(req.Redis)
	if err != nil {
		return nil, err
	}
	limit, err := ClampLimit(req.Limit, cfg.Limit)
	if err != nil {
		return nil, model.NewAppError(model.ErrInvalidParams, "redis client_list: "+err.Error())
	}
	client, name, err := c.dial(ctx, req.Redis, req.DB)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	c.log.Info("redis client_list", zap.String("redis", name), zap.Int("limit", limit))
	raw, err := client.ClientList(ctx).Result()
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("redis client list: %v", err))
	}
	lines := splitNonEmpty(raw)
	truncated := false
	if len(lines) > limit {
		lines = lines[:limit]
		truncated = true
	}
	return &ClientListResult{Clients: lines, Count: len(lines), Truncated: truncated}, nil
}

func (c *Connector) ConfigGet(ctx context.Context, req ConfigGetRequest) (*ConfigGetResult, error) {
	pattern := strings.TrimSpace(req.Pattern)
	if pattern == "" {
		return nil, model.NewAppError(model.ErrInvalidParams, "redis config_get: pattern is required")
	}
	client, name, err := c.dial(ctx, req.Redis, req.DB)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	c.log.Info("redis config_get", zap.String("redis", name), zap.String("pattern", pattern))
	// Only CONFIG GET — never CONFIG SET.
	vals, err := client.ConfigGet(ctx, pattern).Result()
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("redis config get: %v", err))
	}
	return &ConfigGetResult{Config: vals}, nil
}

func (c *Connector) HLen(ctx context.Context, req KeyRequest) (*CardinalityResult, error) {
	return c.cardOp(ctx, "hlen", req, func(ctx context.Context, client *goredis.Client, key string) (int64, error) {
		return client.HLen(ctx, key).Result()
	})
}

func (c *Connector) LLen(ctx context.Context, req KeyRequest) (*CardinalityResult, error) {
	return c.cardOp(ctx, "llen", req, func(ctx context.Context, client *goredis.Client, key string) (int64, error) {
		return client.LLen(ctx, key).Result()
	})
}

func (c *Connector) SCard(ctx context.Context, req KeyRequest) (*CardinalityResult, error) {
	return c.cardOp(ctx, "scard", req, func(ctx context.Context, client *goredis.Client, key string) (int64, error) {
		return client.SCard(ctx, key).Result()
	})
}

func (c *Connector) ZCard(ctx context.Context, req KeyRequest) (*CardinalityResult, error) {
	return c.cardOp(ctx, "zcard", req, func(ctx context.Context, client *goredis.Client, key string) (int64, error) {
		return client.ZCard(ctx, key).Result()
	})
}

func (c *Connector) ZRangeSample(ctx context.Context, req ZRangeSampleRequest) (*ZRangeSampleResult, error) {
	if err := requireKey(req.Key); err != nil {
		return nil, err
	}
	cfg, err := c.lookup(req.Redis)
	if err != nil {
		return nil, err
	}
	limit, err := ClampLimit(req.Limit, cfg.Limit)
	if err != nil {
		return nil, model.NewAppError(model.ErrInvalidParams, "redis zrange_sample: "+err.Error())
	}
	client, name, err := c.dial(ctx, req.Redis, req.DB)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	c.log.Info("redis zrange_sample",
		zap.String("redis", name),
		zap.String("key", req.Key),
		zap.Int("limit", limit),
	)
	// Fetch one extra to detect truncation without dumping the whole set.
	zs, err := client.ZRangeWithScores(ctx, req.Key, 0, int64(limit)).Result()
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("redis zrange: %v", err))
	}
	truncated := len(zs) > limit
	if truncated {
		zs = zs[:limit]
	}
	members := make([]map[string]interface{}, 0, len(zs))
	for _, z := range zs {
		members = append(members, map[string]interface{}{
			"member": z.Member,
			"score":  z.Score,
		})
	}
	return &ZRangeSampleResult{
		Key:       req.Key,
		Members:   members,
		Count:     len(members),
		Truncated: truncated,
	}, nil
}

// LRange returns up to limit list elements starting at start (inclusive).
func (c *Connector) LRange(ctx context.Context, req LRangeRequest) (*LRangeResult, error) {
	if err := requireKey(req.Key); err != nil {
		return nil, err
	}
	cfg, err := c.lookup(req.Redis)
	if err != nil {
		return nil, err
	}
	limit, err := ClampLimit(req.Limit, cfg.Limit)
	if err != nil {
		return nil, model.NewAppError(model.ErrInvalidParams, "redis lrange: "+err.Error())
	}
	client, name, err := c.dial(ctx, req.Redis, req.DB)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	c.log.Info("redis lrange",
		zap.String("redis", name),
		zap.String("key", req.Key),
		zap.Int64("start", req.Start),
		zap.Int("limit", limit),
	)
	// Inclusive stop = start+limit fetches one extra element to detect truncation.
	stop := req.Start + int64(limit)
	vals, err := client.LRange(ctx, req.Key, req.Start, stop).Result()
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("redis lrange: %v", err))
	}
	truncated := len(vals) > limit
	if truncated {
		vals = vals[:limit]
	}
	return &LRangeResult{
		Key:       req.Key,
		Start:     req.Start,
		Values:    vals,
		Count:     len(vals),
		Truncated: truncated,
	}, nil
}

// HGet runs HGET. Missing field returns value=null.
func (c *Connector) HGet(ctx context.Context, req HGetRequest) (*HGetResult, error) {
	if err := requireKey(req.Key); err != nil {
		return nil, err
	}
	if err := requireField(req.Field); err != nil {
		return nil, err
	}
	client, name, err := c.dial(ctx, req.Redis, req.DB)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	c.log.Info("redis hget",
		zap.String("redis", name),
		zap.String("key", req.Key),
		zap.String("field", req.Field),
	)
	val, err := client.HGet(ctx, req.Key, req.Field).Result()
	if err == goredis.Nil {
		return &HGetResult{Key: req.Key, Field: req.Field, Value: nil}, nil
	}
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("redis hget: %v", err))
	}
	return &HGetResult{Key: req.Key, Field: req.Field, Value: &val}, nil
}

// HMGet runs HMGET for the given fields (order preserved). Field count capped by redis.yaml limit.
func (c *Connector) HMGet(ctx context.Context, req HMGetRequest) (*HMGetResult, error) {
	if err := requireKey(req.Key); err != nil {
		return nil, err
	}
	fields, err := normalizeFields(req.Fields)
	if err != nil {
		return nil, err
	}
	cfg, err := c.lookup(req.Redis)
	if err != nil {
		return nil, err
	}
	maxFields := cfg.Limit
	if maxFields <= 0 {
		maxFields = config.DefaultQueryLimit
	}
	if len(fields) > maxFields {
		return nil, model.NewAppError(model.ErrInvalidParams,
			fmt.Sprintf("redis hmget: fields count %d exceeds limit %d", len(fields), maxFields))
	}
	client, name, err := c.dial(ctx, req.Redis, req.DB)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	c.log.Info("redis hmget",
		zap.String("redis", name),
		zap.String("key", req.Key),
		zap.Int("fields", len(fields)),
	)
	vals, err := client.HMGet(ctx, req.Key, fields...).Result()
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("redis hmget: %v", err))
	}
	out := make([]HMGetFieldValue, 0, len(fields))
	for i, f := range fields {
		var ptr *string
		if i < len(vals) && vals[i] != nil {
			s := fmt.Sprint(vals[i])
			ptr = &s
		}
		out = append(out, HMGetFieldValue{Field: f, Value: ptr})
	}
	return &HMGetResult{Key: req.Key, Fields: out, Count: len(out)}, nil
}

// HScan scans hash fields with a required limit (like SCAN).
func (c *Connector) HScan(ctx context.Context, req HScanRequest) (*HScanResult, error) {
	if err := requireKey(req.Key); err != nil {
		return nil, err
	}
	cfg, err := c.lookup(req.Redis)
	if err != nil {
		return nil, err
	}
	limit, err := ClampLimit(req.Limit, cfg.Limit)
	if err != nil {
		return nil, model.NewAppError(model.ErrInvalidParams, "redis hscan: "+err.Error())
	}
	client, name, err := c.dial(ctx, req.Redis, req.DB)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	match := req.Match
	if match == "" {
		match = "*"
	}
	c.log.Info("redis hscan",
		zap.String("redis", name),
		zap.String("key", req.Key),
		zap.Uint64("cursor", req.Cursor),
		zap.String("match", match),
		zap.Int("limit", limit),
	)

	fields := make([]HScanFieldValue, 0, limit)
	cursor := req.Cursor
	truncated := false
	for {
		batch, next, err := client.HScan(ctx, req.Key, cursor, match, int64(limit)).Result()
		if err != nil {
			return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("redis hscan: %v", err))
		}
		for i := 0; i+1 < len(batch); i += 2 {
			if len(fields) >= limit {
				truncated = true
				break
			}
			fields = append(fields, HScanFieldValue{Field: batch[i], Value: batch[i+1]})
		}
		cursor = next
		if truncated || cursor == 0 || len(fields) >= limit {
			if len(fields) >= limit && cursor != 0 {
				truncated = true
			}
			break
		}
	}
	return &HScanResult{
		Key:       req.Key,
		Cursor:    cursor,
		Fields:    fields,
		Count:     len(fields),
		Truncated: truncated,
	}, nil
}

func (c *Connector) cardOp(
	ctx context.Context,
	op string,
	req KeyRequest,
	fn func(context.Context, *goredis.Client, string) (int64, error),
) (*CardinalityResult, error) {
	if err := requireKey(req.Key); err != nil {
		return nil, err
	}
	client, name, err := c.dial(ctx, req.Redis, req.DB)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	c.log.Info("redis "+op, zap.String("redis", name), zap.String("key", req.Key))
	n, err := fn(ctx, client, req.Key)
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("redis %s: %v", op, err))
	}
	return &CardinalityResult{Key: req.Key, Count: n}, nil
}

func (c *Connector) lookup(name string) (model.RedisInstance, error) {
	if name == "" {
		return model.RedisInstance{}, model.NewAppError(model.ErrInvalidParams, "redis: redis name is required")
	}
	cfg, err := c.cfg.GetRedis(name)
	if err != nil {
		return model.RedisInstance{}, model.NewAppError(model.ErrConnectorError, err.Error())
	}
	return cfg, nil
}

func (c *Connector) dial(ctx context.Context, name string, db int) (*goredis.Client, string, error) {
	cfg, err := c.lookup(name)
	if err != nil {
		return nil, "", err
	}
	if db < 0 {
		return nil, "", model.NewAppError(model.ErrInvalidParams, "redis: db must be >= 0")
	}
	conn := cfg.Connection
	port := conn.Port
	if port <= 0 {
		port = 6379
	}
	opts := &goredis.Options{
		Addr: net.JoinHostPort(conn.Host, strconv.Itoa(port)),
		DB:   db,
	}
	if conn.Username != "" {
		opts.Username = conn.Username
	}
	if conn.Password != "" {
		opts.Password = conn.Password
	}
	tlsCfg, err := buildTLSConfig(conn.Host, conn.TLS)
	if err != nil {
		return nil, "", model.NewAppError(model.ErrInvalidParams, err.Error())
	}
	if tlsCfg != nil {
		opts.TLSConfig = tlsCfg
	}
	c.log.Info("redis dial",
		zap.String("redis", name),
		zap.String("host", conn.Host),
		zap.Int("port", port),
		zap.Int("db", db),
		zap.Bool("tls", conn.TLS.Enabled),
		zap.Bool("mtls", conn.TLS.HasClientCert()),
	)
	client := goredis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		c.log.Warn("redis dial failed",
			zap.String("redis", name),
			zap.String("host", conn.Host),
			zap.Int("port", port),
			zap.Int("db", db),
			zap.Bool("tls", conn.TLS.Enabled),
			zap.Error(err),
		)
		return nil, "", model.NewAppError(model.ErrConnectorError, fmt.Sprintf("redis dial %s: %v", name, err))
	}
	return client, name, nil
}

func requireKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return model.NewAppError(model.ErrInvalidParams, "redis: key is required")
	}
	return nil
}

func requireField(field string) error {
	if strings.TrimSpace(field) == "" {
		return model.NewAppError(model.ErrInvalidParams, "redis: field is required")
	}
	return nil
}

func normalizeFields(fields []string) ([]string, error) {
	if len(fields) == 0 {
		return nil, model.NewAppError(model.ErrInvalidParams, "redis: fields is required and must be non-empty")
	}
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" {
			return nil, model.NewAppError(model.ErrInvalidParams, "redis: fields must not contain empty values")
		}
		out = append(out, f)
	}
	return out, nil
}

func splitNonEmpty(s string) []string {
	raw := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
