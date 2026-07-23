package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dop251/goja"
	"github.com/fsj00/ops-mcp/internal/config"
	cmdconn "github.com/fsj00/ops-mcp/internal/connector/command"
	"github.com/fsj00/ops-mcp/internal/connector/docker"
	httpconn "github.com/fsj00/ops-mcp/internal/connector/http"
	kafkaconn "github.com/fsj00/ops-mcp/internal/connector/kafka"
	"github.com/fsj00/ops-mcp/internal/connector/mysql"
	"github.com/fsj00/ops-mcp/internal/connector/postgres"
	redisconn "github.com/fsj00/ops-mcp/internal/connector/redis"
	snmpconn "github.com/fsj00/ops-mcp/internal/connector/snmp"
	"github.com/fsj00/ops-mcp/internal/connector/ssh"
	tcpconn "github.com/fsj00/ops-mcp/internal/connector/tcp"
	udpconn "github.com/fsj00/ops-mcp/internal/connector/udp"
	"github.com/fsj00/ops-mcp/internal/model"
	"go.uber.org/zap"
)

// APISummarizer lists API service summaries (with tool_count when available).
type APISummarizer interface {
	ListAPISummaries() []model.APISummary
}

// Dependencies are injected Connector bridges.
type Dependencies struct {
	SSH      *ssh.Connector
	Docker   *docker.Connector
	MySQL    *mysql.Connector
	Postgres *postgres.Connector
	Redis    *redisconn.Connector
	Kafka    *kafkaconn.Connector
	HTTP     *httpconn.Connector
	Command  *cmdconn.Connector
	SNMP     *snmpconn.Connector
	TCP      *tcpconn.Connector
	UDP      *udpconn.Connector
	APIs     APISummarizer // optional; falls back to Cfg.ListAPISummaries
	Cfg      *config.Manager
	Log      *zap.Logger
}

// Runtime executes plugin JavaScript via Goja.
type Runtime struct {
	deps Dependencies
}

func New(deps Dependencies) *Runtime {
	if deps.Log == nil {
		deps.Log = zap.NewNop()
	}
	return &Runtime{deps: deps}
}

// Execute runs main.js execute(ctx) with the given params.
func (r *Runtime) Execute(ctx context.Context, script string, params map[string]interface{}, timeout time.Duration) (interface{}, error) {
	vm := goja.New()
	vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))

	bridgeCtx := r.buildCtx(ctx, vm, params)
	if err := vm.Set("ctx", bridgeCtx); err != nil {
		return nil, model.NewAppError(model.ErrRuntimeError, err.Error())
	}

	if _, err := vm.RunString(script); err != nil {
		return nil, model.NewAppError(model.ErrRuntimeError, fmt.Sprintf("load script: %v", err))
	}

	fn, ok := goja.AssertFunction(vm.Get("execute"))
	if !ok {
		return nil, model.NewAppError(model.ErrRuntimeError, "main.js must define function execute(ctx)")
	}

	type result struct {
		val interface{}
		err error
	}
	ch := make(chan result, 1)

	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				ch <- result{err: model.NewAppError(model.ErrRuntimeError, fmt.Sprintf("panic: %v", rec))}
			}
		}()
		v, err := fn(goja.Undefined(), vm.Get("ctx"))
		if err != nil {
			ch <- result{err: wrapGojaError(err)}
			return
		}
		ch <- result{val: exportValue(v)}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		vm.Interrupt("context canceled")
		return nil, model.NewAppError(model.ErrPluginTimeout, "plugin execution canceled")
	case <-timer.C:
		vm.Interrupt("timeout")
		return nil, model.NewAppError(model.ErrPluginTimeout, fmt.Sprintf("plugin exceeded timeout %s", timeout))
	case res := <-ch:
		return res.val, res.err
	}
}

