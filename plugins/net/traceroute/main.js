function execute(ctx) {
  var args = [];
  if (ctx.params.max_hops != null) {
    args.push("-m", String(ctx.params.max_hops));
  }
  args.push(ctx.params.host);
  return ctx.command.exec({
    command: "traceroute",
    args: args
  });
}
