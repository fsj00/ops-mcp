function execute(ctx) {
  return ctx.postgres.version({
    database: ctx.params.database
  });
}
