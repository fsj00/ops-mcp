# 配置说明

## 概览

启动时由 **ConfigManager**（Viper）加载以下文件（路径可由环境变量或 CLI flag 覆盖，默认相对工作目录）：

| 文件 | 用途 |
|------|------|
| `config/ops-mcp.yaml` | 服务、日志、插件目录、默认超时等 |
| `config/hosts.yaml` | SSH 主机清单 |
| `config/databases.yaml` | 数据库连接清单 |
| `config/redis.yaml` | Redis 实例连接清单 |
| `config/kafka.yaml` | Kafka 集群连接清单 |
| `config/apis.yaml` | HTTP API 服务清单（OpenAPI → MCP Tool，详见 [apis.md](apis.md)） |
| `config/commands.yaml` | 本机可执行文件白名单（Command Connector，详见 [connector.md](connector.md)） |
| `config/snmp.yaml` | SNMP 设备与凭据清单（SNMP Connector，详见 [connector.md](connector.md)） |

密钥与私钥仅存在于配置文件或挂载卷中，**禁止**写入镜像层或提交到 git。示例一律使用占位符。`commands.yaml` 通常无密钥（仅绝对路径），可本地维护；**禁止**在其中写入密钥或凭据。

## ops-mcp.yaml

服务主配置（字段与当前实现一致；变更须保持兼容语义）。

```yaml
server:
  host: "0.0.0.0"
  port: 20267
  auth:
    token: "CHANGE_ME"

plugins:
  dir: "./plugins"

config:
  hosts: "./config/hosts.yaml"
  databases: "./config/databases.yaml"
  redis: "./config/redis.yaml"
  kafka: "./config/kafka.yaml"
  apis: "./config/apis.yaml"
  commands: "./config/commands.yaml"
  snmp: "./config/snmp.yaml"

defaults:
  plugin_timeout: 30s

log:
  level: info          # debug | info | warn | error
  encoding: json       # json | console
```

| 键 | 说明 |
|----|------|
| `server.host` / `server.port` | HTTP 监听地址 |
| `server.auth.token` | 访问令牌；非空时除 `/health` 外均需鉴权。可用环境变量 `OPS_MCP_AUTH_TOKEN` 覆盖 |
| `plugins.dir` | Plugin 根目录 |
| `config.hosts` / `config.databases` / `config.redis` / `config.kafka` / `config.apis` / `config.commands` / `config.snmp` | 资源清单路径 |
| `defaults.plugin_timeout` | Plugin 未声明 `timeout` 时的默认值；亦作为 API `endpoint.timeout` 缺省 |
| `log.*` | Zap 日志级别与编码 |

### 鉴权

当 `server.auth.token`（或 `OPS_MCP_AUTH_TOKEN`）非空时，调用方必须提供其一：

- `Authorization: Bearer <token>`
- `X-API-Key: <token>`

未带或错误 token → HTTP `401`。`GET /health` 无需鉴权。

Claude Code `.mcp.json` 示例：

```json
{
  "mcpServers": {
    "ops-mcp": {
      "type": "http",
      "url": "http://127.0.0.1:20267/mcp",
      "headers": {
        "Authorization": "Bearer ${OPS_MCP_AUTH_TOKEN}"
      }
    }
  }
}
```

---

## 通用字段

### labels

`hosts` / `databases` / `redis` / `kafka` / `snmp` devices 条目均可选配置 `labels`：自定义 `string → string`，用于 AI 检索、内容标注、跨资源关联（如同一 `app` / `env` 串起 host、DB、Redis、Kafka、交换机）。不参与连接逻辑；经 `list_*` / `/api/*` 摘要透出。`list_snmp_devices` / `GET /api/snmp` 支持按 labels 精确过滤。

```yaml
labels:
  env: production
  app: order
  owner: platform
```

### 证书 / 密钥材料

`hosts.yaml` 的 SSH 私钥与 `redis.yaml` / `kafka.yaml` 的 TLS 材料（`ca` / `cert` / `private_key`）统一约定：内容字段与 `*_file` **互斥**，内容字段支持三种形态之一：

| 形态 | 写法 | 说明 |
|------|------|------|
| 正文 | `<name>: \|` 多行 PEM | PEM / OpenSSH 正文 |
| 文件 | `<name>_file: /path` | 证书或密钥文件路径 |
| Base64 | `<name>: "LS0tLS1..."` | 对文件原始字节做标准 base64 后的单行字符串（避免 YAML 多行 PEM 易错） |

解析：含 `-----BEGIN` 按正文；否则按标准 base64 解码。日志与 Summary **不回显** 材料内容。

