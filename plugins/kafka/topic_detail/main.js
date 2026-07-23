function execute(ctx) {
  return ctx.kafka.topic_detail({
    kafka: ctx.params.kafka,
    topic: ctx.params.topic
  });
}
