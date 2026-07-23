# Plugin 框架

## 理念

**能力即可见 Tool。** MCP Tool 有两类来源：

| 来源 | 形态 | 说明 |
|------|------|------|
| 磁盘 Plugin | `plugin.yml` + `main.js` | 传统路径；未加载则不可见、不可调用 |
| OpenAPI 生成 | `apis.yaml` + 本地 OpenAPI | Go 原生 Tool，**不**要求为每个 Operation 写 `plugin.yml`；见 [apis.md](apis.md) |

二者统一出现在 `tools/list` / `tools/call`；Tool 名全局唯一。

下文描述 **磁盘 Plugin** 契约。

## 目录约定

```
plugins/
├── linux/
│   ├── ls/
│   │   ├── plugin.yml
│   │   └── main.js
│   ├── cat/
│   ├── grep/
│   └── ...
├── docker/
│   ├── ps/
│   └── logs/
├── mysql/
│   └── query/
├── postgres/
│   └── query/
├── redis/
│   ├── ping/
│   ├── scan/
│   └── ...
├── kafka/
│   ├── topics/
│   ├── cluster_info/
│   └── ...
└── apis/
    └── list/          # list_apis
```

规则：

- 每个 Plugin 独占一个目录。
- 目录内必须包含 **`plugin.yml`** 与 **`main.js`**。
- 分类目录（`linux` / `docker` 等）仅用于组织，不参与 Tool 命名；Tool 名以 `plugin.yml` 的 `name` 为准。
- OpenAPI 自动生成的 Tool **不**落在 `plugins/` 下。

## plugin.yml Schema

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | MCP Tool 名，全局唯一，建议 `领域_动作` |
| `version` | string | 是 | 语义化版本，如 `1.0` |
| `description` | string | 是 | 给 Agent 看的工具说明 |
| `type` | string | 是 | 如 `command` / `query`（扩展用） |
| `target.type` | string | 是 | 目标连接器类型：`ssh` / `mysql` / `postgresql` / `docker` / `redis` / `kafka` / `config` / `http` / `command` |
| `input` | map | 是 | 参数定义，键为参数名 |
| `input.<key>.type` | string | 是 | 如 `string` / `number` / `boolean` / `array` / `object` |
| `input.<key>.required` | bool | 否 | 默认 `false` |
| `input.<key>.description` | string | 否 | 写入 MCP inputSchema |
| `runtime` | string | 是 | MVP 固定 `javascript` |
| `timeout` | duration | 否 | 如 `10s`，缺省用全局配置 |

### 示例：linux_ls

```yaml
name: linux_ls
version: "1.0"
description: Linux目录列表
type: command

target:
  type: ssh

input:
  path:
    type: string
    required: true
    description: 要列出的目录路径
  host:
    type: string
    required: true
    description: hosts.yaml 中的主机 name

runtime: javascript
timeout: 10s
```

### 示例：mysql_query

```yaml
name: mysql_query
version: "1.0"
description: 在指定 MySQL 库上执行只读 SQL
type: query

target:
  type: mysql

input:
  database:
    type: string
    required: true
    description: databases.yaml 中的数据库 name
  sql:
    type: string
    required: true
    description: SQL 语句

runtime: javascript
timeout: 30s
```

## main.js 约定

必须导出（或定义）全局函数：

```javascript
function execute(ctx) {
  // 使用 ctx.params / ctx.ssh / ...
  return /* 结果对象，通常由 Connector 返回 */;
}
```

- 入口函数名固定为 **`execute`**。
- 参数仅一个：`ctx`（由 Go 注入）。
- 返回值会被包装进统一 `{ success, data }`；失败时为 `{ success: false, error: { code, message } }`。
- JS 只做编排；禁止在 JS 内直接建原始 socket / SSH / DB 连接；网络经 `ctx.*`（含 `ctx.tcp` / `ctx.udp`）。

