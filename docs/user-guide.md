# ops-mcp 用户手册

面向部署运维人员与 Plugin 作者。更细的契约见同目录其它文档；本手册覆盖：**是什么、怎么部署、内置了哪些能力、如何自己加 Plugin**。

---

## 1. 介绍

### 1.1 是什么

**ops-mcp** 是自研的 Remote MCP Server（Go）。Claude Code、Codex、ChatGPT Agent 等 AI Agent 通过 HTTP JSON-RPC（`POST /mcp`）调用运维能力：SSH 远程执行、Docker / MySQL / PostgreSQL / Redis 查询、HTTP API（OpenAPI 生成 Tool）、主机与资源清单等。

### 1.2 核心理念：能力即可见 Tool

| 原则 | 说明 |
|------|------|
| 未加载 = 无能力 | 没有对应磁盘 Plugin 或未暴露的 OpenAPI Operation，就不会出现在 `tools/list`，也无法 `tools/call` |
| Tool 来源 | 磁盘 Plugin（`plugin.yml` + `main.js`）或 `apis.yaml` + OpenAPI 自动生成 |
| 分层职责 | Go：协议、加载、参数校验、Connector、OpenAPI Tool 执行；JS（`main.js`）：编排并调用 `ctx.*`；YAML（`plugin.yml`）：磁盘 Plugin 元数据与 input schema |

MVP **不做** RBAC、用户系统、Permission、Audit、allow/deny rules。能力边界由「是否部署并加载了对应 Tool」决定。

### 1.3 请求链路（简图）

```text
AI Agent ──POST /mcp──► ops-mcp
                          ├─ 磁盘 Plugin：校验参数 → Goja 执行 main.js → ctx.ssh / docker / mysql / redis / ...
                          └─ OpenAPI Tool：校验 inputSchema → HTTP Connector → 上游 API
```

### 1.4 关键概念

| 概念 | 说明 |
|------|------|
| MCP Tool | Agent 可见的工具名：磁盘 Plugin 为 `plugin.yml` 的 `name`；OpenAPI 为 `{prefix}{operationId}` |
| Connector | Go 侧连接器：`ssh`、`docker`、`mysql`、`postgres`、`redis`、`http`、`snmp`、`tcp`、`udp`、配置读取等；JS 禁止原始 socket |
| 资源 name | 如 `hosts.yaml` 的 `dev-ssh-111`、`redis.yaml` 的 `local-redis`、`kafka.yaml` 的 `local-kafka`、`apis.yaml` 的 `cmdb`；Plugin 里只写 name，不写死 IP/密码 |

---

## 2. 部署

### 2.1 环境要求

- Go ≥ 1.24（源码编译时）
- 可访问的 SSH 目标机（使用 Linux / 远程 Docker Plugin 时）
- Make、Docker（可选，用于 `make` / 镜像部署）

### 2.2 准备配置

```bash
cp config/ops-mcp.yaml.example config/ops-mcp.yaml
cp config/hosts.yaml.example config/hosts.yaml
cp config/databases.yaml.example config/databases.yaml
cp config/redis.yaml.example config/redis.yaml
cp config/kafka.yaml.example config/kafka.yaml
cp config/apis.yaml.example config/apis.yaml
cp config/snmp.yaml.example config/snmp.yaml   # 可选；SNMP 设备
make snmp-up                                   # 可选；本地 snmpsim（UDP 1161）
make net-up                                    # 可选；本地 TCP/UDP echo（19090/19091）
```

编辑要点：

1. **`config/ops-mcp.yaml`**：监听地址、`server.auth.token`、插件目录等。  
   - Token 也可用环境变量 `OPS_MCP_AUTH_TOKEN` 覆盖。
2. **`config/hosts.yaml`**：填写真实 SSH 主机与凭据（密码或私钥）。  
   - **勿提交**到 git（已在 `.gitignore`）。
