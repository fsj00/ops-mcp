function execute(ctx) {
  return ctx.ssh.exec({
    host: ctx.params.host,
    command: "timedatectl",
    args: ["status"]
  });
}
