function execute(ctx) {
  return ctx.ssh.exec({
    host: ctx.params.host,
    command: "ls",
    args: ["-al", ctx.params.path]
  });
}
