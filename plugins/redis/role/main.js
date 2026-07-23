function execute(ctx) {
  return ctx.redis.role({ redis: ctx.params.redis, db: ctx.params.db || 0 });
}
