function execute(ctx) {
  var kafka = ctx.kafka.list();
  return {
    kafka: kafka,
    count: kafka.length
  };
}
