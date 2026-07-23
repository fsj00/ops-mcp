function resolveBin(ctx, host, preferred) {
  var names = preferred
    ? [preferred]
    : ["storcli64", "storcli"];
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
          "for d in /opt/MegaRAID/storcli /usr/sbin /usr/local/bin; do " +
          '  f="$d/$bin"; [ -x "$f" ] && echo "$f" && exit 0; ' +
          "done; exit 1"
      ]
    });
    if (r.exit_code === 0 && String(r.stdout || "").trim()) {
      return String(r.stdout).trim().split("\n")[0];
    }
  }
  throw new Error(
    "storcli/storcli64 not found; run linux_raid_detect first"
  );
}

function execute(ctx) {
  var host = ctx.params.host;
  var action = String(ctx.params.action || "summary").toLowerCase();
  var c = ctx.params.controller != null ? String(ctx.params.controller) : "ALL";
  if (!/^[A-Za-z0-9]+$/.test(c)) {
    throw new Error("invalid controller: use digits or ALL");
  }
  var bin = resolveBin(ctx, host, ctx.params.binary);
  var suffix;
  switch (action) {
    case "summary":
      suffix = "/c" + c + " show";
      break;
    case "controllers":
      suffix = "show ctrlcount";
      break;
    case "virtual_drives":
      suffix = "/c" + c + "/vall show";
      break;
    case "physical_drives":
      suffix = "/c" + c + "/eall/sall show";
      break;
    case "all":
      suffix = "/c" + c + " show all";
      break;
    default:
      throw new Error(
        "unsupported action: use summary|controllers|virtual_drives|physical_drives|all"
      );
  }
  return ctx.ssh.exec({
    host: host,
    command: "bash",
    args: ["-lc", JSON.stringify(bin) + " " + suffix]
  });
}
