function execute(ctx) {
  return ctx.docker.history({
    host: ctx.params.host,
    image: ctx.params.image
  });
}