---

## hosts.yaml

SSH 目标主机。

```yaml
hosts:
  - name: prod-web-01
    description: nginx server
    labels:
      env: production
      role: web
    address:
      host: 10.10.1.10
      port: 22
    auth:
      type: password
      username: root
      password: "CHANGE_ME"

  - name: prod-app-01
    description: app server with key auth
    labels:
      env: production
      role: app
    address:
      host: 10.10.1.11
      port: 22
    auth:
      type: private_key
      username: ubuntu
      private_key: |
        -----BEGIN OPENSSH PRIVATE KEY-----
        ***REPLACE_WITH_REAL_KEY***
        -----END OPENSSH PRIVATE KEY-----

  - name: prod-app-02
    description: app server with key file
    address:
      host: 10.10.1.12
      port: 22
    auth:
      type: private_key
      username: ubuntu
      private_key_file: /etc/ops-mcp/keys/prod-app-02.pem
```

### 字段说明

| 字段 | 说明 |
|------|------|
| `name` | 全局唯一；JS 中 `ctx.ssh.exec({ host: name })` 使用此值 |
| `description` | 人类可读说明 |
| `labels` | 可选。自定义标签，见上文「通用字段」 |
| `address.host` / `address.port` | SSH 地址，端口默认 22 |
| `auth.type` | `password` 或 `private_key` |
| `auth.username` | 登录用户 |
| `auth.password` | `type=password` 时必填 |
| `auth.private_key` | `type=private_key` 时与 `private_key_file` **互斥**：PEM / OpenSSH 正文，或文件内容的 base64 |
| `auth.private_key_file` | `type=private_key` 时与 `private_key` **互斥**：私钥文件路径 |

### 认证类型

**password：**

```yaml
auth:
  type: password
  username: root
  password: "CHANGE_ME"
```

**private_key（正文）：**

```yaml
auth:
  type: private_key
  username: ubuntu
  private_key: |
    -----BEGIN OPENSSH PRIVATE KEY-----
    ***
    -----END OPENSSH PRIVATE KEY-----
```

**private_key（文件路径）：**

```yaml
auth:
  type: private_key
  username: ubuntu
  private_key_file: /etc/ops-mcp/keys/ubuntu.pem
```

**private_key（base64）：**

```yaml
auth:
  type: private_key
  username: ubuntu
  private_key: "LS0tLS1CRUdJTi..."   # 私钥文件内容的标准 base64
```

> `type=private_key` 时必须且只能设置 `private_key` 或 `private_key_file` 之一；同时设置或都未设置会报错。

---

## databases.yaml

数据库连接清单。

```yaml
databases:
  - name: order-mysql
    description: order service mysql (readonly)
    labels:
      env: production
      app: order
    type: mysql
    connection:
      host: 10.10.2.10
      port: 3306
      username: readonly
      password: "CHANGE_ME"
      database: orders
    readonly: true
    limit: 1000

  - name: order-pg
    description: order service postgresql (readonly)
    labels:
      env: production
      app: order
    type: postgresql
    connection:
      host: 10.10.2.20
      port: 5432
      username: readonly
      password: "CHANGE_ME"
      database: orders
      sslmode: disable
    readonly: true
    limit: 1000
```

### 字段说明

| 字段 | 说明 |
|------|------|
| `name` | 全局唯一；`ctx.mysql.query` / `ctx.postgres.query` 的 `database` 参数 |
| `description` | 人类可读说明（可选） |
| `labels` | 可选。自定义标签，见上文「通用字段」 |
| `type` | `mysql` 或 `postgresql` |
| `connection.host` / `port` | 地址；端口缺省 mysql=3306、postgresql=5432 |
| `connection.username` / `password` | 凭证 |
| `connection.database` | 默认库名（可选，也可仅在 SQL 中指定） |
| `connection.sslmode` | PostgreSQL 常用；MySQL 可用对应 TLS 字段扩展 |
| `readonly` | `true` 时 Connector 应拒绝写操作 |
| `limit` | SELECT 最大返回行数；缺省 **1000**。Connector 会用外层 `LIMIT` 强制截断 |

### 支持的类型

| type | Connector | JS API |
|------|-----------|--------|
| `mysql` | MySQL Connector | `ctx.mysql.query` |
| `postgresql` | PostgreSQL Connector | `ctx.postgres.query` |

---

## redis.yaml

Redis 实例连接清单。支持：

- 无密码 / `password` / Redis 6+ ACL（`username` + `password`）
- TLS（加密传输）
- **mTLS**（客户端证书 + 可选 CA 校验服务端）

