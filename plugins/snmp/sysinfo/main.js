function execute(ctx) {
  var oids = [
    "1.3.6.1.2.1.1.1.0", // sysDescr
    "1.3.6.1.2.1.1.2.0", // sysObjectID
    "1.3.6.1.2.1.1.3.0", // sysUpTime
    "1.3.6.1.2.1.1.4.0", // sysContact
    "1.3.6.1.2.1.1.5.0", // sysName
    "1.3.6.1.2.1.1.6.0"  // sysLocation
  ];
  var names = {
    "1.3.6.1.2.1.1.1.0": "sysDescr",
    "1.3.6.1.2.1.1.2.0": "sysObjectID",
    "1.3.6.1.2.1.1.3.0": "sysUpTime",
    "1.3.6.1.2.1.1.4.0": "sysContact",
    "1.3.6.1.2.1.1.5.0": "sysName",
    "1.3.6.1.2.1.1.6.0": "sysLocation"
  };
  var res = ctx.snmp.get({ device: ctx.params.device, oids: oids });
  var info = {};
  for (var i = 0; i < res.vars.length; i++) {
    var v = res.vars[i];
    var key = names[v.oid] || v.oid;
    info[key] = v.value;
  }
  return {
    device: res.device,
    sysinfo: info,
    vars: res.vars
  };
}
