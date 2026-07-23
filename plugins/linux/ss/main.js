function execute(ctx) {
  var extra = ctx.params.args || "-tunlp";
  return ctx.ssh.exec({
    host: ctx.params.host,
    command: "bash",
    args: ["-lc", "ss " + extra]
  });
}
