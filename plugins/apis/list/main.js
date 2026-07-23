function execute(ctx) {
  var apis = ctx.apis.list();
  return {
    apis: apis,
    count: apis.length
  };
}
