# 本地 SNMP 模拟器（开发联调）

用 Docker 跑 [snmpsim](https://github.com/lextudio/docker-snmpsim)，模拟一台交换机 Agent，供 ops-mcp SNMP Connector / Plugin 联调。

## 快速开始

```bash
# 仓库根目录
make snmp-up          # 拉取镜像并后台启动
make snmp-ps          # 查看状态
make snmp-logs        # 看日志
make snmp-down        # 停止并删除容器
```

等价：

```bash
docker compose -f deploy/dev-snmp/docker-compose.yml up -d
```

## 连接参数

| 项 | 值 |
|----|-----|
| 地址 | `127.0.0.1` |
| 端口 | **1161**/UDP（映射容器 161） |
| SNMPv2c community | `public`（对应 `data/public.snmprec`） |
| SNMPv3 user | `readonly` |
| SNMPv3 auth | SHA / `authpass123` |
| SNMPv3 priv | AES / `privpass123` |
| security_level | `authPriv` |
| SNMPv3 contextName | `public`（选中 `public.snmprec`；ops-mcp 设备字段 `context`） |

本机探针（可选，需安装 net-snmp 客户端）：

```bash
snmpget -v2c -c public 127.0.0.1:1161 1.3.6.1.2.1.1.5.0
snmpwalk -v2c -c public 127.0.0.1:1161 1.3.6.1.2.1.1
snmpget -v3 -l authPriv -u readonly -a SHA -A authpass123 -x AES -X privpass123 \
  127.0.0.1:1161 1.3.6.1.2.1.1.5.0
```

## 接入 ops-mcp

```bash
cp config/snmp.yaml.example config/snmp.yaml
# 确保含 local-snmp / local-snmp-v3 设备（见 example）
make run
```

MCP / 管理面验证：

```bash
TOKEN=ops-mcp-local-dev-token
curl -s -H "Authorization: Bearer $TOKEN" 'localhost:20267/api/snmp' | jq .
curl -s -X POST localhost:20267/mcp \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"snmp_sysinfo","arguments":{"device":"local-snmp"}}}'
```

## 数据文件

- [`data/public.snmprec`](data/public.snmprec)：system + ifTable/ifAlias 子集  
- **文件名（不含扩展名）= v2c community**；改 community 请重命名文件并同步 `snmp.yaml`

修改 `.snmprec` 后执行 `make snmp-restart` 生效。

## 集成测试

```bash
make snmp-up
OPS_MCP_INTEGRATION=1 go test -tags=integration ./internal/connector/snmp/ -count=1 -run Local
```

## 说明

- 镜像：`ghcr.io/lextudio/docker-snmpsim:master`（含 amd64 / arm64）
- 仅开发用途；community / v3 密钥为固定测试值，**勿用于生产**
- 不接收 TRAP（`SNMPTRAPD_ENABLED=0`）
