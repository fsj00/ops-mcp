# Goja Runtime

## 角色

Goja 是 ops-mcp 的 JavaScript 执行引擎。每个 Plugin 的 `main.js` 在受控环境中运行：

- Go 注入 `ctx` 对象。
- JS 调用 `ctx` 上的 Connector 桥接方法。
- 遵守 `plugin.yml` 的 `timeout`。

JS **不负责**建连、认证、协议编解码；这些由 Go Connector 完成。

## 入口

```javascript
function execute(ctx) {
  // ...
  return result;
}
```

- 函数名固定：`execute`
- 入参：`ctx`
- 返回：任意可 JSON 序列化的值；推荐直接返回 Connector 的结果对象

## ctx 对象

### `ctx.params`

Tool 调用传入的参数对象，键与 `plugin.yml` 的 `input` 一致。

```javascript
const path = ctx.params.path;
const host = ctx.params.host;
```

调用前 Go 已按 schema 校验；缺失必填项不会进入 `execute`。

---

### `ctx.hosts`

读取已加载的主机配置（不含密码 / 私钥）。

#### `ctx.hosts.list()`

无参数。返回主机摘要数组：

```json
[
  {
    "name": "dev-ssh-111",
    "description": "...",
    "address": { "host": "192.168.44.111", "port": 22 },
    "auth_type": "password",
    "username": "root"
  }
]
```

---

### `ctx.databases`

读取已加载的数据库配置（不含密码）。

#### `ctx.databases.list()`

无参数。返回数据库摘要数组（字段同 `GET /api/databases` 的 `databases`）。

---

### `ctx.apis`

读取已加载的 HTTP API 服务配置（**不含 headers 原文**）。OpenAPI 生成的 MCP Tool **不经** Goja；本 API 仅供磁盘 Plugin（如 `list_apis`）使用。完整契约见 [apis.md](apis.md)。

#### `ctx.apis.list()`

无参数。返回 API 服务摘要数组（字段同 `GET /api/apis` 的 `apis`）。

---

### `ctx.http`

出站 HTTP（经 Go HTTP Connector）。支持两种寻址（**互斥**）：

1. **API 模式**：`api` = `apis.yaml` 中的服务 `name`，再配 `path`（相对 `base_url`）。配置 headers / timeout / verify_tls 作默认；入参 `headers` 可覆盖同名键。
2. **URL 模式**：`url` = 绝对 `http(s)` 地址；不查 `apis.yaml`。可选 `timeout` / `verify_tls`。

| 方法 | 说明 |
|------|------|
| `ctx.http.request(options)` | 通用请求；`method` 必填 |
| `ctx.http.get(options)` | GET |
| `ctx.http.post(options)` | POST |
| `ctx.http.put(options)` | PUT |
| `ctx.http.patch(options)` | PATCH |
| `ctx.http.delete(options)` | DELETE |

**options：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `api` | string | 与 `url` 二选一 |
| `url` | string | 与 `api` 二选一；绝对 URL |
| `method` | string | 仅 `request` 需要 |
| `path` | string | API 模式必填 |
| `query` | object | 查询参数 |
| `headers` | object | 请求头 |
| `body` | any | JSON body（非空时自动 `Content-Type: application/json`） |
| `timeout` | string | 如 `5s`；缺省用配置或全局默认 |
| `verify_tls` | boolean | URL 模式默认 `true`；API 模式默认跟 `apis.yaml` |

**返回值：** `{ status_code, headers, body }`（`body` 尽量解析为 JSON）。

示例（URL 模式，见 Plugin `ops_mcp_health`）：

```javascript
function execute(ctx) {
  return ctx.http.get({
    url: (ctx.params.base_url || "http://127.0.0.1:20267") + "/health",
    timeout: "5s"
  });
}
```

勿把真实 Token 写进可提交的 `main.js`；优先 `apis.yaml` + `${ENV}`，或经 `ctx.params` 传入。

---

### `ctx.ssh`

SSH 远程执行桥接。

