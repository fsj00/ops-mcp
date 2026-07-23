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
  return ctx.command.exec({
    command: "dig",
    args: args
  });
}
