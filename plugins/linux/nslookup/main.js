function execute(ctx) {
  var args = [ctx.params.name];
  if (ctx.params.server) {
    args.push(ctx.params.server);
  }
  return ctx.ssh.exec({
    host: ctx.params.host,
    command: "nslookup",
    args: args
  });
}