3. **`config/databases.yaml`**：数据库清单（含 `limit`，缺省 1000）。
4. **`config/redis.yaml`**：Redis 实例清单（含认证 / TLS / `limit`）。
5. **`config/kafka.yaml`**：Kafka 集群清单（brokers / 可选 SASL、TLS / `limit`）。
6. **`config/apis.yaml`**（可选）：HTTP API 服务 + 本地 OpenAPI 路径；`headers` 可用 `${ENV_NAME}`。详见 [apis.md](apis.md)。
7. **`config/snmp.yaml`**（可选）：SNMP 设备；本地联调用 `make snmp-up`（见 [deploy/dev-snmp/README.md](../deploy/dev-snmp/README.md)）。
8. **TCP/UDP 联调**（可选）：`make net-up`（见 [deploy/dev-net/README.md](../deploy/dev-net/README.md)）；业务 Plugin 应在 `main.js` 限制 `ip`/`port`。

`hosts.yaml` 示例结构（密码请换成真实值，勿写入文档或镜像）：

```yaml
hosts:
  - name: prod-web-01          # Plugin 里引用的就是这个 name
    description: nginx server
    labels:
      env: production
    address:
      host: 10.10.1.10
      port: 22
    auth:
      type: password           # 或 private_key
      username: root
      password: "CHANGE_ME"
      # private_key 时三选一：
      # private_key: |
      #   -----BEGIN OPENSSH PRIVATE KEY-----
      #   ***
      #   -----END OPENSSH PRIVATE KEY-----
      # private_key_file: /etc/ops-mcp/keys/host.pem
      # private_key: "LS0tLS1CRUdJTi..."   # 文件内容 base64
```

鉴权：当 token 非空时，除 `GET /health` 外均需：

- `Authorization: Bearer <token>`，或
- `X-API-Key: <token>`

### 2.3 方式 A：源码 / 二进制（开发）

```bash
# 编译与门禁
make check
# 或：make build / make test / make vet

# 启动（默认读取 ./config/ops-mcp.yaml）
make run
# 等价：
#   ./bin/ops-mcp --config ./config/ops-mcp.yaml
#   go run ./cmd/server --config ./config/ops-mcp.yaml
```

### 2.4 方式 B：发行包 + systemd（推荐生产）

在构建机打包：

```bash
make tar
# 交叉编译到 Linux 服务器：
make tar GOOS=linux GOARCH=amd64
```

产物：`dist/ops-mcp-<version>-<os>-<arch>.tar.gz`（含二进制、plugins、配置模板、`deploy/` 脚本）。

在目标机安装：

```bash
tar -xzf ops-mcp-*-linux-amd64.tar.gz
cd ops-mcp-*-linux-amd64
sudo ./deploy/install.sh
```

默认安装到 `/opt/ops-mcp`，注册 systemd 服务 `ops-mcp` 并启动。首次会从 `*.example` 生成配置（**不覆盖**已有文件）：

```bash
sudo vi /opt/ops-mcp/config/ops-mcp.yaml   # token / 端口
sudo vi /opt/ops-mcp/config/hosts.yaml    # SSH 主机
sudo systemctl restart ops-mcp
```

常用运维：

```bash
sudo systemctl start|stop|restart|status ops-mcp
sudo journalctl -u ops-mcp -f
sudo /opt/ops-mcp/deploy/uninstall.sh            # 停服务，保留文件
sudo /opt/ops-mcp/deploy/uninstall.sh --purge    # 删除安装目录
```

更多选项见 [deploy/README.md](../deploy/README.md)。

### 2.5 方式 C：Docker 部署

```bash
make docker                 # 构建镜像 ops-mcp:latest
make docker-run             # 挂载本地 config/ 与 plugins/，端口默认 20267
```

手动示例（端口需与挂载的 `ops-mcp.yaml` 中 `server.port` 一致）：