完整 `ctx` API 见 [runtime.md](runtime.md)。

### linux_ls 示例

```javascript
function execute(ctx) {
  return ctx.ssh.exec({
    host: ctx.params.host,
    command: "ls",
    args: ["-al", ctx.params.path]
  });
}
```

### mysql_query 示例

```javascript
function execute(ctx) {
  return ctx.mysql.query({
    database: ctx.params.database,
    sql: ctx.params.sql
  });
}
```

### 示例：host_ping（Command Connector）

用本机常见 CLI（如 `ping`，须在 `commands.yaml` 白名单）扩展尚无专用 Connector 的能力。`type: command` 是元数据标签；`target.type: command` 表示本机 Command Connector。

```yaml
name: host_ping
version: "1.0"
description: 在本机执行 ping（只读探测）
type: command

target:
  type: command

input:
  host:
    type: string
    required: true
    description: 目标主机名或 IP
  count:
    type: number
    required: false
    description: 发包次数（-c）；缺省 3

runtime: javascript
timeout: 15s
```

```javascript
function execute(ctx) {
  var count = ctx.params.count != null ? ctx.params.count : 3;
  return ctx.command.exec({
    command: "ping",
    args: ["-c", String(count), ctx.params.host]
  });
}
```

扩展模式：Plugin 固定白名单逻辑名与参数形态；MCP 只传业务参数拼入 `args`。勿做通用「任意命令 / 任意 argv」Tool。`commands.yaml` 的 `path` 用数组适配多 OS（加载时取第一个可用绝对路径）。需要时也可按同样方式包装本机其他 CLI（如 `kubectl`）。

## MCP Tool 自动注册

磁盘 Plugin 加载成功后：

1. 以 `name` 注册 MCP Tool。
2. `description` → Tool description。
3. `input` → JSON Schema 形式的 `inputSchema`（`required` 数组由各字段 `required: true` 聚合）。

Agent 调用示例：

```json
{
  "path": "/var/log",
  "host": "prod-web-01"
}
```

对应 Tool：`linux_ls`。

OpenAPI 生成 Tool 的命名与 schema 见 [apis.md](apis.md)；与磁盘 Plugin 一并出现在 `tools/list`。

## 加载与校验流程

```mermaid
flowchart LR
  Scan[扫描 plugins] --> ReadYML[解析 plugin.yml]
  ReadYML --> Validate[校验必填字段]
  Validate --> LoadJS[加载 main.js]
  LoadJS --> Register[注册 MCP Tool]
  Register --> Ready[可被 tools/call]
```

失败策略：

- 单个 Plugin 损坏：记录错误并跳过，不影响其他 Plugin 加载。
- `POST /api/reload`：成功时原子替换 Tool 表（含 OpenAPI Tools）；整体失败（0 个磁盘 Plugin 成功、或与 OpenAPI Tool 重名冲突等）时保留上一份可用集。

## Plugin 清单

### 元数据

| name | 说明 | target |
|------|------|--------|
| `list_hosts` | 列出可用 SSH 主机（无密钥） | config |
| `list_databases` | 列出可用数据库（无密码） | config |
| `list_redis` | 列出可用 Redis 实例（无密码） | config |
| `list_apis` | 列出可用 HTTP API 服务（无 headers 原文） | config |
| `list_commands` | 列出本机命令白名单（`commands.yaml`） | config |
| `list_snmp_devices` | 列出 SNMP 设备（`snmp.yaml`，支持 labels 过滤） | config |
| `ops_mcp_health` | 探测本机 ops-mcp `GET /health`（`ctx.http` URL 模式示例） | http |
| `host_ping` | 本机 `ping`（Command Connector） | command |
| `host_traceroute` | 本机 `traceroute` | command |
| `host_dig` | 本机 `dig` DNS 查询 | command |
| `host_nslookup` | 本机 `nslookup` DNS 查询 | command |

### Linux

