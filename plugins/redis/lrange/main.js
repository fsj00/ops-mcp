function execute(ctx) {
  return ctx.redis.lrange({
    redis: ctx.params.redis,
    db: ctx.params.db || 0,
    key: ctx.params.key,
    start: ctx.params.start || 0,
    limit: ctx.params.limit
  });
}