```bash
docker run --rm -p 20267:20267 \
  -v "$PWD/config:/app/config:ro" \
  -v "$PWD/plugins:/app/plugins:ro" \
  -e OPS_MCP_AUTH_TOKEN \
  ops-mcp:latest
```

说明：

- 镜像内嵌的是 `*.yaml.example`（占位凭据）。**生产务必挂载**含真实凭据的 `config/`，且不要把密钥打进镜像层。
- Docker 相关 Tool 一律经 SSH 在目标机执行，须配置 `hosts.yaml`；镜像部署时挂载真实 config 即可，无需 Docker socket。

### 2.6 验证部署

假设监听 `20267`，token 与配置一致：

```bash
TOKEN=ops-mcp-local-dev-token

curl -s localhost:20267/health | jq .

curl -s -H "Authorization: Bearer $TOKEN" \
  localhost:20267/api/plugins | jq .

curl -s -H "Authorization: Bearer $TOKEN" \
  localhost:20267/api/tools | jq .

curl -s -H "Authorization: Bearer $TOKEN" \
  localhost:20267/api/databases | jq .

curl -s -H "Authorization: Bearer $TOKEN" \
  localhost:20267/api/redis | jq .
curl -s -H "Authorization: Bearer $TOKEN" \
  localhost:20267/api/kafka | jq .

curl -s -X POST localhost:20267/mcp \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | jq .
```

### 2.7 接入 Claude Code（示例）

在项目目录创建 `.mcp.json`：

```json
{
  "mcpServers": {
    "ops-mcp": {
      "type": "http",
      "url": "http://127.0.0.1:20267/mcp",
      "headers": {
        "Authorization": "Bearer ops-mcp-local-dev-token"
      }
    }
  }
}
```

也可用 `Authorization: Bearer ${OPS_MCP_AUTH_TOKEN}`，与服务端 token 对齐。

调用示例：

- `list_hosts`：无需参数，查看可用主机
- `linux_top`：`host=dev-ssh-111`，`count=10`
- `docker_ps`：`host=dev-ssh-111`（`host` 必填）

更完整的 API 约定见 [api.md](api.md)、配置字段见 [configuration.md](configuration.md)。

---

## 3. 内置 Plugin 与 OpenAPI Tools

启动时扫描 `plugins/`（路径由 `ops-mcp.yaml` 的 `plugins.dir` 指定）。分类子目录（`linux` / `docker` / `mysql` / `postgres` / `redis` / `hosts` / `databases` / `apis`）仅用于组织，**磁盘 Tool 名以 `plugin.yml` 的 `name` 为准**。另可从 `apis.yaml` 生成 OpenAPI Tools（见 §3.5）。

### 3.1 主机与配置

| Tool 名 | 说明 | 主要参数 | target |
|---------|------|----------|--------|
| `list_hosts` | 列出 `hosts.yaml` 中的主机元数据（不含密码/私钥） | 无 | config |
| `list_databases` | 列出 `databases.yaml` 中的库元数据（不含密码） | 无 | config |
| `list_redis` | 列出 `redis.yaml` 中的实例元数据（不含密码） | 无 | config |
| `list_kafka` | 列出 `kafka.yaml` 中的集群元数据（不含密码） | 无 | config |
| `list_apis` | 列出 `apis.yaml` 中的 API 服务摘要（不含 headers 原文） | 无 | config |
| `list_commands` | 列出 `commands.yaml` 本机命令白名单 | 无 | config |
| `list_snmp_devices` | 列出 `snmp.yaml` SNMP 设备（可 labels 过滤） | 可选 labels/limit/offset | config |
| `snmp_get` / `snmp_walk` / `snmp_bulk` | 通用 SNMP 查询 | `device` + oids/oid | snmp |
| `snmp_sysinfo` / `snmp_interfaces` | 交换机系统信息 / 端口表 | `device` | snmp |
| `ops_mcp_health` | 探测 ops-mcp `GET /health`（`ctx.http` URL 直连示例） | `base_url`（可选） | http |
| `host_ping` | 本机 `ping`（Command Connector 示例） | `host`；`count`（可选） | command |
| `tcp_exchange` | TCP 原始字节交换（调试透传，非推荐业务模式） | `ip`/`port`/`data` | tcp |
| `udp_exchange` | UDP 原始字节交换（调试透传，非推荐业务模式） | `ip`/`port`/`data` | udp |