密码认证与 mTLS 可叠加。TLS 材料字段约定见上文「证书 / 密钥材料」。

```yaml
redis:
  - name: local-redis
    description: local docker redis
    labels:
      env: dev
    connection:
      host: 127.0.0.1
      port: 6379
      username: ""            # 可选；Redis 6+ ACL
      password: "CHANGE_ME"   # 可选；无认证可留空
      # 逻辑库 db 由各 redis_* Tool 参数指定，缺省 0（不在此写死）
    readonly: true
    limit: 1000

  - name: prod-redis-mtls
    description: redis with mutual TLS
    labels:
      env: production
      app: cache
    connection:
      host: redis.example.com
      port: 6380
      tls:
        enabled: true
        server_name: redis.example.com
        ca_file: /etc/ops-mcp/certs/ca.crt
        cert_file: /etc/ops-mcp/certs/client.crt
        private_key_file: /etc/ops-mcp/certs/client.key
        # 也可用内容字段：ca / cert / private_key（PEM 或 base64，勿提交真实内容）
        # insecure_skip_verify: false
    readonly: true
    limit: 1000
```

### 字段说明

| 字段 | 说明 |
|------|------|
| `name` | 全局唯一；`ctx.redis.*` 的 `redis` 参数 |
| `description` | 人类可读说明 |
| `labels` | 可选。自定义标签，见上文「通用字段」 |
| `connection.host` / `port` | 地址；端口缺省 **6379** |
| `connection.username` | Redis 6+ ACL 用户名；可空 |
| `connection.password` | 密码；可空（无认证实例） |
| `connection.tls.enabled` | 是否启用 TLS |
| `connection.tls.server_name` | SNI / 证书校验名；缺省用 `host` |
| `connection.tls.ca` / `ca_file` | 校验服务端的 CA（内容或文件，互斥；可选） |
| `connection.tls.cert` / `cert_file` | mTLS 客户端证书（内容或文件，互斥） |
| `connection.tls.private_key` / `private_key_file` | mTLS 客户端私钥（内容或文件，互斥） |
| `connection.tls.insecure_skip_verify` | 跳过服务端证书校验（仅本地排障） |
| `readonly` | 约定为只读运维查询；Connector 仅暴露只读类命令 |
| `limit` | `SCAN` / `HSCAN` / `SLOWLOG GET` / `CLIENT LIST` / `LRANGE` / `ZRANGE` 采样 / `HMGET` fields 等最大条数；缺省 **1000**。调用方传入的 `limit`/`count` **必填且 > 0**，超出时按此上限截断 |

mTLS 时 `cert` 与 `private_key`（各用内容或 `*_file`）需成对出现。

`GET /api/redis` / `list_redis` 仅返回 `tls.enabled`、`has_ca`、`has_client_cert` 等摘要，**不回显** PEM / 私钥内容。

### 安全兜底

| 命令 / Plugin | 兜底 |
|---------------|------|
| `redis_scan` | **必须**提供 `limit`；禁止 `KEYS` |
| `redis_hscan` | **必须**提供 `limit` |
| `redis_lrange` | **必须**提供 `limit`，禁止全量导出 |
| `redis_hmget` | `fields` 非空，数量不得超过配置 `limit` |
| `redis_slowlog_get` | **必须**提供 `count` |
| `redis_client_list` | **必须**提供 `limit`，超出截断 |
| `redis_zrange_sample` | **必须**提供 `limit`，禁止全量导出 |
| `redis_config_get` | 仅 `CONFIG GET`，不提供 `CONFIG SET` |

---

## kafka.yaml

Kafka 集群连接清单。支持：

- 多 bootstrap `brokers`
- 可选 SASL：`plain` / `scram-sha-256` / `scram-sha-512`
- 可选 TLS / mTLS（材料字段约定同 redis）

```yaml
kafka:
  - name: local-kafka
    description: local kafka (dev)
    labels:
      env: dev
    connection:
      brokers:
        - 127.0.0.1:9092
      # sasl:
      #   mechanism: plain
      #   username: readonly
      #   password: "CHANGE_ME"
      # tls:
      #   enabled: true
      #   server_name: kafka.example.com
      #   ca_file: /etc/ops-mcp/certs/ca.crt
    readonly: true
    limit: 1000
```

### 字段说明