#### `ctx.ssh.exec(options)`

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `host` | string | 是 | `hosts.yaml` 中的主机 `name` |
| `command` | string | 是 | 可执行文件名或命令 |
| `args` | string[] | 否 | 参数列表 |
| `workdir` | string | 否 | 远端工作目录 |
| `env` | object | 否 | 额外环境变量 |

**返回值：**

```json
{
  "stdout": "...",
  "stderr": "...",
  "exit_code": 0
}
```

**示例：**

```javascript
function execute(ctx) {
  return ctx.ssh.exec({
    host: ctx.params.host,
    command: "ls",
    args: ["-al", ctx.params.path]
  });
}
```

---

### `ctx.commands`

读取已加载的本机命令白名单摘要（`commands.yaml`）。

#### `ctx.commands.list()`

无参数。返回命令摘要数组（字段同 `GET /api/commands` 的 `commands`）：

```json
[
  {
    "name": "ping",
    "description": "本机 ping",
    "path": "/sbin/ping"
  }
]
```

---

### `ctx.command`

本机进程执行桥接（Command Connector / Local Process Executor）。仅执行 `commands.yaml` 白名单中的逻辑名；**无 shell**。

#### `ctx.command.exec(options)`

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `command` | string | 是 | `commands.yaml` 中的逻辑 `name`（不是任意路径） |
| `args` | string[] | 否 | 参数列表，原样传给进程 |
| `workdir` | string | 否 | 本机工作目录 |
| `env` | object | 否 | 额外环境变量（合并进进程环境） |

**返回值：** 与 `ctx.ssh.exec` 相同：`{ stdout, stderr, exit_code }`。

**示例（ping，Plugin 固定参数形态）：**

```javascript
function execute(ctx) {
  var count = ctx.params.count != null ? ctx.params.count : 3;
  return ctx.command.exec({
    command: "ping",
    args: ["-c", String(count), ctx.params.host]
  });
}
```

未登记的 `command` → `INVALID_PARAMS`；进程无法启动 → `CONNECTOR_ERROR`。

---

### `ctx.mysql`

#### `ctx.mysql.query(options)`

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `database` | string | 是 | `databases.yaml` 中 `type: mysql` 的 `name` |
| `sql` | string | 是 | 仅允许单条 `SELECT` / `UNION`（`github.com/pingcap/parser` 语义校验） |
| `args` | array | 否 | 占位符参数（若驱动支持） |

**返回值：**

```json
{
  "columns": ["id", "name"],
  "rows": [
    [1, "alice"],
    [2, "bob"]
  ],
  "row_count": 2
}
```

非 SELECT（如 `INSERT`/`UPDATE`/`DELETE`）返回 `INVALID_PARAMS`。执行前按 `databases.yaml` 的 `limit`（默认 1000）包裹外层 `LIMIT`；日志会打印实际执行的 SQL。

**示例：**

```javascript
function execute(ctx) {
  return ctx.mysql.query({
    database: ctx.params.database,
    sql: ctx.params.sql
  });
}
```

#### `ctx.mysql.version(options)`

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `database` | string | 是 | `databases.yaml` 中 `type: mysql` 的 `name` |

**返回值：** `{ "version": "..." }`

---

### `ctx.postgres`

#### `ctx.postgres.query(options)`

字段与返回值形态同 `ctx.mysql.query`，`database` 指向 `type: postgresql` 的配置项。

```javascript
function execute(ctx) {
  return ctx.postgres.query({
    database: ctx.params.database,
    sql: ctx.params.sql
  });
}
```

#### `ctx.postgres.version(options)`

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `database` | string | 是 | `databases.yaml` 中 `type: postgresql` 的 `name` |

**返回值：** `{ "version": "..." }`

---

### `ctx.docker`

所有 Docker API 均通过 SSH 在远端执行 `docker` CLI；**`host` 必填**（`hosts.yaml` 中的 name）。不支持默认本地 Docker。

#### `ctx.docker.ps(options)`

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `host` | string | 是 | `hosts.yaml` 中的主机 `name` |
| `all` | boolean | 否 | 是否包含已停止容器，默认 `false` |

