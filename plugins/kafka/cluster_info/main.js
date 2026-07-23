function execute(ctx) {
  return ctx.kafka.cluster_info({ kafka: ctx.params.kafka });
}
