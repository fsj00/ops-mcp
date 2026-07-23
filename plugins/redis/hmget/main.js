function execute(ctx) {
  return ctx.redis.hmget({
    redis: ctx.params.redis,
    db: ctx.params.db || 0,
    key: ctx.params.key,
    fields: ctx.params.fields
  });
}
