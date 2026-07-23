function execute(ctx) {
  var args = ["list-units", "--no-pager"];
  if (ctx.params.all === true) args.push("--all");
  if (ctx.params.type) args.push("--type=" + ctx.params.type);
  if (ctx.params.state) args.push("--state=" + ctx.params.state);
  return ctx.ssh.exec({
    host: ctx.params.host,
    command: "systemctl",
    args: args
  });
}
