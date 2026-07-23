function execute(ctx) {
  return ctx.kafka.consumer_groups({
    kafka: ctx.params.kafka,
    limit: ctx.params.limit
  });
}