### 3.2 Linux（经 SSH）

以下 Tool 均需必填参数 **`host`**（`hosts.yaml` 中的 name）。

| Tool 名 | 说明 | 其它主要参数 |
|---------|------|----------------|
| `linux_ls` | 目录列表（`ls -al`） | `path`（必填） |
| `linux_cat` | 读文件 | `path`（必填） |
| `linux_grep` | 文本检索 | `pattern`、`path`（必填） |
| `linux_tail` | 文件尾部 | `path`（必填），`lines`（可选，默认 50） |
| `linux_du` | 目录占用（`du -h`） | `path`（必填），`max_depth`（可选，默认 1） |
| `linux_df` | 文件系统空间（`df -hT`） | `path`（可选） |
| `linux_lsblk` | 块设备 | — |
| `linux_journalctl` | systemd 日志 | `unit` / `lines` / `since` / `boot` / `priority` / `grep`（可选） |
| `linux_dmesg` | 内核日志（`dmesg`） | `lines` / `level` / `human`（可选） |
| `linux_systemctl_status` | systemd 服务状态 | `service`（必填） |
| `linux_systemctl_cat` | 单元文件内容 | `unit`（必填） |
| `linux_systemctl_list_units` | 列出单元 | `type` / `state` / `all`（可选） |
| `linux_systemctl_failed` | 失败单元 | — |
| `linux_top` | CPU 占用最高的进程 | `count`（可选，默认 10） |
| `linux_ps` | 进程列表 | `filter`（可选，grep 关键字） |
| `linux_free` | 内存使用 | — |
| `linux_lscpu` | CPU 信息 | — |
| `linux_dmidecode` | DMI/SMBIOS（通常需 root） | `type`（可选，如 system / memory） |
| `linux_uptime` | 运行时间与负载 | — |
| `linux_uname` | 内核标识 | `args`（可选，默认 `-a`） |
| `linux_hostnamectl` | 主机名信息 | — |
| `linux_timedatectl` | 时间与时区 | — |
| `linux_date` | 当前时间 | `format`（可选，`+FORMAT`） |
| `linux_ip_addr` | 网卡地址（`ip addr`） | `iface`（可选） |
| `linux_ip_route` | 路由表（`ip route`） | — |
| `linux_ss` | 套接字（`ss`） | `args`（可选，默认 `-tunlp`） |
| `linux_netstat` | 网络连接（`ss -tunlp`） | — |
| `linux_lsof` | 打开的文件/端口 | `args`（可选，默认 `-i`） |
| `linux_nslookup` | DNS（`nslookup`） | `name`（必填），`server`（可选） |
| `linux_dig` | DNS（`dig`） | `name`（必填），`type` / `server`（可选） |
| `linux_raid_detect` | **先调用**：探测 RAID 厂商 / CLI / 软 RAID，并给出建议 Plugin | — |
| `linux_raid_storcli` | Broadcom/LSI MegaRAID（`storcli`）只读 | `action`（可选：summary/controllers/virtual_drives/physical_drives/all），`controller`（可选） |
| `linux_raid_megacli` | Broadcom/LSI 旧版（`MegaCli`）只读 | `action`（可选：adapter/virtual_drives/physical_drives/bbu/config） |
| `linux_raid_perccli` | Dell PERC（`perccli`）只读 | 同 `linux_raid_storcli` 的 `action` / `controller` |
| `linux_raid_ssacli` | HPE Smart Array（`ssacli`/`hpssacli`）只读 | `action`（可选：summary/status/config/detail），`slot`（可选） |
| `linux_raid_mdadm` | Linux 软件 RAID（`mdadm` / `/proc/mdstat`）只读 | `action`（可选：mdstat/scan/detail/examine），`device`（detail 时必填） |
| `linux_raid_arcconf` | Adaptec/Microchip（`arcconf`）只读 | `action`（可选：list/config/status/version），`controller`（可选） |

