function execute(ctx) {
  var devices = ctx.snmp_devices.list({
    labels: ctx.params.labels || {},
    limit: ctx.params.limit || 0,
    offset: ctx.params.offset || 0
  });
  return {
    devices: devices,
    count: devices.length
  };
}
