# 开发路线图

ops-mcp 按阶段交付。**当前完成：Phase 0–9（含 Phase 9 TCP/UDP Connector）。** 每一阶段依赖上一阶段可运行基线。

```mermaid
flowchart LR
  P0[Docs] --> P1[Phase1 Core]
  P1 --> P2[Phase2 SSH]
  P2 --> P3[Phase3 Docker]
  P3 --> P4[Phase4 Database]
  P4 --> P5[Phase5 Redis Kafka]
  P5 --> P6[Phase6 HTTP API]
  P6 --> P7[Phase7 Command]
  P7 --> P8[Phase8 SNMP]
  P8 --> P9[Phase9 TCP UDP]
```

---

## Phase 0：文档（已完成）

### 交付物

- 根 [README.md](../README.md)
- [architecture.md](architecture.md) / [api.md](api.md) / [plugin.md](plugin.md) / [runtime.md](runtime.md) / [connector.md](connector.md) / [configuration.md](configuration.md) / 本文档

### 验收

- [x] 架构、API、Plugin、配置契约齐全，可指导编码

---

## Phase 1：核心框架 + 首批 Linux Plugin（已完成）

### 目标

可编译、可运行的 MCP Remote Server；Plugin 可加载；Goja 可执行；至少 4 个 Linux Plugin 可用。

### 交付物

- Go module（Go ≥ 1.24，`github.com/fsj00/ops-mcp`）
- `cmd/server` + Cobra 启动
- Gin：`POST /mcp`、`GET /api/plugins`、`GET /api/tools`、`POST /api/reload`、`GET /health`
- MCP Server：`initialize` / `tools/list` / `tools/call`（Bearer / API Key 鉴权）
- Plugin Loader / Manager + YAML 解析
- Goja Runtime + Executor（参数校验、超时）
- Plugin：`linux_ls` / `linux_cat` / `linux_grep` / `linux_tail`（后续阶段已扩展为完整 Linux 集）
- Zap 日志与 `config/ops-mcp.yaml`
- Makefile / Dockerfile / `deploy/` systemd 安装

### 验收

- [x] `go build ./...` 成功
- [x] 进程启动后 `tools/list` 返回上述 4 个 Tool（及后续扩展 Tool）
- [x] `tools/call` 调用 `linux_ls` 返回统一 `{success, data}` 形态
- [x] `GET /api/plugins` 与 `POST /api/reload` 可用

### 下一阶段依赖

稳定的 Plugin 加载与 Goja 桥接接口，便于挂接真实 SSH Connector。

---

## Phase 2：SSH Connector（已完成）

### 目标

通过 `hosts.yaml` 管理主机，Linux Plugin 经 SSH 远程执行。

### 交付物

- `internal/connector/ssh`（按调用建连）
- `config/hosts.yaml`（及 example）
- ConfigManager：`GetHost` / `ListHostSummaries`
- Runtime：`ctx.ssh.exec`、`ctx.hosts.list`
- Plugin：`list_hosts` 与完整 Linux 运维集（见 [plugin.md](plugin.md)）
- 认证：`password` / `private_key`

### 验收

- [x] 配置两台示例主机（密钥占位）可解析
- [x] `linux_ls` 对真实（或测试容器）SSH 主机执行成功
- [x] 错误主机名 / 认证失败返回 `CONNECTOR_ERROR`，日志无明文密码

### 下一阶段依赖

SSH 路径稳定后，Docker 经 SSH 在远端执行 `docker` CLI。

---

## Phase 3：Docker Connector（已完成）

### 目标

查询容器状态与日志。

### 交付物

- `internal/connector/docker`
- Runtime：`ctx.docker.ps` / `logs` / `info` / `stats` / `inspect` / `top` / `history`
- Plugin：`docker_ps`、`docker_logs`、`docker_info`、`docker_stats`、`docker_inspect`、`docker_top`、`docker_history`
- 一律经 SSH（`host` 必填），无本地 Docker 回退
- Dockerfile + `make docker` / `make docker-run`；发行包与 systemd 安装（`make tar` + `deploy/`）

### 验收

- [x] `docker_ps` 返回容器列表
- [x] `docker_logs` 按 container / tail 返回日志
- [x] `docker_info` / `docker_stats` / `docker_inspect` / `docker_top` / `docker_history` 可用
- [x] `host` 必填，缺省返回参数错误
- [x] `tools/list` 含上述 Tool

### 下一阶段依赖

Connector 模式复用到数据库驱动封装。

---