RAID 推荐流程：先 `linux_raid_detect` → 按返回的 `recommended` / `suggested_plugin` 调用对应厂商 Plugin。以上均为只读查询，不暴露改配 / 重建 / 删除类参数。

### 3.3 Docker

以下 Tool 均经 SSH 在远端执行，**`host` 必填**（无本地 Docker 回退）。

| Tool 名 | 说明 | 其它主要参数 |
|---------|------|----------------|
| `docker_ps` | 容器列表 | `all`（可选，是否含已停止） |
| `docker_logs` | 容器日志 | `container`（必填），`tail` / `since` / `timestamps`（可选） |
| `docker_info` | Docker 守护进程信息 | — |
| `docker_stats` | 容器资源占用（`--no-stream`） | `container`（可选；省略则全部运行中） |
| `docker_inspect` | 容器 / 镜像详情 | `target`（必填） |
| `docker_top` | 容器内进程 | `container`（必填），`ps_args`（可选） |
| `docker_history` | 镜像层历史 | `image`（必填） |

### 3.4 Database

均需必填参数 `database`（`databases.yaml` 中的 name）。`*_query` 仅接受单条 SELECT，并按配置 `limit`（默认 1000）截断。

| Tool | 说明 | 额外参数 |
|------|------|----------|
| `postgres_query` | PostgreSQL SELECT | `sql`（必填） |
| `postgres_version` | PostgreSQL 版本 | — |
| `mysql_query` | MySQL SELECT | `sql`（必填） |
| `mysql_version` | MySQL 版本 | — |

Redis / Kafka 完整 Tool 清单见 [plugin.md](plugin.md)。

### 3.5 HTTP API（OpenAPI 生成 Tool）

在 `apis.yaml` 中配置 API 服务与本地 OpenAPI 文档后，启动时按 Discovery 规则自动生成 MCP Tool（**无需**为每个 Operation 写 `plugin.yml`）。

典型流程：

1. 准备 OpenAPI 文件（仓库已附 `config/openapi/` 下 prometheus / loki / flink / starrocks 只读文档，也可自备如 `./openapi/cmdb.yaml`）。
2. 配置 `config/apis.yaml`（可复制 [apis.yaml.example](../config/apis.yaml.example)，含 Prometheus / Loki / Flink 示例；按环境改 `endpoint`、`prefix`、`headers`、`discovery`）。
3. 设置环境变量（如 `CMDB_TOKEN`）供 `${CMDB_TOKEN}` 展开。
4. 启动后用 `list_apis` 查看服务；用 `tools/list` 查找形如 `prometheus_query_instant`、`loki_query_range`、`flink_list_jobs` 的 Tool。

完整契约见 [apis.md](apis.md)。

### 3.6 统一返回形态

业务结果一般为：

```json
{ "success": true, "data": { } }
```

或失败：

```json
{ "success": false, "error": { "code": "...", "message": "..." } }
```

详情见 [api.md](api.md)。

---

## 4. 如何自己添加 Plugin

新增磁盘能力 = 新增一个 Plugin 目录（`plugin.yml` + `main.js`），**不必改 MCP 协议层**。必要时才扩展 Go Connector。若目标是暴露已有 HTTP API，优先用 `apis.yaml` + OpenAPI（见 [apis.md](apis.md)），不必手写 JS。
### 4.1 步骤概览

