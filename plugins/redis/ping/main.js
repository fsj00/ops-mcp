function execute(ctx) {
  return ctx.redis.ping({ redis: ctx.params.redis, db: ctx.params.db || 0 });
}
