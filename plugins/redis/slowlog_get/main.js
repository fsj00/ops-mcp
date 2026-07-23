function execute(ctx) {
  return ctx.redis.slowlog_get({
    redis: ctx.params.redis,
    db: ctx.params.db || 0,
    count: ctx.params.count
  });
}
