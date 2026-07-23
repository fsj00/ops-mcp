function execute(ctx) {
  return ctx.kafka.broker_config({
    kafka: ctx.params.kafka,
    broker_id: ctx.params.broker_id,
    prefix: ctx.params.prefix
  });
}
