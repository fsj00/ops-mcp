function execute(ctx) {
  var opts = { host: ctx.params.host };
  if (ctx.params.container) {
    opts.container = ctx.params.container;
  }
  return ctx.docker.stats(opts);
}