## Phase 4：Database Connector（已完成）

### 目标

MySQL / PostgreSQL 只读查询能力。

### 交付物

- `internal/connector/mysql`、`internal/connector/postgres`、`sqlguard`、`dbutil`
- `config/databases.yaml`（及 example）
- Runtime：`ctx.mysql.query` / `version`、`ctx.postgres.query` / `version`、`ctx.databases.list`
- Plugin：`mysql_query`、`mysql_version`、`postgres_query`、`postgres_version`、`list_databases`
- `readonly: true`；经 `pingcap/parser` 仅允许 SELECT；`limit` 强制截断

### 验收

- [x] 对测试库执行 `SELECT` 返回 `columns` / `rows`
- [x] 经 `pingcap/parser` 拒绝 `INSERT`/`UPDATE`/`DELETE` 等非 SELECT
- [x] `databases.yaml` 的 `limit`（默认 1000）强制截断结果集
- [x] `postgres_version` / `mysql_version` 可查询版本
- [x] 错误库名返回明确错误；日志打印执行 SQL

### 下一阶段依赖

数据面 Connector 模式可推广到 Redis / Kafka。

---

## Phase 5：Redis / Kafka（已完成）

### 目标

扩展缓存与消息队列只读/运维查询类能力。

### 交付物

- Redis Connector + 对应 Plugin（`redis_ping` / `redis_scan` 等只读 Tool + `list_redis`）— **已完成**
- Kafka Connector（`Execute(action, params)`）+ Plugin（`kafka_cluster_info` / `kafka_topics` / `kafka_consumer_lag` 等 + `list_kafka`）— **已完成**
- 配置扩展：`redis.yaml` / `kafka.yaml` — **已完成**
- 文档同步更新 — **已完成**

### 验收

- [x] Redis Plugin 加载后出现在 `tools/list`
- [x] Redis 只读查询类 Tool 端到端可用（含认证实例）
- [x] Kafka 只读查询类 Tool 端到端可用（`list_kafka` + `kafka_*`；真实集群可选集成）
- [x] README / docs 与 Redis / Kafka 实现一致

### 下一阶段计划

Phase 5 已完成。后续按需扩展更多只读 Kafka action，或推进运维体验（如更多本机 CLI Plugin）。

---

## Phase 6：HTTP API Plugin（已完成）

### 目标

通过 `apis.yaml` 管理多个 HTTP API 服务，基于本地 OpenAPI 3.x 自动生成 MCP Tool，经 HTTP Connector 调用上游。

### 交付物

- 文档契约：[apis.md](apis.md) 及 configuration / architecture / api / plugin / runtime / connector / user-guide 同步 — **已完成**
- `config/apis.yaml.example` + `config.apis` — **已完成**
- ConfigManager：加载 `apis.yaml`、`${ENV}` 展开、`GetAPI` / `ListAPISummaries` — **已完成**
- `internal/openapi`：解析 OpenAPI、Discovery Matcher、Tool 生成 — **已完成**
- `internal/connector/http`：出站 HTTP — **已完成**
- MCP：合并注册 OpenAPI Tools；`tools/call` 执行路径 — **已完成**
- `GET /api/apis`；`POST /api/reload` 重建 OpenAPI Tools — **已完成**
- 磁盘 Plugin：`list_apis` + `ctx.apis.list` — **已完成**

### 验收

- [x] 文档与 example 契约齐全
- [x] `apis.yaml` + 本地 OpenAPI 启动后，过滤后的 Tool 出现在 `tools/list`（registry / MCP 合并）
- [x] `tools/call` 经 HTTP Connector 打到上游（mock httptest），返回统一 `{success, data}`
- [x] Discovery include/exclude 与 Path 匹配单测通过
- [x] Tool 名与磁盘 Plugin 重名时加载失败并保留旧集
- [x] `GET /api/apis` / `list_apis` 脱敏摘要可用；日志无敏感 header
- [x] `go build ./...`、`go vet ./...`、`go test ./...` 通过

### 下一阶段依赖

HTTP Connector 与 OpenAPI Tool 注册稳定后，可扩展更多 media type 或通用 `ctx.http`（非本 Phase 必做）。Phase 7（Command Connector）可并行推进。

---

## Phase 7：Command Connector（本机进程执行）

### 目标

提供 Local Process Executor：按 `commands.yaml` 二进制白名单在 ops-mcp 本机无 shell 执行 CLI，作为尚无专用 Connector 时的能力扩展点（示例用本机 `ping`）。

### 交付物

