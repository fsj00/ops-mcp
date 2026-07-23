function execute(ctx) {
  var args = [];
  if (ctx.params.type) {
    args.push("-t", ctx.params.type);
  }
  return ctx.ssh.exec({
    host: ctx.params.host,
    command: "dmidecode",
    args: args
  });
}
