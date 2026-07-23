function execute(ctx) {
  var host = ctx.params.host;
  var action = String(ctx.params.action || "mdstat").toLowerCase();

  if (action === "mdstat") {
    return ctx.ssh.exec({
      host: host,
      command: "cat",
      args: ["/proc/mdstat"]
    });
  }

  if (action === "scan") {
    return ctx.ssh.exec({
      host: host,
      command: "mdadm",
      args: ["--detail", "--scan"]
    });
  }

  if (action === "examine") {
    return ctx.ssh.exec({
      host: host,
      command: "mdadm",
      args: ["--examine", "--scan"]
    });
  }

  if (action === "detail") {
    var device = ctx.params.device;
    if (!device) {
      throw new Error("device is required for action=detail (e.g. /dev/md0)");
    }
    device = String(device);
    if (!/^\/dev\/md[0-9a-zA-Z_/-]+$/.test(device)) {
      throw new Error("invalid device: expect /dev/mdN");
    }
    return ctx.ssh.exec({
      host: host,
      command: "mdadm",
      args: ["--detail", device]
    });
  }

  throw new Error("unsupported action: use mdstat|scan|detail|examine");
}
