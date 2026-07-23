function execute(ctx) {
  return ctx.ssh.exec({
    host: ctx.params.host,
    command: "ip",
    args: ["route"]
  });
}
