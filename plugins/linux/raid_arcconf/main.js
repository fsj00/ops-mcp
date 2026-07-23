function resolveBin(ctx, host) {
  var r = ctx.ssh.exec({
    host: host,
    command: "bash",
    args: [
      "-lc",
      'if p=$(command -v arcconf 2>/dev/null); then echo "$p"; exit 0; fi; ' +
        "for d in /usr/StorMan /usr/sbin /usr/local/bin; do " +
        '  f="$d/arcconf"; [ -x "$f" ] && echo "$f" && exit 0; ' +
        "done; exit 1"
    ]
  });
  if (r.exit_code === 0 && String(r.stdout || "").trim()) {
    return String(r.stdout).trim().split("\n")[0];
  }
  throw new Error("arcconf not found; run linux_raid_detect first");
}

function execute(ctx) {
  var host = ctx.params.host;
  var action = String(ctx.params.action || "list").toLowerCase();
  var c = ctx.params.controller != null ? String(ctx.params.controller) : "1";
  if (!/^[0-9]+$/.test(c)) {
    throw new Error("invalid controller: use digits");
  }
  var bin = resolveBin(ctx, host);
  var suffix;
  switch (action) {
    case "list":
      suffix = "LIST";
      break;
    case "config":
      suffix = "GETCONFIG " + c;
      break;
    case "status":
      suffix = "GETSTATUS " + c;
      break;
    case "version":
      suffix = "GETVERSION";
      break;
    default:
      throw new Error(
        "unsupported action: use list|config|status|version"
      );
  }
  return ctx.ssh.exec({
    host: host,
    command: "bash",
    args: ["-lc", JSON.stringify(bin) + " " + suffix]
  });
}