func (r *Runtime) buildCtx(ctx context.Context, vm *goja.Runtime, params map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"params": params,
		"ssh": map[string]interface{}{
			"exec": r.wrapSSHExec(ctx, vm),
		},
		"docker": map[string]interface{}{
			"ps":      r.wrapDockerPS(ctx, vm),
			"logs":    r.wrapDockerLogs(ctx, vm),
			"info":    r.wrapDockerInfo(ctx, vm),
			"stats":   r.wrapDockerStats(ctx, vm),
			"inspect": r.wrapDockerInspect(ctx, vm),
			"top":     r.wrapDockerTop(ctx, vm),
			"history": r.wrapDockerHistory(ctx, vm),
		},
		"mysql": map[string]interface{}{
			"query":   r.wrapMySQLQuery(ctx, vm),
			"version": r.wrapMySQLVersion(ctx, vm),
		},
		"postgres": map[string]interface{}{
			"query":   r.wrapPostgresQuery(ctx, vm),
			"version": r.wrapPostgresVersion(ctx, vm),
		},
		"redis": map[string]interface{}{
			"list":            r.wrapRedisList(vm),
			"ping":            r.wrapRedisPing(ctx, vm),
			"info":            r.wrapRedisInfo(ctx, vm),
			"role":            r.wrapRedisRole(ctx, vm),
			"dbsize":          r.wrapRedisDBSize(ctx, vm),
			"scan":            r.wrapRedisScan(ctx, vm),
			"type":            r.wrapRedisType(ctx, vm),
			"ttl":             r.wrapRedisTTL(ctx, vm),
			"exists":          r.wrapRedisExists(ctx, vm),
			"get":             r.wrapRedisGet(ctx, vm),
			"memory_usage":    r.wrapRedisMemoryUsage(ctx, vm),
			"object_encoding": r.wrapRedisObjectEncoding(ctx, vm),
			"slowlog_get":     r.wrapRedisSlowlogGet(ctx, vm),
			"client_list":     r.wrapRedisClientList(ctx, vm),
			"config_get":      r.wrapRedisConfigGet(ctx, vm),
			"hlen":            r.wrapRedisHLen(ctx, vm),
			"llen":            r.wrapRedisLLen(ctx, vm),
			"scard":           r.wrapRedisSCard(ctx, vm),
			"zcard":           r.wrapRedisZCard(ctx, vm),
			"zrange_sample":   r.wrapRedisZRangeSample(ctx, vm),
			"lrange":          r.wrapRedisLRange(ctx, vm),
			"hget":            r.wrapRedisHGet(ctx, vm),
			"hmget":           r.wrapRedisHMGet(ctx, vm),
			"hscan":           r.wrapRedisHScan(ctx, vm),
		},
		"kafka": map[string]interface{}{
			"list":                 r.wrapKafkaList(vm),
			"cluster_info":         r.wrapKafkaAction(ctx, vm, kafkaconn.ActionClusterInfo),
			"brokers":              r.wrapKafkaAction(ctx, vm, kafkaconn.ActionBrokers),
			"topics":               r.wrapKafkaAction(ctx, vm, kafkaconn.ActionTopics),
			"topic_detail":         r.wrapKafkaAction(ctx, vm, kafkaconn.ActionTopicDetail),
			"partition_health":     r.wrapKafkaAction(ctx, vm, kafkaconn.ActionPartitionHealth),
			"consumer_groups":      r.wrapKafkaAction(ctx, vm, kafkaconn.ActionConsumerGroups),
			"consumer_lag":         r.wrapKafkaAction(ctx, vm, kafkaconn.ActionConsumerLag),
			"consumer_lag_summary": r.wrapKafkaAction(ctx, vm, kafkaconn.ActionConsumerLagSummary),
			"topic_offsets":        r.wrapKafkaAction(ctx, vm, kafkaconn.ActionTopicOffsets),
			"broker_config":        r.wrapKafkaAction(ctx, vm, kafkaconn.ActionBrokerConfig),
		},
		"snmp": map[string]interface{}{
			"get":  r.wrapSNMPGet(ctx, vm),
			"walk": r.wrapSNMPWalk(ctx, vm),
			"bulk": r.wrapSNMPBulk(ctx, vm),
		},
		"snmp_devices": map[string]interface{}{
			"list": r.wrapSNMPDevicesList(vm),
		},
		"tcp": map[string]interface{}{
			"exchange": r.wrapTCPExchange(ctx, vm),
		},
		"udp": map[string]interface{}{
			"exchange": r.wrapUDPExchange(ctx, vm),
		},
		"databases": map[string]interface{}{
			"list": r.wrapDatabasesList(vm),
		},
		"hosts": map[string]interface{}{
			"list": r.wrapHostsList(vm),
		},
		"apis": map[string]interface{}{
			"list": r.wrapAPIsList(vm),
		},
		"commands": map[string]interface{}{
			"list": r.wrapCommandsList(vm),
		},
		"command": map[string]interface{}{
			"exec": r.wrapCommandExec(ctx, vm),
		},
		"http": map[string]interface{}{
			"request": r.wrapHTTPRequest(ctx, vm),
			"get":     r.wrapHTTPGet(ctx, vm),
			"post":    r.wrapHTTPPost(ctx, vm),
			"put":     r.wrapHTTPPut(ctx, vm),
			"patch":   r.wrapHTTPPatch(ctx, vm),
			"delete":  r.wrapHTTPDelete(ctx, vm),
		},
	}
}

