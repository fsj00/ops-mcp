function execute(ctx) {
  return ctx.redis.zrange_sample({
    redis: ctx.params.redis,
    db: ctx.params.db || 0,
    key: ctx.params.key,
    limit: ctx.params.limit
  });
}
