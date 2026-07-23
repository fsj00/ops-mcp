function execute(ctx) {
  return ctx.kafka.consumer_lag_summary({
    kafka: ctx.params.kafka,
    group: ctx.params.group,
    limit: ctx.params.limit
  });
}
