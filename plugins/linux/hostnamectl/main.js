function execute(ctx) {
  return ctx.ssh.exec({
    host: ctx.params.host,
    command: "hostnamectl",
    args: ["status"]
  });
}
