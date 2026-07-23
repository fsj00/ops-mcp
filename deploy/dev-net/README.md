# 本地 TCP / UDP Echo（开发联调）

供 `ctx.tcp.exchange` / `ctx.udp.exchange` 与冒烟 Plugin `tcp_exchange` / `udp_exchange` 使用。

| 协议 | 宿主机 | 容器内 | 行为 |
|------|--------|--------|------|
| TCP | `127.0.0.1:19090` | `9090` | 读至 EOF 后原样回显 |
| UDP | `127.0.0.1:19091` | `9091` | 单包原样回显 |

## 启动 / 停止

```bash
make net-up
make net-ps
make net-logs
make net-down
```

或：

```bash
docker compose -f deploy/dev-net/docker-compose.yml up -d
```

## MCP 冒烟示例

服务已启动且 token 与 `config/ops-mcp.yaml` 一致时：

```bash
TOKEN=ops-mcp-local-dev-token

# TCP
curl -s -X POST localhost:20267/mcp \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"tcp_exchange","arguments":{"ip":"127.0.0.1","port":19090,"data":"0102030a","timeout":"2s"}}}'

# UDP
curl -s -X POST localhost:20267/mcp \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"udp_exchange","arguments":{"ip":"127.0.0.1","port":19091,"data":"0102030a","timeout":"2s"}}}'
```

期望响应中 `hex` 为 `0102030a`。

> `tcp_exchange` / `udp_exchange` 为调试透传 Tool；业务 Plugin 应在 `main.js` 内固定或校验 `ip`/`port`，不建议把任意地址交给 Agent。
