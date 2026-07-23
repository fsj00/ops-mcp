function execute(ctx) {
  return ctx.redis.zcard({ redis: ctx.params.redis, db: ctx.params.db || 0, key: ctx.params.key });
}
