function execute(ctx) {
  var n = ctx.params.count || "10";
  // ps 按 CPU 排序，取前 N（含表头共 N+1 行）
  var cmd = "ps aux --sort=-%cpu | head -n " + (parseInt(n, 10) + 1);
  return ctx.ssh.exec({
    host: ctx.params.host,
    command: "bash",
    args: ["-lc", cmd]
  });
}
