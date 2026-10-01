import {z} from 'zod';

export const maxGachaSeed = 2 ** 31 - 1;

/**
 * @typedef {object} GachaRequestLimits
 * @property {number} maxOptionChars
 */

/**
 * @typedef {object} GachaRenderRequest
 * @property {string} label
 * @property {3 | 4 | 5} rarity
 * @property {24 | 30} fps
 * @property {640 | 854} width  Long edge of the portrait frame.
 * @property {number} seed  Seeds the page's randomness, so a seed replays the same flecks and sparkles.
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
 * roll looks different.
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
