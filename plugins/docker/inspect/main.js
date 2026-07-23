function execute(ctx) {
  return ctx.docker.inspect({
    host: ctx.params.host,
    target: ctx.params.target
  });
}
