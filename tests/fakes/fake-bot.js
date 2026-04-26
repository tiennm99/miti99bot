/**
 * @file fake-bot — records bot.command() calls for dispatcher tests.
 *
 * We never import real grammY in unit tests — everything the dispatcher
 * touches on the bot object is recorded here for assertions.
 */

export function makeFakeBot() {
  /** @type {Array<{name: string, handler: Function}>} */
  const commandCalls = [];
  /** @type {Array<{event: string, handler: Function}>} */
  const onCalls = [];
  /** @type {Array<Function>} */
  const useCalls = [];

  return {
    commandCalls,
    onCalls,
    useCalls,
    command(name, handler) {
      commandCalls.push({ name, handler });
    },
    on(event, handler) {
      onCalls.push({ event, handler });
    },
    /** Records middleware registered via bot.use() (e.g. timing middleware). */
    use(middleware) {
      useCalls.push(middleware);
    },
  };
}
