/**
 * @file Loldle sticker pools — keyed by round outcome.
 *
 * Telegram `file_id`s are bot-scoped: these IDs are only valid for @miti99bot.
 * To add/replace stickers, DM a sticker to the bot and use `/stickerid` (util
 * module, private) to capture the bot-scoped file_id, then paste it here.
 *
 * Empty pools are safe — `pickSticker` returns null and the handler skips the
 * replyWithSticker call entirely.
 */

/** Cheerful stickers shown before a win message. */
export const WIN_STICKERS = [
  "CAACAgIAAxkBAAECGBVp56vuLw5TSWy8TiFGkQYAAWxi0wUAAh9EAAJTD8BJ2wQ6vyMilxs7BA",
  "CAACAgIAAxkBAAECGBhp56wAATkzsBBcIxMSC_8mp2dBczsAAvBEAAL1vklJuCnu695nAvI7BA",
  "CAACAgIAAxkBAAECGBlp56wBZ4rmluL3KNlOuWsyctN0FQACNEQAAue5OEv37J_IfMnpljsE",
  "CAACAgIAAxkBAAECGCFp56w7WlD6vTIsHE2WUTs4C2IjXAAC9FsAAtgFMUu63usfH16ZpzsE",
];

/** Deflated stickers shown when the player runs out of guesses. */
export const LOSE_STICKERS = [
  "CAACAgIAAxkBAAECGCBp56w3HUyYOHOeMwLfULT8p8SPtQACWkYAAiPesEjnptWk36YKZjsE",
];

/** Resigned stickers shown on /loldle_giveup. */
export const GIVEUP_STICKERS = [
  "CAACAgIAAxkBAAECGCJp56xIk6B6McSfnYykLhgXVCSnmQACBlkAApSyWEo6G2rnqDvZxjsE",
];

/**
 * Pick a random sticker from a pool, or null if the pool is empty.
 * @param {readonly string[]} pool
 * @returns {string|null}
 */
export function pickSticker(pool) {
  if (!pool || pool.length === 0) return null;
  return pool[Math.floor(Math.random() * pool.length)];
}
