function execute(ctx) {
  return ctx.snmp.bulk({
    device: ctx.params.device,
    oid: ctx.params.oid,
    max_oids: ctx.params.max_oids || 0,
    max_repetitions: ctx.params.max_repetitions || 0
  });
}
