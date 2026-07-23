function execute(ctx) {
  return ctx.postgres.query({
    database: ctx.params.database,
    sql: ctx.params.sql
  });
}
