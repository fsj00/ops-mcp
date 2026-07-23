function execute(ctx) {
  return ctx.redis.exists({ redis: ctx.params.redis, db: ctx.params.db || 0, key: ctx.params.key });
}