1. 在 `plugins/<分类>/<插件名>/` 下创建目录。  
2. 编写 `plugin.yml`（Tool 元数据与参数）。  
3. 编写 `main.js`（`execute(ctx)`，只通过 `ctx.*` 调用 Connector）。  
4. 重启服务（或使用管理端 reload，若已启用）。  
5. 用 `tools/list` / `tools/call` 或 `/api/tools` 验证。

### 4.2 目录约定

```text
plugins/
└── linux/                 # 分类名随意，仅组织用
    └── df/                # 插件目录
        ├── plugin.yml
        └── main.js
```

规则：

- 每目录必须有 **`plugin.yml`** 与 **`main.js`**
- `name` 全局唯一，建议 `领域_动作`（如 `linux_df`）
- 资源用配置中的 **name** 引用，禁止在 JS 里写死 IP/账号/密码

### 4.3 编写 plugin.yml

| 字段 | 必填 | 说明 |
|------|------|------|
| `name` | 是 | MCP Tool 名 |
| `version` | 是 | 如 `"1.0"` |
| `description` | 是 | 给 Agent 看的说明（写清楚用途与参数） |
| `type` | 是 | 如 `command` / `query` |
| `target.type` | 是 | `ssh` / `docker` / `config` / `http` / `command` 等 |
| `input` | 是 | 参数 map；可为空对象 `{}` |
| `input.<k>.type` | 是 | `string` / `number` / `boolean` 等 |
| `input.<k>.required` | 否 | 默认 `false` |
| `input.<k>.description` | 否 | 进入 MCP inputSchema |
| `runtime` | 是 | MVP 固定 `javascript` |
| `timeout` | 否 | 如 `10s`；缺省用全局 `defaults.plugin_timeout` |

示例：假设新增「查看磁盘」`linux_df`：

```yaml
name: linux_df
version: "1.0"
description: 查看远程主机磁盘使用（df -h）
type: command

target:
  type: ssh

input:
  host:
    type: string
    required: true
    description: hosts.yaml 中的主机 name

runtime: javascript
timeout: 10s
```

### 4.4 编写 main.js

必须定义全局函数 **`execute(ctx)`**：

```javascript
function execute(ctx) {
  return ctx.ssh.exec({
    host: ctx.params.host,
    command: "df",
    args: ["-h"]
  });
}
```

常用 `ctx` API（完整列表见 [runtime.md](runtime.md)）：

| API | 用途 |
|-----|------|
| `ctx.params` | 已校验的入参 |
| `ctx.ssh.exec({ host, command, args })` | SSH 执行命令 |
| `ctx.docker.ps({ host, all? })` | 容器列表（`host` 必填） |
| `ctx.docker.logs({ host, container, ... })` | 容器日志 |
| `ctx.docker.info({ host })` | Docker info |
| `ctx.docker.stats({ host, container? })` | 资源占用 |
| `ctx.docker.inspect({ host, target })` | inspect |
| `ctx.docker.top({ host, container, ps_args? })` | 容器内进程 |
| `ctx.docker.history({ host, image })` | 镜像历史 |
| `ctx.hosts.list()` | 主机摘要（无密钥） |
| `ctx.commands.list()` | 本机命令白名单摘要 |
| `ctx.command.exec({ command, args })` | 本机执行白名单命令（无 shell） |
| `ctx.postgres.query` / `version` | PostgreSQL 只读查询 / 版本 |
| `ctx.mysql.query` / `version` | MySQL 只读查询 / 版本 |

约束：

- JS **不得**直接建 TCP/SSH/DB/本机进程；只能调 `ctx.*`
- 无 `require` / npm；仅语言能力 + 注入的 `ctx`
- 推荐直接 `return ctx.*.*(...)`，由 Go 统一包装错误

### 4.5 验证新 Plugin