func (r *Runtime) wrapHostsList(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if r.deps.Cfg == nil {
			panic(appErrorValue(vm, model.NewAppError(model.ErrConnectorError, "config manager not configured")))
		}
		return vm.ToValue(r.deps.Cfg.ListHostSummaries())
	}
}

func (r *Runtime) wrapDatabasesList(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if r.deps.Cfg == nil {
			panic(appErrorValue(vm, model.NewAppError(model.ErrConnectorError, "config manager not configured")))
		}
		return vm.ToValue(r.deps.Cfg.ListDatabaseSummaries())
	}
}

func (r *Runtime) wrapRedisList(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if r.deps.Cfg == nil {
			panic(appErrorValue(vm, model.NewAppError(model.ErrConnectorError, "config manager not configured")))
		}
		return vm.ToValue(r.deps.Cfg.ListRedisSummaries())
	}
}

func (r *Runtime) wrapAPIsList(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if r.deps.APIs != nil {
			return vm.ToValue(r.deps.APIs.ListAPISummaries())
		}
		if r.deps.Cfg == nil {
			panic(appErrorValue(vm, model.NewAppError(model.ErrConnectorError, "config manager not configured")))
		}
		return vm.ToValue(r.deps.Cfg.ListAPISummaries())
	}
}

func (r *Runtime) wrapSSHExec(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if r.deps.SSH == nil {
			panic(appErrorValue(vm, model.NewAppError(model.ErrConnectorError, "ssh connector not configured")))
		}
		var req ssh.ExecRequest
		if err := mapToStruct(call.Argument(0).Export(), &req); err != nil {
			panic(appErrorValue(vm, model.NewAppError(model.ErrInvalidParams, err.Error())))
		}
		res, err := r.deps.SSH.Exec(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapDockerPS(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeDockerArg[docker.PSRequest](vm, call)
		res, err := r.dockerConn(vm).PS(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapDockerLogs(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeDockerArg[docker.LogsRequest](vm, call)
		res, err := r.dockerConn(vm).Logs(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapDockerInfo(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeDockerArg[docker.InfoRequest](vm, call)
		res, err := r.dockerConn(vm).Info(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapDockerStats(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeDockerArg[docker.StatsRequest](vm, call)
		res, err := r.dockerConn(vm).Stats(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapDockerInspect(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeDockerArg[docker.InspectRequest](vm, call)
		res, err := r.dockerConn(vm).Inspect(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapDockerTop(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeDockerArg[docker.TopRequest](vm, call)
		res, err := r.dockerConn(vm).Top(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapDockerHistory(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeDockerArg[docker.HistoryRequest](vm, call)
		res, err := r.dockerConn(vm).History(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) dockerConn(vm *goja.Runtime) *docker.Connector {
	if r.deps.Docker == nil {
		panic(appErrorValue(vm, model.NewAppError(model.ErrConnectorError, "docker connector not configured")))
	}
	return r.deps.Docker
}

func (r *Runtime) wrapMySQLQuery(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[mysql.QueryRequest](vm, call)
		res, err := r.mysqlConn(vm).Query(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapMySQLVersion(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[mysql.VersionRequest](vm, call)
		res, err := r.mysqlConn(vm).Version(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapPostgresQuery(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[postgres.QueryRequest](vm, call)
		res, err := r.postgresConn(vm).Query(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapPostgresVersion(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[postgres.VersionRequest](vm, call)
		res, err := r.postgresConn(vm).Version(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) mysqlConn(vm *goja.Runtime) *mysql.Connector {
	if r.deps.MySQL == nil {
		panic(appErrorValue(vm, model.NewAppError(model.ErrConnectorError, "mysql connector not configured")))
	}
	return r.deps.MySQL
}

func (r *Runtime) postgresConn(vm *goja.Runtime) *postgres.Connector {
	if r.deps.Postgres == nil {
		panic(appErrorValue(vm, model.NewAppError(model.ErrConnectorError, "postgres connector not configured")))
	}
	return r.deps.Postgres
}

func (r *Runtime) redisConn(vm *goja.Runtime) *redisconn.Connector {
	if r.deps.Redis == nil {
		panic(appErrorValue(vm, model.NewAppError(model.ErrConnectorError, "redis connector not configured")))
	}
	return r.deps.Redis
}

func (r *Runtime) wrapRedisPing(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[redisconn.PingRequest](vm, call)
		res, err := r.redisConn(vm).Ping(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapRedisInfo(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[redisconn.InfoRequest](vm, call)
		res, err := r.redisConn(vm).Info(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapRedisRole(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[redisconn.RoleRequest](vm, call)
		res, err := r.redisConn(vm).Role(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapRedisDBSize(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[redisconn.DBSizeRequest](vm, call)
		res, err := r.redisConn(vm).DBSize(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapRedisScan(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[redisconn.ScanRequest](vm, call)
		res, err := r.redisConn(vm).Scan(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapRedisType(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[redisconn.KeyRequest](vm, call)
		res, err := r.redisConn(vm).Type(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapRedisTTL(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[redisconn.KeyRequest](vm, call)
		res, err := r.redisConn(vm).TTL(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapRedisExists(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[redisconn.KeyRequest](vm, call)
		res, err := r.redisConn(vm).Exists(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapRedisGet(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[redisconn.KeyRequest](vm, call)
		res, err := r.redisConn(vm).Get(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapRedisMemoryUsage(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[redisconn.KeyRequest](vm, call)
		res, err := r.redisConn(vm).MemoryUsage(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapRedisObjectEncoding(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[redisconn.KeyRequest](vm, call)
		res, err := r.redisConn(vm).ObjectEncoding(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapRedisSlowlogGet(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[redisconn.SlowlogGetRequest](vm, call)
		res, err := r.redisConn(vm).SlowlogGet(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapRedisClientList(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[redisconn.ClientListRequest](vm, call)
		res, err := r.redisConn(vm).ClientList(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapRedisConfigGet(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[redisconn.ConfigGetRequest](vm, call)
		res, err := r.redisConn(vm).ConfigGet(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapRedisHLen(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[redisconn.KeyRequest](vm, call)
		res, err := r.redisConn(vm).HLen(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapRedisLLen(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[redisconn.KeyRequest](vm, call)
		res, err := r.redisConn(vm).LLen(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapRedisSCard(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[redisconn.KeyRequest](vm, call)
		res, err := r.redisConn(vm).SCard(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapRedisZCard(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[redisconn.KeyRequest](vm, call)
		res, err := r.redisConn(vm).ZCard(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapRedisZRangeSample(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[redisconn.ZRangeSampleRequest](vm, call)
		res, err := r.redisConn(vm).ZRangeSample(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapRedisLRange(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[redisconn.LRangeRequest](vm, call)
		res, err := r.redisConn(vm).LRange(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapRedisHGet(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[redisconn.HGetRequest](vm, call)
		res, err := r.redisConn(vm).HGet(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapRedisHMGet(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[redisconn.HMGetRequest](vm, call)
		res, err := r.redisConn(vm).HMGet(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func (r *Runtime) wrapRedisHScan(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[redisconn.HScanRequest](vm, call)
		res, err := r.redisConn(vm).HScan(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}

func decodeDockerArg[T any](vm *goja.Runtime, call goja.FunctionCall) T {
	return decodeArg[T](vm, call)
}

func decodeArg[T any](vm *goja.Runtime, call goja.FunctionCall) T {
	var req T
	if goja.IsUndefined(call.Argument(0)) || goja.IsNull(call.Argument(0)) {
		return req
	}
	if err := mapToStruct(call.Argument(0).Export(), &req); err != nil {
		panic(appErrorValue(vm, model.NewAppError(model.ErrInvalidParams, err.Error())))
	}
	return req
}

func mapToStruct(in interface{}, out interface{}) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func appErrorValue(vm *goja.Runtime, err error) goja.Value {
	ae, ok := err.(*model.AppError)
	if !ok {
		ae = model.NewAppError(model.ErrConnectorError, err.Error())
	}
	return vm.ToValue(map[string]interface{}{
		"code":    string(ae.Code),
		"message": ae.Message,
	})
}

func wrapGojaError(err error) error {
	if ae, ok := err.(*model.AppError); ok {
		return ae
	}
	if ex, ok := err.(*goja.Exception); ok {
		exported := ex.Value().Export()
		switch t := exported.(type) {
		case *model.AppError:
			return t
		case error:
			if ae, ok := t.(*model.AppError); ok {
				return ae
			}
		case map[string]interface{}:
			if code, _ := t["code"].(string); code != "" {
				msg, _ := t["message"].(string)
				return model.NewAppError(model.ErrorCode(code), msg)
			}
		}
		return model.NewAppError(model.ErrRuntimeError, ex.Error())
	}
	return model.NewAppError(model.ErrRuntimeError, err.Error())
}

func exportValue(v goja.Value) interface{} {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return nil
	}
	exported := v.Export()
	// Re-marshal to plain JSON types for stable encoding.
	b, err := json.Marshal(exported)
	if err != nil {
		return exported
	}
	var out interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		return exported
	}
	return out
}
