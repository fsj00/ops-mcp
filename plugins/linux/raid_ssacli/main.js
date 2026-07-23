function resolveBin(ctx, host, preferred) {
  var names = preferred
    ? [preferred]
    : ["ssacli", "hpssacli", "hpacucli"];
  for (var i = 0; i < names.length; i++) {
    var name = names[i];
    var r = ctx.ssh.exec({
      host: host,
      command: "bash",
      args: [
        "-lc",
        "bin=" +
          JSON.stringify(name) +
          "; " +
          'if p=$(command -v "$bin" 2>/dev/null); then echo "$p"; exit 0; fi; ' +
          "for d in /usr/sbin /opt/SmartStorageAdmin/ssacli/bin /usr/local/bin; do " +
          '  f="$d/$bin"; [ -x "$f" ] && echo "$f" && exit 0; ' +
          "done; exit 1"
      ]
    });
    if (r.exit_code === 0 && String(r.stdout || "").trim()) {
      return String(r.stdout).trim().split("\n")[0];
    }
  }
  throw new Error(
    "ssacli/hpssacli not found; run linux_raid_detect first"
  );
}

function execute(ctx) {
  var host = ctx.params.host;
  var action = String(ctx.params.action || "summary").toLowerCase();
  var bin = resolveBin(ctx, host, ctx.params.binary);
  var ctrl = "all";
  if (ctx.params.slot != null && String(ctx.params.slot) !== "") {
    var slot = String(ctx.params.slot);
    if (!/^[0-9]+$/.test(slot)) {
      throw new Error("invalid slot: use digits");
    }
    ctrl = "slot=" + slot;
  }
  var suffix;
  switch (action) {
    case "summary":
      suffix = "ctrl " + ctrl + " show";
      break;
    case "status":
      suffix = "ctrl " + ctrl + " show status";
      break;
    case "config":
      suffix = "ctrl " + ctrl + " show config";
      break;
    case "detail":
      suffix = "ctrl " + ctrl + " show detail";
      break;
    default:
      throw new Error(
        "unsupported action: use summary|status|config|detail"
      );
  }
  return ctx.ssh.exec({
    host: host,
    command: "bash",
    args: ["-lc", JSON.stringify(bin) + " " + suffix]
  });
}
