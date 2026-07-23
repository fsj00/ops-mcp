function execute(ctx) {
  var args = [];
  if (ctx.params.server) {
    args.push("@" + ctx.params.server);
  }
  args.push(ctx.params.name);
  if (ctx.params.type) {
    args.push(ctx.params.type);
  }
  args.push("+noall", "+answer");
  return ctx.ssh.exec({
    host: ctx.params.host,
    command: "dig",
    args: args
  });
}
