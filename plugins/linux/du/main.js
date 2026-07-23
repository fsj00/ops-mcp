function execute(ctx) {
  var depth = ctx.params.max_depth || "1";
  return ctx.ssh.exec({
    host: ctx.params.host,
    command: "du",
    args: ["-h", "--max-depth=" + String(depth), ctx.params.path]
  });
}
