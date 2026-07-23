function execute(ctx) {
  return ctx.redis.dbsize({ redis: ctx.params.redis, db: ctx.params.db || 0 });
}
