function resolveBin(ctx, host, preferred) {
  var names = preferred
    ? [preferred]
    : ["MegaCli64", "MegaCli", "megacli"];
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
          "for d in /opt/MegaRAID/MegaCli /opt/MegaCli /usr/sbin /usr/local/bin; do " +
          '  f="$d/$bin"; [ -x "$f" ] && echo "$f" && exit 0; ' +
          "done; exit 1"
      ]
    });
    if (r.exit_code === 0 && String(r.stdout || "").trim()) {
      return String(r.stdout).trim().split("\n")[0];
    }
  }
  throw new Error(
    "MegaCli not found; run linux_raid_detect first"
  );
}

function execute(ctx) {
  var host = ctx.params.host;
  var action = String(ctx.params.action || "adapter").toLowerCase();
  var bin = resolveBin(ctx, host, ctx.params.binary);
  var args;
  switch (action) {
    case "adapter":
      args = "-AdpAllInfo -aAll";
      break;
    case "virtual_drives":
      args = "-LDInfo -Lall -aAll";
      break;
    case "physical_drives":
      args = "-PDList -aAll";
      break;
    case "bbu":
      args = "-AdpBbuCmd -aAll";
      break;
    case "config":
      args = "-CfgDsply -aAll";
      break;
    default:
      throw new Error(
        "unsupported action: use adapter|virtual_drives|physical_drives|bbu|config"
      );
  }
  return ctx.ssh.exec({
    host: host,
    command: "bash",
    args: ["-lc", JSON.stringify(bin) + " " + args]
  });
}
