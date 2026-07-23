function execute(ctx) {
  var extra = ctx.params.args || "-a";
  return ctx.ssh.exec({
    host: ctx.params.host,
    command: "bash",
    args: ["-lc", "uname " + extra]
  });
}
