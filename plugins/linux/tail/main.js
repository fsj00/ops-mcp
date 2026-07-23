function execute(ctx) {
  var n = ctx.params.lines || "50";
  return ctx.ssh.exec({
    host: ctx.params.host,
    command: "tail",
    args: ["-n", String(n), ctx.params.path]
  });
}
