function execute(ctx) {
  var device = ctx.params.device;
  var max = ctx.params.max_oids || 0;
  // IF-MIB columns under ifEntry / ifXEntry
  var columns = [
    { oid: "1.3.6.1.2.1.2.2.1.2", field: "ifDescr" },
    { oid: "1.3.6.1.2.1.2.2.1.3", field: "ifType" },
    { oid: "1.3.6.1.2.1.2.2.1.5", field: "ifSpeed" },
    { oid: "1.3.6.1.2.1.2.2.1.7", field: "ifAdminStatus" },
    { oid: "1.3.6.1.2.1.2.2.1.8", field: "ifOperStatus" },
    { oid: "1.3.6.1.2.1.31.1.1.1.18", field: "ifAlias" }
  ];
  var byIndex = {};
  var truncated = false;
  for (var c = 0; c < columns.length; c++) {
    var col = columns[c];
    var walk = ctx.snmp.bulk({ device: device, oid: col.oid, max_oids: max });
    if (walk.truncated) truncated = true;
    for (var i = 0; i < walk.vars.length; i++) {
      var v = walk.vars[i];
      var idx = v.oid.substring(col.oid.length + 1); // after "oid."
      if (!byIndex[idx]) {
        byIndex[idx] = { ifIndex: idx };
      }
      byIndex[idx][col.field] = v.value;
    }
  }
  var interfaces = [];
  for (var k in byIndex) {
    if (Object.prototype.hasOwnProperty.call(byIndex, k)) {
      interfaces.push(byIndex[k]);
    }
  }
  interfaces.sort(function (a, b) {
    return Number(a.ifIndex) - Number(b.ifIndex);
  });
  return {
    device: device,
    interfaces: interfaces,
    count: interfaces.length,
    truncated: truncated
  };
}
