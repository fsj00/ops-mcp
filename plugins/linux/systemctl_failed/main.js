function execute(ctx) {
  return ctx.ssh.exec({
    host: ctx.params.host,
    command: "systemctl",
    args: ["--failed", "--no-pager"]
  });
}
