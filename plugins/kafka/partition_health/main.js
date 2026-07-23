function execute(ctx) {
  return ctx.kafka.partition_health({
    kafka: ctx.params.kafka,
    topic: ctx.params.topic
  });
}
