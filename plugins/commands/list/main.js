function execute(ctx) {
  var commands = ctx.commands.list();
  return {
    commands: commands,
    count: commands.length
  };
}
