function execute(ctx) {
  var args = ["addr"];
  if (ctx.params.iface) {
    args.push("show", "dev", ctx.params.iface);
  }
  return ctx.ssh.exec({
    host: ctx.params.host,
    command: "ip",
    args: args
  });
}
