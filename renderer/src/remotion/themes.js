/**
 * Palette definitions for the rendered wheel.
 *
 * Slice orders are chosen so that neighbouring wedges stay distinguishable
 * under normal vision and under deuteranopia/protanopia simulation, because a
 * 200px GIF in a chat client is the realistic viewing size. `confetti` is kept
 * separate from `slices`: slice colors are tuned to carry dark label text at
 * large areas, which makes several of them far too light to read as 8px paper
 * chips against the background.
 */
export const themes = {
  classic: {
    background: '#f8fafc',
    wheelShadow: '0 18px 52px rgba(15, 23, 42, 0.22)',
    center: '#111827',
    centerRing: '#f9fafb',
    pointer: '#ef4444',
    pointerStroke: '#991b1b',
    text: '#111827',
    textInverse: '#ffffff',
    textHalo: 'rgba(255, 255, 255, 0.88)',
    sliceStroke: '#ffffff',
    slices: ['#f97316', '#14b8a6', '#f472b6', '#60a5fa', '#facc15', '#a78bfa', '#fb7185', '#34d399'],
    confetti: ['#f97316', '#14b8a6', '#f59e0b', '#3b82f6', '#ec4899', '#8b5cf6', '#10b981', '#f43f5e'],
  },
  festival: {
    background: '#fff7ed',
    wheelShadow: '0 20px 56px rgba(124, 45, 18, 0.24)',
    center: '#7f1d1d',
    centerRing: '#fffbeb',
    pointer: '#dc2626',
    pointerStroke: '#7f1d1d',
    text: '#1f2937',
    textInverse: '#fffbeb',
    textHalo: 'rgba(255, 251, 235, 0.92)',
    sliceStroke: '#ffffff',
    slices: ['#fb7185', '#06b6d4', '#84cc16', '#a78bfa', '#f59e0b', '#f472b6', '#fb923c', '#22c55e'],
    confetti: ['#f43f5e', '#ea580c', '#65a30d', '#0891b2', '#7c3aed', '#db2777', '#16a34a', '#d97706'],
  },
  mono: {
    background: '#f4f4f5',
    wheelShadow: '0 18px 52px rgba(24, 24, 27, 0.18)',
    center: '#18181b',
    centerRing: '#fafafa',
    pointer: '#27272a',
    pointerStroke: '#09090b',
    text: '#18181b',
    textInverse: '#fafafa',
    textHalo: 'rgba(250, 250, 250, 0.9)',
    // Grey dividers instead of white: the lightest slices sit within 1.1:1 of
    // the background, so a white stroke leaves the wheel with no silhouette.
    sliceStroke: '#71717a',
    slices: ['#87878f', '#cacad0', '#9d9da5', '#e1e1e5', '#b3b3ba', '#f9f9fa'],
    // Inverted to darks: every slice color in this theme is within 2.4:1 of the
    // background and disappears at particle size.
    confetti: ['#18181b', '#3f3f46', '#52525b', '#71717a', '#a1a1aa'],
  },
};

/**
 * @param {'classic' | 'festival' | 'mono'} name
 */
export const getTheme = (name) => themes[name] ?? themes.classic;

/**
 * Picks which palette entry a slice uses.
 *
 * Cycling the palette by index alone makes the first and last slice share a
 * color whenever the option count leaves a remainder of one, and those two are
 * neighbours on the wheel, so they merge into a single double-width wedge
 * separated only by a hairline stroke. The last slice is moved to the opposite
 * side of the palette instead.
 *
 * @param {number} index slice index
 * @param {number} optionCount total slices
 * @param {number} paletteLength available colors
 * @returns {number}
 */
export const getSliceColorIndex = (index, optionCount, paletteLength) => {
  if (paletteLength <= 0) {
    return 0;
  }

  if (index === optionCount - 1 && optionCount % paletteLength === 1) {
    return Math.floor(paletteLength / 2);
  }

  return index % paletteLength;
};

/**
 * Resolves the color a slice is painted with.
 *
 * @param {{slices: string[]}} theme
 * @param {number} index slice index
 * @param {number} optionCount total slices
 * @returns {string}
 */
export const getSliceColor = (theme, index, optionCount) =>
  theme.slices[getSliceColorIndex(index, optionCount, theme.slices.length)] ??
  theme.slices[0] ??
  '#cccccc';

/**
 * @param {string} hex `#rrggbb`
 * @returns {number[]} channel values in [0, 255]
 */
const parseHex = (hex) =>
  (hex.replace('#', '').match(/.{2}/gu) ?? []).map((channel) => Number.parseInt(channel, 16));

/**
 * @param {number[]} channels
 * @returns {string}
 */
const toHex = (channels) =>
  `#${channels
    .map((channel) => Math.round(Math.min(255, Math.max(0, channel))).toString(16).padStart(2, '0'))
    .join('')}`;

/**
 * @param {string} from
 * @param {string} to
 * @param {number} amount 0 keeps `from`, 1 returns `to`
 * @returns {string}
 */
const mixColors = (from, to, amount) => {
  const source = parseHex(from);
  const target = parseHex(to);

  return toHex(source.map((channel, index) => channel + ((target[index] ?? channel) - channel) * amount));
};

/**
 * @param {string} hex
 * @returns {number}
 */
const getRelativeLuminance = (hex) => {
  const [red = 0, green = 0, blue = 0] = parseHex(hex)
    .map((channel) => channel / 255)
    .map((channel) => (channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4));

  return 0.2126 * red + 0.7152 * green + 0.0722 * blue;
};

/**
 * @param {string} foreground
 * @param {string} background
 * @returns {number} WCAG contrast ratio
 */
const getContrastRatio = (foreground, background) => {
  const first = getRelativeLuminance(foreground);
  const second = getRelativeLuminance(background);

  return (Math.max(first, second) + 0.05) / (Math.min(first, second) + 0.05);
};

/**
 * How far the winner outline is pulled from the slice color toward the theme
 * ink. Enough to clear 3:1 against every slice in every theme, while keeping
 * the hue so the outline reads as belonging to the slice it frames.
 */
const winnerOutlineInkMix = 0.72;

/**
 * Winner colors derived from the winning slice.
 *
 * The celebration is tied to the slice that actually won: the announcement
 * carries that slice's color and both it and the wheel outline are framed in a
 * deepened version of the same hue. A fixed dark chip read as an unrelated
 * element pasted over the wheel.
 *
 * Ink is chosen by contrast rather than fixed, because a palette entry light
 * enough to carry dark label text is not guaranteed to carry it at pill size in
 * every future theme.
 *
 * @param {{slices: string[], text: string, textInverse: string}} theme
 * @param {string} sliceColor color of the winning slice
 * @returns {{background: string, outline: string, text: string}}
 */
export const getWinnerPalette = (theme, sliceColor) => ({
  background: sliceColor,
  outline: mixColors(sliceColor, theme.text, winnerOutlineInkMix),
  text:
    getContrastRatio(theme.text, sliceColor) >= getContrastRatio(theme.textInverse, sliceColor)
      ? theme.text
      : theme.textInverse,
});
