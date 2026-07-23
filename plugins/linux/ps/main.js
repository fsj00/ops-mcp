function execute(ctx) {
  var cmd = "ps aux";
  if (ctx.params.filter) {
    cmd = "ps aux | grep -F -- " + JSON.stringify(ctx.params.filter) + " | grep -v grep";
  }
  return ctx.ssh.exec({
    host: ctx.params.host,
    command: "bash",
    args: ["-lc", cmd]
  });
}
