/**
 * @file util module — /info and /help.
 *
 * The only "fully implemented" module in v1. /help is a pure renderer over
 * the current registry, so it has no module-specific state. /info just reads
 * the grammY context.
 */

import { helpCommand } from "./help-command.js";
import { infoCommand } from "./info-command.js";

/** @type {import("../registry.js").BotModule} */
const utilModule = {
  name: "util",
  commands: [infoCommand, helpCommand],
};

export default utilModule;