均需必填参数 `host`（经 SSH 执行）。

| name | 说明 | target |
|------|------|--------|
| `linux_ls` | 目录列表 | ssh |
| `linux_cat` | 读文件 | ssh |
| `linux_grep` | 文本检索 | ssh |
| `linux_tail` | 文件尾部 | ssh |
| `linux_du` | 目录占用 | ssh |
| `linux_df` | 文件系统空间 | ssh |
| `linux_lsblk` | 块设备 | ssh |
| `linux_journalctl` | systemd 日志 | ssh |
| `linux_dmesg` | 内核日志（dmesg） | ssh |
| `linux_systemctl_status` | systemd 状态 | ssh |
| `linux_systemctl_cat` | 单元文件内容 | ssh |
| `linux_systemctl_list_units` | 列出单元 | ssh |
| `linux_systemctl_failed` | 失败单元 | ssh |
| `linux_top` | 进程概览 | ssh |
| `linux_ps` | 进程列表 | ssh |
| `linux_free` | 内存 | ssh |
| `linux_lscpu` | CPU 信息 | ssh |
| `linux_dmidecode` | DMI/SMBIOS | ssh |
| `linux_uptime` | 运行时间 / 负载 | ssh |
| `linux_uname` | 内核标识 | ssh |
| `linux_hostnamectl` | 主机名信息 | ssh |
| `linux_timedatectl` | 时间 / 时区 | ssh |
| `linux_date` | 当前时间 | ssh |
| `linux_ip_addr` | 网卡地址 | ssh |
| `linux_ip_route` | 路由表 | ssh |
| `linux_ss` | 套接字（ss） | ssh |
| `linux_netstat` | 网络连接（ss -tunlp） | ssh |
| `linux_lsof` | 打开文件 / 端口 | ssh |
| `linux_nslookup` | DNS（nslookup） | ssh |
| `linux_dig` | DNS（dig） | ssh |

### Docker

均需必填参数 `host`（经 SSH 在远端执行，无本地 Docker 回退）。

| name | 说明 | target |
|------|------|--------|
| `docker_ps` | 容器列表 | docker |
| `docker_logs` | 容器日志 | docker |
| `docker_info` | Docker 守护进程信息 | docker |
| `docker_stats` | 容器资源占用 | docker |
| `docker_inspect` | 容器 / 镜像详情 | docker |
| `docker_top` | 容器内进程 | docker |
| `docker_history` | 镜像层历史 | docker |

### Database

| name | 说明 | target |
|------|------|--------|
| `mysql_query` | MySQL 只读 SELECT（parser 校验 + limit） | mysql |
| `mysql_version` | MySQL 服务器版本 | mysql |
| `postgres_query` | PostgreSQL 只读 SELECT（parser 校验 + limit） | postgresql |
| `postgres_version` | PostgreSQL 服务器版本 | postgresql |

### Redis

| name | 说明 | target |
|------|------|--------|
| `redis_ping` | PING；可选 `db`（缺省 0） | redis |
| `redis_info` | INFO（可选 section / `db`） | redis |
| `redis_role` | ROLE | redis |
| `redis_dbsize` | DBSIZE | redis |
| `redis_scan` | SCAN（**必须** `limit`） | redis |
| `redis_type` | TYPE | redis |
| `redis_ttl` | TTL | redis |
| `redis_exists` | EXISTS | redis |
| `redis_get` | GET（string；不存在时 value 为 null） | redis |
| `redis_memory_usage` | MEMORY USAGE | redis |
| `redis_object_encoding` | OBJECT ENCODING | redis |
| `redis_slowlog_get` | SLOWLOG GET（**必须** `count`） | redis |
| `redis_client_list` | CLIENT LIST（**必须** `limit`） | redis |
| `redis_config_get` | CONFIG GET（只读） | redis |
| `redis_hlen` / `redis_llen` / `redis_scard` / `redis_zcard` | 集合基数 | redis |
| `redis_zrange_sample` | ZRANGE 采样（**必须** `limit`） | redis |
| `redis_lrange` | LRANGE（**必须** `limit`；可选 `start`） | redis |
| `redis_hget` | HGET（字段不存在时 value 为 null） | redis |
| `redis_hmget` | HMGET（`fields` 非空，数量受 limit 约束） | redis |
| `redis_hscan` | HSCAN（**必须** `limit`） | redis |

