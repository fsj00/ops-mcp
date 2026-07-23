function execute(ctx) {
  return ctx.redis.info({
    redis: ctx.params.redis,
    db: ctx.params.db || 0,
    section: ctx.params.section
  });
}
