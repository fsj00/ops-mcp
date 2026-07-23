function execute(ctx) {
  return ctx.mysql.version({
    database: ctx.params.database
  });
}