| 字段 | 说明 |
|------|------|
| `name` | 全局唯一；`ctx.kafka.*` 的 `kafka` 参数 |
| `description` | 人类可读说明（可选） |
| `labels` | 可选。自定义标签，见上文「通用字段」 |
| `connection.brokers` | bootstrap broker 列表（必填，至少一个） |
| `connection.sasl.mechanism` | `plain` / `scram-sha-256` / `scram-sha-512`；启用 SASL 时建议显式设置 |
| `connection.sasl.username` / `password` | SASL 凭证；启用 SASL 时 username 必填 |
| `connection.tls.*` | 同 `redis.yaml` TLS 字段 |
| `readonly` | 约定为只读 Admin 查询；Connector 仅暴露只读 action |
| `limit` | topics / consumer_groups / lag_summary 等列表上限；缺省 **1000** |

`GET /api/kafka` / `list_kafka` **不回显** 密码与 PEM 材料。

---

## apis.yaml

HTTP API 服务清单：本地 OpenAPI 文档 + Discovery 过滤 → 自动生成 MCP Tool。完整契约见 **[apis.md](apis.md)**。仓库示例见 [config/apis.yaml.example](../config/apis.yaml.example)（含 Prometheus / Loki / Flink）。

最小示例：

```yaml
apis:
  - name: cmdb
    description: CMDB API Service
    openapi:
      path: ./openapi/cmdb.yaml
    endpoint:
      base_url: http://cmdb-api:8080
      timeout: 10s
      verify_tls: true
    prefix: cmdb_
    labels:
      env: production
      system: cmdb
    headers:
      Authorization: "Bearer ${CMDB_TOKEN}"
    discovery:
      include:
        - operation_ids:
            - "^get.*"
            - "^list.*"
      exclude:
        - methods:
            - POST
            - PUT
            - PATCH
            - DELETE
```

### 字段说明（摘要）

| 键 | 说明 |
|----|------|
| `name` | API 服务唯一名称 |
| `openapi.path` | 本地 OpenAPI 3.x 文件路径 |
| `endpoint.base_url` / `timeout` / `verify_tls` | 上游连接 |
| `prefix` | 生成 Tool 名前缀（如 `cmdb_` → `cmdb_getHostById`） |
| `headers` | 固定 Header；支持 `${ENV_NAME}` |
| `discovery.include` / `exclude` | Operation 暴露过滤；缺省/`[]` include = 全部 |

`headers`、`endpoint.base_url`、`openapi.path` 支持环境变量替换；未定义变量导致加载失败。详见 [apis.md](apis.md)。

---

## commands.yaml

本机可执行文件白名单：供 Command Connector（`ctx.command.exec`）按逻辑 `name` 解析绝对路径。完整行为见 [connector.md](connector.md)。

`ops-mcp.yaml` 默认 `config.commands: "./config/commands.yaml"`。仓库自带可开箱使用的 `commands.yaml`（含 ping / traceroute / dig 等常用项）。

```yaml
commands:
  - name: ping
    description: "本机 ping（次数与目标由 Plugin 约束）"
    path:
      - /sbin/ping
      - /bin/ping
      - /usr/bin/ping
      - /usr/sbin/ping
```

| 键 | 必填 | 说明 |
|----|------|------|
| `name` | 是 | 全局唯一；JS 入参 `command` 使用此值 |
| `path` | 是 | **绝对路径数组**（也兼容单个字符串）；启动 / reload 时按顺序查找本机存在且可执行的项，取**第一个**；全部不可用则跳过该条目并打 **warning**；非绝对路径则配置错误 |
| `description` | 否 | 进摘要 API / `ctx.commands.list` |

缺失 `commands.yaml` 视为空清单（与 `redis.yaml` / `apis.yaml` 一致）；空清单下任何 `ctx.command.exec` 均因 name 未登记而失败。

仓库同时提供 `config/commands.yaml.example`。文件通常不含密钥，是否加入 `.gitignore` 由部署方决定；**禁止**写入密钥材料。

---

## snmp.yaml

SNMP 设备与凭据清单：供 SNMP Connector（`ctx.snmp.*`）按设备 `name` 解析地址与认证。完整行为见 [connector.md](connector.md)。

`ops-mcp.yaml` 默认 `config.snmp: "./config/snmp.yaml"`。仓库仅保留 `config/snmp.yaml.example`；含真实 community / v3 密码的 `snmp.yaml` 不得提交。

**双模式凭据（每台设备恰好二选一）：**

1. `credential: <name>` — 引用顶层 `credentials[]`（大规模共享推荐）
2. 内联 `auth:` — 字段形状与 credential 条目相同（无 `name`）

两者都填或都不填 → **加载失败**。不做全局默认 community。