- 文档契约：connector / runtime / configuration / plugin / user-guide / architecture / api / cursor / README 同步
- `config/commands.yaml.example` + `config.commands`
- ConfigManager：`GetCommand` / `ListCommandSummaries`；缺失文件视为空清单；`path` 须为绝对路径
- `internal/connector/command`：`Exec`（argv、超时、stdout/stderr 截断）
- Runtime：`ctx.command.exec`、`ctx.commands.list`
- `GET /api/commands`；`POST /api/reload` 含 `commands.yaml`
- 磁盘 Plugin：`list_commands`、`host_ping`（示例）

### 验收

- [x] `commands.yaml` 白名单外 `command` → `INVALID_PARAMS`
- [x] 无 shell：argv 原样执行；结果形态 `{ stdout, stderr, exit_code }`
- [x] `host_ping` / `list_commands` 出现在 `tools/list`；端到端可调用
- [x] `GET /api/commands` 可用
- [x] `go build ./...`、`go vet ./...`、`go test ./...` 通过

### 下一阶段计划

Phase 8（SNMP Connector）扩展网络设备只读运维能力。

---

## Phase 8：SNMP Connector（交换机 / 网络设备）

### 目标

通过 `snmp.yaml` 管理大量交换机等 SNMP 设备；支持 SNMPv2c / v3；凭据可用 profile 引用或设备内联；暴露通用 get/walk/bulk 与首批交换机运维 Plugin。

### 交付物

- `config/snmp.yaml.example` + `config.snmp`；双模式凭据互斥校验
- ConfigManager：`GetSNMPDevice` / `ResolveSNMPAuth` / `ListSNMPDeviceSummaries`（labels + 分页）
- `internal/connector/snmp`：Get / Walk / Bulk（gosnmp）；并发与 walk 截断
- Runtime：`ctx.snmp.*`、`ctx.snmp_devices.list`
- `GET /api/snmp`；磁盘 Plugin：`list_snmp_devices` / `snmp_get` / `snmp_walk` / `snmp_bulk` / `snmp_sysinfo` / `snmp_interfaces`

### 验收

- [x] 引用与内联凭据均可加载；互斥违规 fail-fast
- [x] `tools/list` 含上述 SNMP Plugin；`GET /api/snmp` 可用且无密钥泄漏
- [x] Walk/Bulk 截断返回 `truncated: true`
- [x] `go build ./...`、`go vet ./...`、`go test ./...` 通过

### 下一阶段计划

按需增加 LLDP/邻居、厂商私有 OID Plugin，或 CMDB 同步设备清单。Phase 9（TCP/UDP Connector）支持私有字节协议设备。

---

## Phase 9：TCP / UDP Connector（私有协议设备）

### 目标

提供基础 `ctx.tcp.exchange` / `ctx.udp.exchange`，由磁盘 Plugin 编解码业务帧，对接无标准协议的设备（如 UDP 光保）。无 `net.yaml`；地址由调用传入，**建议**在 Plugin 内限制目标。

### 交付物

- `internal/connector/tcp`、`internal/connector/udp`、`internal/connector/netutil`
- Runtime：`ctx.tcp.exchange`、`ctx.udp.exchange`
- 冒烟 Plugin：`tcp_exchange` / `udp_exchange`（透传，文档标明非推荐）
- 本地联调：`deploy/dev-net` + `make net-up`（TCP 19090 / UDP 19091 echo）

### 验收

- [x] hex / `number[]` 载荷编解码；超时与 `max_response_bytes` 生效
- [x] `tools/list` 含 `tcp_exchange` / `udp_exchange`
- [x] `make net-up` 后可对 echo 冒烟
- [x] `go build ./...`、`go vet ./...`、`go test ./...` 通过

### 下一阶段计划

按需增加厂商设备 Plugin（在 `main.js` 固定地址与拼帧），或 TCP TLS 包装。

---

## 明确不做（全阶段 MVP 外）

除非单独立项，否则不实现：

- RBAC / 用户系统
- Permission 引擎
- Audit
- `allow_rules.yaml` / `deny_rules.yaml`（Command 白名单仅限可执行文件路径，不解析子命令正则）

能力边界继续由磁盘 Plugin 集合、OpenAPI Discovery 暴露集合与 `commands.yaml` 二进制白名单表达。
## 每阶段输出规范

每个 Phase 结束时应提供：

1. 目录结构
2. 启动方式（`make run` / 发行包 / Docker）
3. 测试方式（`make check`；相关冒烟 / 集成）
4. 下一阶段计划
