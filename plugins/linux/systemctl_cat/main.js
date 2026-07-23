function execute(ctx) {
  return ctx.ssh.exec({
    host: ctx.params.host,
    command: "systemctl",
    args: ["cat", ctx.params.unit, "--no-pager"]
  });
}
