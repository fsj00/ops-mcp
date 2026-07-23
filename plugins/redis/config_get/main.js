function execute(ctx) {
  return ctx.redis.config_get({
    redis: ctx.params.redis,
    db: ctx.params.db || 0,
    pattern: ctx.params.pattern
  });
}
