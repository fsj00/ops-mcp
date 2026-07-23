function execute(ctx) {
  var parts = ["journalctl", "--no-pager"];
  if (ctx.params.boot === true) parts.push("-b");
  if (ctx.params.unit) parts.push("-u", JSON.stringify(String(ctx.params.unit)));
  if (ctx.params.priority) parts.push("-p", JSON.stringify(String(ctx.params.priority)));
  if (ctx.params.since) parts.push("--since", JSON.stringify(String(ctx.params.since)));
  if (ctx.params.grep) parts.push("-g", JSON.stringify(String(ctx.params.grep)));
  var n = ctx.params.lines || "100";
  parts.push("-n", String(n));
  return ctx.ssh.exec({
    host: ctx.params.host,
    command: "bash",
    args: ["-lc", parts.join(" ")]
  });
}
