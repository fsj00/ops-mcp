function execute(ctx) {
  return ctx.redis.memory_usage({ redis: ctx.params.redis, db: ctx.params.db || 0, key: ctx.params.key });
}
