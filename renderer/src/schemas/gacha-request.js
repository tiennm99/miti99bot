import {z} from 'zod';

export const maxGachaSeed = 2 ** 31 - 1;

/** Landscape frame sizes; heights keep 16:9 rounded to even pixels for H.264. */
export const gachaFrameSizes = Object.freeze({640: 360, 854: 480});

/**
 * @typedef {object} GachaRequestLimits
 * @property {number} maxOptionChars
 */

/**
 * @typedef {object} GachaRenderRequest
 * @property {string} label
 * @property {3 | 4 | 5} rarity
 * @property {24 | 30} fps
 * @property {640 | 854} width
 * @property {number} seed  Lays out the stars and particles for this roll.
 */

/**
 * @param {GachaRequestLimits} limits
 */
export const createGachaRequestSchema = (limits) =>
  z
    .object({
      label: z
        .string()
        .transform((label) => label.trim())
        .pipe(z.string().min(1).max(limits.maxOptionChars)),
      rarity: z.union([z.literal(3), z.literal(4), z.literal(5)]),
      fps: z.union([z.literal(24), z.literal(30)]).default(24),
      width: z.union([z.literal(640), z.literal(854)]).default(640),
      seed: z.number().int().min(0).max(maxGachaSeed).optional(),
    })
    .strict();

/**
 * Parses a request, picking a fresh seed when the caller sends none so every
 * roll draws a different sky.
 *
 * @param {unknown} input
 * @param {GachaRequestLimits} limits
 * @param {() => number} pickSeed
 * @returns {GachaRenderRequest}
 */
export const parseGachaRequest = (input, limits, pickSeed) => {
  const parsed = createGachaRequestSchema(limits).parse(input);
  return {...parsed, seed: parsed.seed ?? pickSeed()};
};
