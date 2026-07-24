import {z} from 'zod';

const themeSchema = z.union([z.literal('classic'), z.literal('festival'), z.literal('mono')]);

/**
 * @typedef {object} WheelRequestLimits
 * @property {number} maxOptions
 * @property {number} maxOptionChars
 */

/**
 * @typedef {object} WheelRenderRequest
 * @property {string[]} options
 * @property {number} winnerIndex
 * @property {number} durationMs
 * @property {number} holdMs
 * @property {12 | 15 | 20} fps
 * @property {384 | 480 | 512} size
 * @property {'classic' | 'festival' | 'mono'} theme
 */

/**
 * @param {WheelRequestLimits} limits
 */
export const createWheelRequestSchema = (limits) =>
  z
    .object({
      options: z
        .array(z.string())
        .transform((options) => options.map((option) => option.trim()))
        .pipe(
          z
            .array(z.string().min(1).max(limits.maxOptionChars))
            .min(2)
            .max(limits.maxOptions),
        ),
      winnerIndex: z.number().int().nonnegative().optional(),
      durationMs: z.number().int().min(3000).max(10000).default(6500),
      holdMs: z.number().int().min(500).max(2500).default(1200),
      fps: z.union([z.literal(12), z.literal(15), z.literal(20)]).default(15),
      size: z.union([z.literal(384), z.literal(480), z.literal(512)]).default(512),
      theme: themeSchema.default('classic'),
    })
    .strict()
    .superRefine((value, context) => {
      if (value.winnerIndex !== undefined && value.winnerIndex >= value.options.length) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['winnerIndex'],
          message: 'winnerIndex must be within options range',
        });
      }
    });

/**
 * @param {unknown} input
 * @param {WheelRequestLimits} limits
 * @param {(optionCount: number) => number} pickWinner
 * @returns {WheelRenderRequest}
 */
export const parseWheelRequest = (input, limits, pickWinner) => {
  const parsed = createWheelRequestSchema(limits).parse(input);

  return {
    ...parsed,
    winnerIndex: parsed.winnerIndex ?? pickWinner(parsed.options.length),
  };
};
