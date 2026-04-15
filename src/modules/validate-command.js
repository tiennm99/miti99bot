/**
 * @file validate-command — shared validators for module-registered commands.
 *
 * Enforces the contract defined in phase-04 of the plan:
 *   - visibility ∈ {public, protected, private}
 *   - name matches /^[a-z0-9_]{1,32}$/ (Telegram's slash-command limit; applied
 *     uniformly across all visibility levels because private commands are also
 *     slash commands, just hidden from the menu and /help)
 *   - description is non-empty, ≤256 chars (Telegram's setMyCommands limit)
 *   - handler is a function
 *
 * All errors include the module and command name for debuggability.
 */

export const VISIBILITIES = Object.freeze(["public", "protected", "private"]);
export const COMMAND_NAME_RE = /^[a-z0-9_]{1,32}$/;
export const MAX_DESCRIPTION_LENGTH = 256;

/**
 * @typedef {object} ModuleCommand
 * @property {string} name — without leading slash; matches COMMAND_NAME_RE.
 * @property {"public"|"protected"|"private"} visibility
 * @property {string} description — ≤256 chars; required for all visibilities.
 * @property {(ctx: any) => Promise<void>|void} handler
 */

/**
 * Throws on any contract violation. Called once per command at registry build.
 *
 * @param {ModuleCommand} cmd
 * @param {string} moduleName — for error messages.
 */
export function validateCommand(cmd, moduleName) {
  const prefix = `module "${moduleName}" command`;

  if (!cmd || typeof cmd !== "object") {
    throw new Error(`${prefix}: command entry is not an object`);
  }

  // visibility
  if (!VISIBILITIES.includes(cmd.visibility)) {
    throw new Error(
      `${prefix} "${cmd.name}": visibility must be one of ${VISIBILITIES.join("|")}, got "${cmd.visibility}"`,
    );
  }

  // name
  if (typeof cmd.name !== "string") {
    throw new Error(`${prefix}: name must be a string`);
  }
  if (cmd.name.startsWith("/")) {
    throw new Error(`${prefix} "${cmd.name}": name must not start with "/"`);
  }
  if (!COMMAND_NAME_RE.test(cmd.name)) {
    throw new Error(
      `${prefix} "${cmd.name}": name must match ${COMMAND_NAME_RE} (lowercase letters, digits, underscore; 1–32 chars)`,
    );
  }

  // description — required for all visibilities (private commands need it for internal debugging)
  if (typeof cmd.description !== "string" || cmd.description.length === 0) {
    throw new Error(`${prefix} "${cmd.name}": description is required`);
  }
  if (cmd.description.length > MAX_DESCRIPTION_LENGTH) {
    throw new Error(
      `${prefix} "${cmd.name}": description exceeds ${MAX_DESCRIPTION_LENGTH} chars (got ${cmd.description.length})`,
    );
  }

  // handler
  if (typeof cmd.handler !== "function") {
    throw new Error(`${prefix} "${cmd.name}": handler must be a function`);
  }
}