```yaml
credentials:
  - name: dc1-ro-v2c
    version: 2c
    community: "CHANGE_ME"
  - name: core-v3
    version: 3
    security_level: authPriv
    username: readonly
    auth_protocol: SHA
    auth_password: "CHANGE_ME"
    priv_protocol: AES
    priv_password: "CHANGE_ME"

devices:
  - name: sw-dc1-core-01
    description: DC1 core switch
    labels: { site: dc1, role: core }
    address: { host: 10.10.0.1, port: 161 }
    credential: dc1-ro-v2c
  - name: sw-lab-01
    labels: { site: lab }
    address: { host: 10.99.0.1 }
    auth:
      version: 2c
      community: "CHANGE_ME"

defaults:
  timeout: 5s
  retries: 1
  max_repetitions: 25
  walk_max_oids: 1000
```

| 键 | 说明 |
|----|------|
| `credentials[].name` | 凭据 profile 名；被 `devices[].credential` 引用 |
| `credentials[]` / `auth` | `version`: `2c` \| `3`；v2c 需 `community`；v3 需 `username` + `security_level` 等 |
| `devices[].name` | 全局唯一；Plugin 参数 `device` 使用此值 |
| `devices[].address.host` / `port` | UDP 目标；port 缺省 161 |
| `devices[].credential` / `auth` | 互斥 |
| `devices[].context` | 可选 SNMPv3 contextName（本地 snmpsim 需设为数据文件名，如 `public`） |
| `defaults.*` | 设备未覆盖时的 timeout / retries / max_repetitions / walk_max_oids |

本地 Docker 模拟器：`make snmp-up`，详见 [deploy/dev-snmp/README.md](../deploy/dev-snmp/README.md)。

缺失 `snmp.yaml` 视为空清单。摘要 API / `list_snmp_devices` 不回显密钥，可暴露 `auth_mode`（`credential` \| `inline`）与引用名。

---

## ConfigManager 行为

1. 进程启动时加载上述配置；`hosts.yaml` 缺失则启动失败；`databases.yaml` / `redis.yaml` / `apis.yaml` / `commands.yaml` / `snmp.yaml` 缺失视为空清单。
2. 向 Connector / OpenAPI 加载器提供：
   - `GetHost(name string) (Host, error)`
   - `GetDatabase(name string) (Database, error)`
   - `GetRedis(name string) (RedisInstance, error)`
   - `GetAPI(name string) (APIService, error)`
   - `ListAPISummaries() []APISummary`（不含 headers 原文）
   - `GetCommand(name string) (Command, error)`
   - `ListCommandSummaries() []CommandSummary`
   - `GetSNMPDevice` / `ResolveSNMPAuth` / `ListSNMPDeviceSummaries`（支持 labels 过滤与分页）
3. `POST /api/reload` 可选重载配置（见 [api.md](api.md)）；默认至少重载 Plugin；`config: true` 时同步重载资源 YAML（含 `apis.yaml` / `commands.yaml` / `snmp.yaml`）并重建 OpenAPI Tools。

## 安全建议

- 生产环境用只读 DB / Redis 账号，并设置 `readonly: true`。
- SSH 优先密钥认证；密钥权限 `600`，通过 volume 挂载。
- 将 `config/hosts.yaml`、`config/databases.yaml`、`config/redis.yaml`、`config/apis.yaml`、`config/snmp.yaml`（含真实密钥）加入 `.gitignore`；仓库只保留 `*.example`。
- `commands.yaml` 仅登记可信绝对路径；子命令安全由各磁盘 Plugin 固定，勿暴露通用「任意 argv」Tool。
- SNMP 优先共享只读 community / v3 用户；日志不记录 community / auth_password / priv_password。
- 日志中脱敏 `password` / `private_key` / `Authorization` 等敏感 header。
- API Discovery 应用 `exclude` 挡住写操作与敏感 Operation（见 [apis.md](apis.md)）。

## 与 Plugin / Tool 的关系

配置描述 **资源**；能力来自 **磁盘 Plugin** 或 **OpenAPI 生成 Tool**。

- 没有 `linux_ls` Plugin → Agent 不能列目录。
- 没有 `prod-web-01` host → 即使有 Plugin，以该 host 调用也会失败。
- 没有暴露的 OpenAPI Operation → 不会出现对应 MCP Tool。
- 没有 `ping` 白名单条目 → 即使有 `host_ping` Plugin，`ctx.command.exec` 也会因未登记失败。
- 资源与能力解耦，便于同一套 Plugin / OpenAPI 对接多套环境配置。
