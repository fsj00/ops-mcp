function execute(ctx) {
  var databases = ctx.databases.list();
  return {
    databases: databases,
    count: databases.length
  };
}