### Kafka

| name | 说明 | target |
|------|------|--------|
| `list_kafka` | 列出 `kafka.yaml` 实例元数据（不含密码） | config |
| `kafka_cluster_info` | 集群信息（cluster id / controller / brokers） | kafka |
| `kafka_brokers` | broker 列表 | kafka |
| `kafka_topics` | topic 列表（可选 prefix/limit） | kafka |
| `kafka_topic_detail` | topic 分区 / 副本 / ISR | kafka |
| `kafka_partition_health` | under-replicated / offline / no leader | kafka |
| `kafka_consumer_groups` | consumer group 列表 | kafka |
| `kafka_consumer_lag` | 指定 group 分区 lag | kafka |
| `kafka_consumer_lag_summary` | 按 group 汇总 lag | kafka |
| `kafka_topic_offsets` | earliest / latest offset | kafka |
| `kafka_broker_config` | broker / 集群配置（敏感值脱敏） | kafka |

### SNMP

均需必填参数 `device`（`snmp.yaml` 中的设备 name），清单类除外。

| name | 说明 | target |
|------|------|--------|
| `list_snmp_devices` | 列出设备摘要（labels / limit / offset） | config |
| `snmp_get` | 通用 GET（`oids` 数组） | snmp |
| `snmp_walk` | 通用 WALK | snmp |
| `snmp_bulk` | 通用 BULKWALK | snmp |
| `snmp_sysinfo` | sysDescr / sysName / sysUpTime 等 | snmp |
| `snmp_interfaces` | ifTable 子集（端口状态等） | snmp |

### TCP / UDP

原始字节交换。Connector **无**资源 YAML；`ip`/`port` 由 Plugin 传入。

| name | 说明 | target |
|------|------|--------|
| `tcp_exchange` | 调试冒烟：透传 `ip`/`port`/`data` → `ctx.tcp.exchange`（**非推荐**业务模式） | tcp |
| `udp_exchange` | 调试冒烟：透传 `ip`/`port`/`data` → `ctx.udp.exchange`（**非推荐**业务模式） | udp |

业务设备 Plugin（推荐）：在 `main.js` 内**固定或校验** `ip`/`port`，MCP 只暴露业务字段；载荷用 hex 或字节数组拼帧。本地联调：`make net-up`（见 [deploy/dev-net/README.md](../deploy/dev-net/README.md)）。

### HTTP API（OpenAPI 生成）

由 `apis.yaml` 自动生成，**不是**磁盘 Plugin。命名形如 `{prefix}{operationId}`（例：`cmdb_getHostById`）。清单类磁盘 Plugin 见上文 `list_apis`。完整契约见 [apis.md](apis.md)。

## 作者检查清单

- [ ] `name` 全局唯一且稳定（Agent 会记住 Tool 名；亦勿与 OpenAPI `{prefix}{operationId}` 冲突）
- [ ] `description` 写清用途与限制（如只读）
- [ ] `input` 字段完整，`required` 正确
- [ ] `timeout` 符合命令预期耗时
- [ ] `main.js` 仅通过 `ctx.*` 调用 Connector
- [ ] 资源名（host / database / redis / kafka / snmp device / API 服务 name）来自配置，不硬编码 IP（SSH/DB/SNMP 等）
- [ ] 使用 `ctx.tcp` / `ctx.udp` 时：在 Plugin 内限制 `ip`/`port`；**不建议**把 MCP 入参地址原样透传
