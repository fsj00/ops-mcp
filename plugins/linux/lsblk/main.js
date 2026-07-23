function execute(ctx) {
  return ctx.ssh.exec({
    host: ctx.params.host,
    command: "lsblk",
    args: ["-o", "NAME,SIZE,TYPE,FSTYPE,MOUNTPOINT,UUID"]
  });
}
