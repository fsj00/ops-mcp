function execute(ctx) {
  return ctx.ssh.exec({
    host: ctx.params.host,
    command: "grep",
    args: ["-n", ctx.params.pattern, ctx.params.path]
  });
}
