function execute(ctx) {
  return ctx.snmp.walk({
    device: ctx.params.device,
    oid: ctx.params.oid,
    max_oids: ctx.params.max_oids || 0
  });
}
