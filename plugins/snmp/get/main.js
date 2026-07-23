function execute(ctx) {
  return ctx.snmp.get({
    device: ctx.params.device,
    oids: ctx.params.oids
  });
}
