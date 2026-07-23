function execute(ctx) {
  return ctx.docker.info({ host: ctx.params.host });
}
