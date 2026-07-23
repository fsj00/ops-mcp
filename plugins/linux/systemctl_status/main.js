function execute(ctx) {
  return ctx.ssh.exec({
    host: ctx.params.host,
    command: "systemctl",
    args: ["status", ctx.params.service, "--no-pager"]
  });
}
