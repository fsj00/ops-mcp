function execute(ctx) {
  return ctx.redis.hscan({
    redis: ctx.params.redis,
    db: ctx.params.db || 0,
    key: ctx.params.key,
    cursor: ctx.params.cursor || 0,
    match: ctx.params.match,
    limit: ctx.params.limit
  });
}