**返回值：** 容器摘要数组（id、name、image、status、ports 等）。

#### `ctx.docker.logs(options)`

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `host` | string | 是 | `hosts.yaml` 中的主机 `name` |
| `container` | string | 是 | 容器名或 ID |
| `tail` | number / string | 否 | 尾部行数 |
| `since` | string | 否 | 时间过滤 |
| `timestamps` | boolean | 否 | 是否带时间戳 |

**返回值：**

```json
{
  "logs": "..."
}
```

#### `ctx.docker.info(options)`

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `host` | string | 是 | `hosts.yaml` 中的主机 `name` |

**返回值：** `{ format, info? , raw? }`（优先 JSON）。

#### `ctx.docker.stats(options)`

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `host` | string | 是 | `hosts.yaml` 中的主机 `name` |
| `container` | string | 否 | 指定容器；省略则全部运行中容器 |

底层固定 `--no-stream`，避免挂起。

#### `ctx.docker.inspect(options)`

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `host` | string | 是 | `hosts.yaml` 中的主机 `name` |
| `target` | string | 是 | 容器或镜像名 / ID |

**返回值：** `{ objects: [...] }`。

#### `ctx.docker.top(options)`

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `host` | string | 是 | `hosts.yaml` 中的主机 `name` |
| `container` | string | 是 | 容器名或 ID |
| `ps_args` | string | 否 | 传给底层 `ps` 的额外参数 |

**返回值：** `{ output: "..." }`。

#### `ctx.docker.history(options)`

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `host` | string | 是 | `hosts.yaml` 中的主机 `name` |
| `image` | string | 是 | 镜像名或 ID |

**返回值：** 镜像层历史数组。

---

### `ctx.redis`

Redis 只读运维查询。资源名来自 `redis.yaml` 的 `name`。支持无密码、password、Redis 6+ ACL，以及 TLS / mTLS（见 `connection.tls`）。

大数据量命令必须传 `limit`/`count`（且 > 0），并由配置上限截断（缺省 1000）。

#### `ctx.redis.list()`

无参数。返回已配置 Redis 实例摘要数组（不含密码；字段同 `GET /api/redis` 的 `redis`）。

#### 通用字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `redis` | string | 是 | `redis.yaml` 中的实例 `name` |
| `db` | number | 否 | 逻辑库编号（SELECT），缺省 **0** |

#### 方法一览

| JS | 说明 | 额外必填 |
|----|------|----------|
| `ctx.redis.list` | 列出已配置实例（无密码） | —（无 `db`） |
| `ctx.redis.ping` | PING | — |
| `ctx.redis.info` | INFO；可选 `section` | — |
| `ctx.redis.role` | ROLE | — |
| `ctx.redis.dbsize` | DBSIZE | — |
| `ctx.redis.scan` | SCAN（禁止 KEYS） | `limit`；可选 `cursor`/`match` |
| `ctx.redis.type` | TYPE | `key` |
| `ctx.redis.ttl` | TTL（秒） | `key` |
| `ctx.redis.exists` | EXISTS | `key` |
| `ctx.redis.get` | GET（string；缺失时 `value` 为 null） | `key` |
| `ctx.redis.memory_usage` | MEMORY USAGE | `key` |
| `ctx.redis.object_encoding` | OBJECT ENCODING | `key` |
| `ctx.redis.slowlog_get` | SLOWLOG GET | `count` |
| `ctx.redis.client_list` | CLIENT LIST（截断） | `limit` |
| `ctx.redis.config_get` | CONFIG GET（无 SET） | `pattern` |
| `ctx.redis.hlen` / `llen` / `scard` / `zcard` | 基数 | `key` |
| `ctx.redis.zrange_sample` | ZRANGE 采样 | `key`、`limit` |
| `ctx.redis.lrange` | LRANGE（从 start 起采样） | `key`、`limit`；可选 `start` |
| `ctx.redis.hget` | HGET（缺失时 `value` 为 null） | `key`、`field` |
| `ctx.redis.hmget` | HMGET | `key`、`fields`（非空，数量受 limit 约束） |
| `ctx.redis.hscan` | HSCAN | `key`、`limit`；可选 `cursor`/`match` |

