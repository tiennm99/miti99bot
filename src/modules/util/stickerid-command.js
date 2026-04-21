/**
 * @file /stickerid — dev helper that returns the file_id of a replied sticker.
 *
 * Telegram sticker file_ids are bot-scoped: a file_id obtained from any other
 * bot will not work with sendSticker for this bot. To collect IDs for use in
 * `/loldle` congrats/lose/giveup pools, send a sticker to the bot, reply to
 * it with `/stickerid`, then copy the returned file_id into code.
 *
 * Private visibility — hidden from the Telegram `/` menu and from `/help`.
 */

import { escapeHtml } from "../../util/escape-html.js";

/** @type {import("../validate-command.js").ModuleCommand} */
export const stickerIdCommand = {
  name: "stickerid",
  visibility: "private",
  description: "Reply to a sticker with this command to get its bot-scoped file_id",
  handler: async (ctx) => {
    const sticker = ctx.message?.reply_to_message?.sticker;
    if (!sticker) {
      return ctx.reply(
        "Reply to a sticker message with /stickerid to get its file_id.\n" +
          "Usage: send a sticker to me, then tap Reply on it and type /stickerid.",
      );
    }

    const setName = sticker.set_name ?? "(no set)";
    const lines = [
      "<b>file_id</b>",
      `<code>${escapeHtml(sticker.file_id)}</code>`,
      "",
      "<b>file_unique_id</b>",
      `<code>${escapeHtml(sticker.file_unique_id)}</code>`,
      "",
      `set: ${escapeHtml(setName)} · emoji: ${escapeHtml(sticker.emoji ?? "—")}`,
    ];
    await ctx.reply(lines.join("\n"), { parse_mode: "HTML" });
  },
};

export default stickerIdCommand;
