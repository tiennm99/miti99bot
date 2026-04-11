/**
 * @file /info — debug helper that echoes chat id, thread id, and sender id.
 *
 * Plain text reply (no parse mode). `message_thread_id` is only present for
 * forum-topic messages, so it's optional with a "n/a" fallback so debug users
 * know the field was checked.
 */

/** @type {import("../validate-command.js").ModuleCommand} */
export const infoCommand = {
  name: "info",
  visibility: "public",
  description: "Show chat id, thread id, and sender id (debug helper)",
  handler: async (ctx) => {
    const chatId = ctx.chat?.id ?? "n/a";
    const threadId = ctx.message?.message_thread_id ?? "n/a";
    const senderId = ctx.from?.id ?? "n/a";
    await ctx.reply(`chat id: ${chatId}\nthread id: ${threadId}\nsender id: ${senderId}`);
  },
};

export default infoCommand;
