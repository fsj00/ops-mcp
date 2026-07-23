function execute(ctx) {
  return ctx.kafka.topic_offsets({
    kafka: ctx.params.kafka,
    topic: ctx.params.topic
  });
}
