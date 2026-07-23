function execute(ctx) {
  var count = ctx.params.count != null ? ctx.params.count : 3;
  return ctx.command.exec({
    command: "ping",
    args: ["-c", String(count), ctx.params.host]
  });
}
