function execute(ctx) {
  return ctx.kafka.topics({
    kafka: ctx.params.kafka,
    prefix: ctx.params.prefix,
    limit: ctx.params.limit,
    include_internal: ctx.params.include_internal === true
  });
}
