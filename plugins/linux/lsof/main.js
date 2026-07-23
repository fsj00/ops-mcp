function execute(ctx) {
  var extra = ctx.params.args || "-i";
  return ctx.ssh.exec({
    host: ctx.params.host,
    command: "bash",
    args: ["-lc", "lsof " + extra]
  });
}
