function execute(ctx) {
  return ctx.redis.client_list({
    redis: ctx.params.redis,
    db: ctx.params.db || 0,
    limit: ctx.params.limit
  });
}
