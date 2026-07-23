function execute(ctx) {
  var redis = ctx.redis.list();
  return {
    redis: redis,
    count: redis.length
  };
}
