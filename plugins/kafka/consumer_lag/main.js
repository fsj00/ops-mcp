function execute(ctx) {
  return ctx.kafka.consumer_lag({
    kafka: ctx.params.kafka,
    group: ctx.params.group,
    topic: ctx.params.topic
  });
}
