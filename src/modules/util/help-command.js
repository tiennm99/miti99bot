/**
 * @file /help — renders public + protected commands grouped by module.
 *
 * /help is a pure renderer over the registry; it holds no command metadata of
 * its own. Modules with zero visible (public or protected) commands are
 * omitted entirely. Private commands are always skipped — that's the point.
 *
 * Output uses Telegram HTML parse mode. Every user-influenced substring
 * (module names, command descriptions) is HTML-escaped so a rogue `<script>`
 * in a description renders literally.
 */

import { escapeHtml } from "../../util/escape-html.js";
import { getCurrentRegistry } from "../registry.js";

/**
 * @typedef {import("../validate-command.js").ModuleCommand} ModuleCommand
 */

const REPO_URL = "https://github.com/tiennm99/miti99bot";
const SUPPORT_FOOTER = `Enjoying the bot? Support me by starring the repo: <a href="${REPO_URL}">${REPO_URL}</a>`;

/**
 * Pure render step — exported separately so tests can assert on the string
 * without instantiating a bot context.
 *
 * @param {import("../registry.js").Registry} reg
 * @returns {string}
 */
export function renderHelp(reg) {
  /** @type {Map<string, string[]>} */
  const byModule = new Map();

  const visibleMaps = [
    { map: reg.publicCommands, suffix: "" },
    { map: reg.protectedCommands, suffix: " (protected)" },
  ];

  for (const { map, suffix } of visibleMaps) {
    for (const entry of map.values()) {
      const modName = entry.module.name;
      const line = `/${entry.cmd.name} — ${escapeHtml(entry.cmd.description)}${suffix}`;
      const existing = byModule.get(modName);
      if (existing) existing.push(line);
      else byModule.set(modName, [line]);
    }
  }

  // Render in env.MODULES order (reg.modules is already in that order).
  const sections = [];
  for (const mod of reg.modules) {
    const lines = byModule.get(mod.name);
    if (!lines || lines.length === 0) continue;
    sections.push(`<b>${escapeHtml(mod.name)}</b>\n${lines.join("\n")}`);
  }

  const body = sections.length > 0 ? sections.join("\n\n") : "no commands registered";
  return `${body}\n\n${SUPPORT_FOOTER}`;
}

/** @type {ModuleCommand} */
export const helpCommand = {
  name: "help",
  visibility: "public",
  description: "Show all available commands",
  handler: async (ctx) => {
    const reg = getCurrentRegistry();
    const text = renderHelp(reg);
    await ctx.reply(text, { parse_mode: "HTML", link_preview_options: { is_disabled: true } });
  },
};

export default helpCommand;