**示例（scan）：**

```javascript
function execute(ctx) {
  return ctx.redis.scan({
    redis: ctx.params.redis,
    cursor: ctx.params.cursor || 0,
    match: ctx.params.match,
    limit: ctx.params.limit
  });
}
```

缺少 `limit`/`count` 或 ≤0 → `INVALID_PARAMS`。

---

### `ctx.kafka`

Kafka 只读 Admin 查询。资源名来自 `kafka.yaml` 的 `name`。支持可选 SASL（plain / scram-sha-256 / scram-sha-512）与 TLS / mTLS。

Go Connector 以 `Execute(action, params)` 分发；JS 侧使用下列类型化方法（**不要**调用统一 `execute`）。

#### `ctx.kafka.list()`

无参数。返回已配置 Kafka 实例摘要数组（不含密码；字段同 `GET /api/kafka` 的 `kafka`）。

#### 通用字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `kafka` | string | 是 | `kafka.yaml` 中的实例 `name` |

#### 方法一览

| JS | 说明 | 额外参数 |
|----|------|----------|
| `ctx.kafka.list` | 列出已配置实例（无密码） | — |
| `ctx.kafka.cluster_info` | 集群信息 | — |
| `ctx.kafka.brokers` | broker 列表 | — |
| `ctx.kafka.topics` | topic 列表 | 可选 `prefix` / `limit` / `include_internal` |
| `ctx.kafka.topic_detail` | topic 分区详情 | `topic` |
| `ctx.kafka.partition_health` | 分区健康 | 可选 `topic` |
| `ctx.kafka.consumer_groups` | consumer group 列表 | 可选 `limit` |
| `ctx.kafka.consumer_lag` | 指定 group lag | `group`；可选 `topic` |
| `ctx.kafka.consumer_lag_summary` | lag 汇总 | 可选 `group` / `limit` |
| `ctx.kafka.topic_offsets` | earliest/latest offset | `topic` |
| `ctx.kafka.broker_config` | broker 配置（敏感值脱敏） | 可选 `broker_id` / `prefix` |

**示例：**

```javascript
function execute(ctx) {
  return ctx.kafka.consumer_lag({
    kafka: ctx.params.kafka,
    group: ctx.params.group,
    topic: ctx.params.topic
  });
}
```

---

### `ctx.snmp` / `ctx.snmp_devices`

SNMP 只读查询。资源名来自 `snmp.yaml` 的 `devices[].name`。支持 SNMPv2c / v3；凭据可为 `credential` 引用或设备内联 `auth`（见 [configuration.md](configuration.md)）。

#### `ctx.snmp_devices.list({ labels?, limit?, offset? })`

返回设备摘要数组（不含密钥）。`labels` 为精确匹配过滤；`limit` 缺省 100。

#### 查询方法

| 方法 | 说明 | 额外参数 |
|------|------|----------|
| `ctx.snmp.get` | SNMP GET | `device`、`oids`（数组） |
| `ctx.snmp.walk` | SNMP WALK | `device`、`oid`；可选 `max_oids` |
| `ctx.snmp.bulk` | SNMP BULKWALK | `device`、`oid`；可选 `max_oids` / `max_repetitions` |

统一结果：`{ device, vars: [{ oid, type, value }], truncated, count }`。

**示例：**

```javascript
function execute(ctx) {
  return ctx.snmp.get({
    device: ctx.params.device,
    oids: ["1.3.6.1.2.1.1.5.0"]
  });
}
```

---

### `ctx.tcp` / `ctx.udp`

原始字节请求-响应。每次调用必填 `ip`、`port`、`data`；**无**资源 YAML。见 [connector.md](connector.md) TCP/UDP。

| 方法 | 说明 |
|------|------|
| `ctx.tcp.exchange` | TCP dial → write → read → close |
| `ctx.udp.exchange` | 发一个 UDP datagram 并收一个响应 |

