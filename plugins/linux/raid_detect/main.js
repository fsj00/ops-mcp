function which(ctx, host, names) {
  var found = [];
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
          'if p=$(command -v "$bin" 2>/dev/null); then echo "$p"; fi; ' +
          "for d in /opt/MegaRAID/storcli /opt/MegaRAID/MegaCli /opt/MegaCli " +
          "/usr/sbin /usr/local/sbin /usr/local/bin " +
          "/opt/SmartStorageAdmin/ssacli/bin /usr/StorMan; do " +
          '  f="$d/$bin"; [ -x "$f" ] && echo "$f"; ' +
          "done"
      ]
    });
    var lines = String(r.stdout || "")
      .split("\n")
      .map(function (s) {
        return s.trim();
      })
      .filter(function (s) {
        return s.length > 0;
      });
    var seen = {};
    for (var j = 0; j < lines.length; j++) {
      if (!seen[lines[j]]) {
        seen[lines[j]] = true;
        found.push({ tool: name, path: lines[j] });
      }
    }
  }
  return found;
}

function execText(ctx, host, script) {
  var r = ctx.ssh.exec({
    host: host,
    command: "bash",
    args: ["-lc", script]
  });
  return {
    exit_code: r.exit_code,
    stdout: String(r.stdout || "").trim(),
    stderr: String(r.stderr || "").trim()
  };
}

function execute(ctx) {
  var host = ctx.params.host;
  var vendors = [];

  var storcli = which(ctx, host, ["storcli64", "storcli"]);
  if (storcli.length > 0) {
    vendors.push({
      vendor: "broadcom_lsi",
      brand: "Broadcom / LSI MegaRAID",
      tools: storcli,
      suggested_plugin: "linux_raid_storcli",
      notes: "优先使用 storcli；常见于 LSI / Broadcom MegaRAID 卡"
    });
  }

  var megacli = which(ctx, host, ["MegaCli64", "MegaCli", "megacli"]);
  if (megacli.length > 0) {
    vendors.push({
      vendor: "broadcom_lsi_legacy",
      brand: "Broadcom / LSI MegaRAID (legacy MegaCli)",
      tools: megacli,
      suggested_plugin: "linux_raid_megacli",
      notes: "旧版 MegaCli；若同时有 storcli，优先 linux_raid_storcli"
    });
  }

  var perccli = which(ctx, host, ["perccli64", "perccli"]);
  if (perccli.length > 0) {
    vendors.push({
      vendor: "dell_perc",
      brand: "Dell PERC",
      tools: perccli,
      suggested_plugin: "linux_raid_perccli",
      notes: "Dell PowerEdge 自带 PERC 管理工具（语法近 storcli）"
    });
  }

  var ssacli = which(ctx, host, ["ssacli", "hpssacli", "hpacucli"]);
  if (ssacli.length > 0) {
    vendors.push({
      vendor: "hpe_smart_array",
      brand: "HPE Smart Array",
      tools: ssacli,
      suggested_plugin: "linux_raid_ssacli",
      notes: "HPE/HP Smart Array；优先 ssacli，旧机可能为 hpssacli/hpacucli"
    });
  }

  var arcconf = which(ctx, host, ["arcconf"]);
  if (arcconf.length > 0) {
    vendors.push({
      vendor: "adaptec_microchip",
      brand: "Adaptec / Microchip",
      tools: arcconf,
      suggested_plugin: "linux_raid_arcconf",
      notes: "Adaptec / Microchip RAID 卡"
    });
  }

  var mdadm = which(ctx, host, ["mdadm"]);
  var mdstat = execText(ctx, host, "cat /proc/mdstat 2>/dev/null || true");
  var hasMd =
    mdadm.length > 0 ||
    (mdstat.stdout &&
      mdstat.stdout.indexOf("Personalities") >= 0 &&
      /md[0-9]+/.test(mdstat.stdout));
  if (hasMd) {
    vendors.push({
      vendor: "linux_md",
      brand: "Linux Software RAID (md)",
      tools: mdadm,
      suggested_plugin: "linux_raid_mdadm",
      notes: "内核软 RAID；可先看 /proc/mdstat，再用 mdadm --detail"
    });
  }

  var pci = execText(
    ctx,
    host,
    "lspci 2>/dev/null | grep -iE 'RAID|SAS|MegaRAID|Smart Array|PERC|Adaptec|AAC-RAID' || true"
  );

  var recommended = [];
  var seenRec = {};
  for (var i = 0; i < vendors.length; i++) {
    var p = vendors[i].suggested_plugin;
    if (!seenRec[p]) {
      seenRec[p] = true;
      recommended.push(p);
    }
  }

  return {
    host: host,
    workflow:
      "先调用 linux_raid_detect，再按 recommended / suggested_plugin 选择对应只读 Plugin（勿混用厂商 CLI）",
    pci_raid_hints: pci.stdout || "",
    mdstat: mdstat.stdout || "",
    vendors: vendors,
    recommended: recommended,
    count: vendors.length
  };
}
