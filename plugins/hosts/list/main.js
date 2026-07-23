function execute(ctx) {
  var hosts = ctx.hosts.list();
  return {
    hosts: hosts,
    count: hosts.length
  };
}