```bash
# 重启后检查是否出现在列表中
curl -s -H "Authorization: Bearer $TOKEN" localhost:20267/api/tools | jq .

# MCP tools/call 示例（以 linux_df 为例）
curl -s -X POST localhost:20267/mcp \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/call",
    "params": {
      "name": "linux_df",
      "arguments": { "host": "dev-ssh-111" }
    }
  }' | jq .
```

单个 Plugin 损坏时，实现上通常会记录错误并跳过，不影响其它已加载 Plugin；以实际日志为准。

### 4.6 用 Command Connector 扩展未内置能力

当 ops-mcp 尚无专用 Connector，但本机（或跳板机）已有可用 CLI（如几乎各系统都有的 `ping`，或按需安装的 `kubectl`）时：

| 路径 | 适用 |
|------|------|
| **本机 CLI** | ops-mcp 所在主机已装工具 → `commands.yaml` 白名单 + `target.type: command` + `ctx.command.exec` |
| **远端 CLI** | 工具在 SSH 目标机上 → 继续用 `ctx.ssh.exec`（与 `linux_*` 相同），**不必**走 Command Connector |

步骤（本机）：

1. 在 `commands.yaml` 登记逻辑名与绝对路径数组（如 `name: ping` → `[/sbin/ping, /bin/ping, …]`；加载时取本机第一个可用路径）。
2. 新增磁盘 Plugin：`main.js` **固定** `command` 名与参数形态，仅把 MCP 业务参数拼进 `args`。
3. 重启或 `POST /api/reload`（含 `config: true`）；用 `GET /api/commands` / `tools/call` 验证。

选型：已有 SSH 且只需远端执行 → 只加 SSH Plugin；需要本机进程且要二进制白名单 → Command Connector。勿做通用「任意命令」Tool。

### 4.7 什么时候需要改 Go？

| 场景 | 做法 |
|------|------|
| 已有 Connector 能覆盖（SSH 跑一条命令、Docker ps/logs、本机白名单 CLI） | **只加 Plugin**（本机 CLI 还须在 `commands.yaml` 登记） |
| 暴露已有 HTTP API（有 OpenAPI） | 配置 `apis.yaml` + Discovery；见 [apis.md](apis.md) |
| 需要新协议 / 新资源类型（如 Redis） | 先实现 Connector + Runtime 桥接，再写 Plugin；契约见 [connector.md](connector.md) |

---

## 5. 安全与运维注意

- 真实密码、私钥、API Token 只放在本地 `config/hosts.yaml` / `databases.yaml` / `redis.yaml` / `kafka.yaml` / `apis.yaml`（或环境变量 / 挂载卷），**禁止**写入可提交文件、镜像、日志。
- `commands.yaml` 只登记可信绝对路径，勿写入密钥；子命令由 Plugin 固定。
- Token 使用强随机值；生产用环境变量注入，勿把生产 token 提交进仓库。
- MVP 无细粒度权限：能访问 MCP 的调用方即可调用**已加载的全部 Tool**。通过网络隔离 + Token + 只部署必要 Plugin / 收紧 Discovery / 收紧命令白名单控制面。
- Connector / 日志应对 `password` / `private_key` / `Authorization` 脱敏。

---

## 6. 相关文档

| 文档 | 内容 |
|------|------|
| [configuration.md](configuration.md) | 配置项详解 |
| [apis.md](apis.md) | HTTP API：OpenAPI → MCP Tool |
| [plugin.md](plugin.md) | Plugin 框架契约 |
| [runtime.md](runtime.md) | `ctx` API |
| [api.md](api.md) | HTTP / MCP 接口 |
| [connector.md](connector.md) | Connector |
| [architecture.md](architecture.md) | 架构 |
| [roadmap.md](roadmap.md) | 路线图 |
| [../README.md](../README.md) | 项目总览 |
| [../cursor.md](../cursor.md) | 开发者 / Agent 门禁 |
| [../Makefile](../Makefile) | `make help` 查看构建目标 |