**参数：** `ip`、`port`、`data`（hex 字符串或 `0..255` 数组）；可选 `timeout`（如 `"2s"`）、`max_response_bytes`。  
**结果：** `{ ip, port, protocol, request_bytes, response_bytes, hex, bytes, rtt_ms }`。

缺省：`timeout=5s`，`max_response_bytes=65536`。

**建议用法（业务 Plugin 限制目标）：**

```javascript
function execute(ctx) {
  // 建议：在 Plugin 内固定或校验 ip/port，勿把任意地址交给 Agent
  return ctx.udp.exchange({
    ip: "192.168.1.10",
    port: 10086,
    data: buildFrame(ctx.params),
    timeout: "2s"
  });
}
```

**不建议：** 将 `ctx.params.ip` / `ctx.params.port` 原样透传（调试冒烟 Plugin `tcp_exchange` / `udp_exchange` 除外）。本地联调：`make net-up`（TCP `127.0.0.1:19090`，UDP `127.0.0.1:19091`）。

---

## 超时与取消

1. Executor 以 `plugin.yml.timeout`（或全局默认）启动计时。
2. 超时后中断 Goja 执行，并尽量取消进行中的 Connector 调用。
3. 向调用方返回 `PLUGIN_TIMEOUT`（见 [api.md](api.md)）。

## 错误传播

| 来源 | 行为 |
|------|------|
| JS `throw` | 捕获为 `RUNTIME_ERROR` |
| Connector 失败 | 抛出或返回错误，映射为 `CONNECTOR_ERROR` |
| 非法参数（进 execute 前） | `INVALID_PARAMS`，不进入 JS |

推荐在 JS 中直接 `return ctx.*.*(...)`，由 Go 统一包装；避免在 JS 内吞掉错误后返回模糊字符串。

## 安全约束

- 无文件系统任意读写 API（除经 Connector 的受控操作）。
- 无 `require` / 网络原始 socket（TCP/UDP 仅经 `ctx.tcp` / `ctx.udp`）。
- 不加载第三方 npm 包；仅标准语言能力 + 注入的 `ctx`。
- **每次执行新建 Goja VM**，不共享可变全局状态。
- TCP/UDP：**建议**在 Plugin 内限制 `ip`/`port`；**不建议**把 MCP 入参地址原样透传。

## 与 Go 侧的对应关系

| JS API | Go 组件 |
|--------|---------|
| `ctx.hosts.list` | ConfigManager 主机摘要 |
| `ctx.databases.list` | ConfigManager 数据库摘要 |
| `ctx.apis.list` | ConfigManager API 服务摘要 |
| `ctx.commands.list` | ConfigManager 本机命令白名单摘要 |
| `ctx.http.*` | `internal/connector/http` |
| `ctx.ssh.exec` | `internal/connector/ssh` |
| `ctx.command.exec` | `internal/connector/command` |
| `ctx.mysql.query` / `version` | `internal/connector/mysql` |
| `ctx.postgres.query` / `version` | `internal/connector/postgres` |
| `ctx.docker.ps` / `logs` / `info` / `stats` / `inspect` / `top` / `history` | `internal/connector/docker` |
| `ctx.redis.*` | `internal/connector/redis` |
| `ctx.kafka.*` | `internal/connector/kafka`（Go `Execute` → 类型化桥接） |
| `ctx.snmp.*` / `ctx.snmp_devices.list` | `internal/connector/snmp` + ConfigManager |
| `ctx.tcp.exchange` | `internal/connector/tcp` |
| `ctx.udp.exchange` | `internal/connector/udp` |
| `ctx.params` | Executor 校验后的参数 map |

OpenAPI 生成 Tool 由 MCP 层直接调用 `internal/connector/http`，不经过本 Runtime；磁盘 Plugin 可通过 `ctx.http.*` 调用同一 Connector。

桥接实现位于 `internal/runtime/goja.go`：将 Go 函数导出为 Goja 可调用对象。
