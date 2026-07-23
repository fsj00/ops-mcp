function execute(ctx) {
  return ctx.redis.hget({
    redis: ctx.params.redis,
    db: ctx.params.db || 0,
    key: ctx.params.key,
    field: ctx.params.field
  });
}
