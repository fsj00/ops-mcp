function execute(ctx) {
  return ctx.mysql.query({
    database: ctx.params.database,
    sql: ctx.params.sql
  });
}
