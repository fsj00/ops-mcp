function execute(ctx) {
  var opts = { host: ctx.params.host };
  if (ctx.params.all === true) {
    opts.all = true;
  }
  return ctx.docker.ps(opts);
}
