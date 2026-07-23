function execute(ctx) {
  var opts = {
    host: ctx.params.host,
    container: ctx.params.container
  };
  if (ctx.params.ps_args) {
    opts.ps_args = ctx.params.ps_args;
  }
  return ctx.docker.top(opts);
}
