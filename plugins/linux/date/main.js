function execute(ctx) {
  var args = [];
  if (ctx.params.format) {
    var fmt = String(ctx.params.format);
    if (fmt.charAt(0) !== "+") {
      fmt = "+" + fmt;
    }
    args.push(fmt);
  }
  return ctx.ssh.exec({
    host: ctx.params.host,
    command: "date",
    args: args
  });
}
