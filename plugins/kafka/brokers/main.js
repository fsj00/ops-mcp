function execute(ctx) {
  return ctx.kafka.brokers({ kafka: ctx.params.kafka });
}
