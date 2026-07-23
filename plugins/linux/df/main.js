function execute(ctx) {
  var args = ["-hT"];
  if (ctx.params.path) {
    args.push(ctx.params.path);
  }
  return ctx.ssh.exec({
    host: ctx.params.host,
    command: "df",
    args: args
  });
}
