function execute(ctx) {
  var parts = ["dmesg", "--color=never"];
  if (ctx.params.human !== false) {
    parts.push("-T");
  }
  if (ctx.params.level) {
    parts.push("--level=" + JSON.stringify(String(ctx.params.level)));
  }
  var cmd = parts.join(" ");
  if (ctx.params.lines) {
    var n = parseInt(ctx.params.lines, 10) || 100;
    cmd += " | tail -n " + n;
  }
  return ctx.ssh.exec({
    host: ctx.params.host,
    command: "bash",
    args: ["-lc", cmd]
  });
}
