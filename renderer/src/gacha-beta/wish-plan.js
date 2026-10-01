/**
 * Plan for the beta wish video: what the pack-cards page shows for a request,
 * when the scripted drag tears the pack open, and which files the page may
 * load. Kept free of Chrome so it can be unit tested.
 */

import path from 'node:path';
import {getRankLetter, getRarityPalette} from '../remotion/gacha-timeline.js';
import {gachaFrameSizes} from '../schemas/gacha-request.js';

/** @typedef {import('../schemas/gacha-request.js').GachaRenderRequest} GachaRenderRequest */
/** @typedef {import('../remotion/gacha-timeline.js').GachaRarity} GachaRarity */

/** Origin the page is served from; every request to it is answered from disk. */
export const wishOrigin = 'https://gacha.wish';

export const gachaBetaTotalSeconds = 6;

/**
 * Seconds into the clip. The pack waits briefly, then a drag across its seal
 * tears it open; pack-cards plays the rest of the opening and the card's
 * arrival on its own.
 */
export const wishTimeline = Object.freeze({
  dragStart: 0.5,
  dragEnd: 1,
});

/**
 * The beta wish is portrait: the request's `width` (640 or 854) is the long
 * edge, so the frame is the /api/gacha frame turned upright.
 *
 * @param {GachaRenderRequest['width']} width
 * @returns {{width: number, height: number}}
 */
export const getWishFrameSize = (width) => ({width: gachaFrameSizes[width], height: width});

/** pack-cards rarity profile and glow preset for each requested rarity. */
const rarityLooks = Object.freeze({
  3: {rarity: 'rare', glow: /** @type {const} */ ('silver')},
  4: {rarity: 'epic', glow: /** @type {const} */ ('platinum')},
  5: {rarity: 'legendary', glow: /** @type {const} */ ('gold')},
});

/**
 * @typedef {object} WishPageProps
 * @property {string} label
 * @property {'B' | 'A' | 'S'} rank  Same rank letter as /api/gacha.
 * @property {GachaRarity} stars
 * @property {string} rarity
 * @property {'silver' | 'platinum' | 'gold'} glow
 * @property {{accent: string, tint: string}} artwork  Rarity colours: blue, purple, gold.
 */

/**
 * @param {GachaRenderRequest} request
 * @returns {WishPageProps}
 */
export const getWishPageProps = (request) => {
  const palette = getRarityPalette(request.rarity);
  return {
    label: request.label,
    rank: getRankLetter(request.rarity),
    stars: request.rarity,
    ...rarityLooks[request.rarity],
    artwork: {accent: palette.glow, tint: palette.deep},
  };
};

/**
 * Pointer position for a frame during the drag, as fractions of the pack
 * control's box, or null outside it. The button is held while the pointer
 * sweeps across the seal.
 *
 * @param {number} seconds
 * @returns {{x: number, y: number} | null}
 */
export const getDragPointer = (seconds) => {
  const {dragStart, dragEnd} = wishTimeline;
  if (seconds < dragStart || seconds > dragEnd) {
    return null;
  }
  return {x: 0.08 + 0.9 * ((seconds - dragStart) / (dragEnd - dragStart)), y: 0.12};
};

const contentTypes = Object.freeze({
  '.html': 'text/html; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.webp': 'image/webp',
  '.json': 'application/json; charset=utf-8',
  '.woff2': 'font/woff2',
});

/**
 * Maps a page request to a file inside one of the allowed roots. Requests
 * under `/pack-cards/` read the installed package; everything else reads the
 * page directory. Returns null for other origins, unknown file types, and any
 * path that would leave its root.
 *
 * @param {string} url
 * @param {{page: string, packCards: string}} roots  Absolute directories.
 * @returns {{file: string, contentType: string} | null}
 */
export const resolveWishAsset = (url, roots) => {
  let parsed;
  try {
    parsed = new URL(url);
  } catch {
    return null;
  }
  if (parsed.origin !== wishOrigin) {
    return null;
  }
  let pathname;
  try {
    pathname = decodeURIComponent(parsed.pathname);
  } catch {
    return null;
  }
  const [root, relative] = pathname.startsWith('/pack-cards/')
    ? [roots.packCards, pathname.slice('/pack-cards/'.length)]
    : [roots.page, pathname.slice(1)];
  const file = path.resolve(root, relative || 'index.html');
  const contentType = contentTypes[/** @type {keyof typeof contentTypes} */ (path.extname(file))];
  if (!file.startsWith(root + path.sep) || !contentType) {
    return null;
  }
  return {file, contentType};
};
