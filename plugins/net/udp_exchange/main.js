// 冒烟/调试透传：不建议生产业务 Plugin 把任意 ip/port 交给 Agent。
function execute(ctx) {
  var req = {
    ip: ctx.params.ip,
    port: ctx.params.port,
    data: ctx.params.data
  };
  if (ctx.params.timeout != null) {
    req.timeout = ctx.params.timeout;
  }
  if (ctx.params.max_response_bytes != null) {
    req.max_response_bytes = ctx.params.max_response_bytes;
  }
  return ctx.udp.exchange(req);
}
