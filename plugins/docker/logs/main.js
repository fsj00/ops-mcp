function execute(ctx) {
  var opts = {
    host: ctx.params.host,
    container: ctx.params.container
  };
  if (ctx.params.tail) opts.tail = ctx.params.tail;
  if (ctx.params.since) opts.since = ctx.params.since;
  if (ctx.params.timestamps === true) opts.timestamps = true;
  return ctx.docker.logs(opts);
}
