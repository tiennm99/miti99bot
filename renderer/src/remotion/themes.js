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
    textHalo: 'rgba(255, 255, 255, 0.88)',
    sliceStroke: '#ffffff',
    winnerStroke: '#111827',
    winnerPillBg: '#111827',
    winnerPillText: '#ffffff',
    winnerPillRing: '#facc15',
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
    textHalo: 'rgba(255, 251, 235, 0.92)',
    sliceStroke: '#ffffff',
    winnerStroke: '#7f1d1d',
    winnerPillBg: '#7f1d1d',
    winnerPillText: '#fffbeb',
    winnerPillRing: '#f59e0b',
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
    textHalo: 'rgba(250, 250, 250, 0.9)',
    // Grey dividers instead of white: the lightest slices sit within 1.1:1 of
    // the background, so a white stroke leaves the wheel with no silhouette.
    sliceStroke: '#71717a',
    winnerStroke: '#18181b',
    winnerPillBg: '#18181b',
    winnerPillText: '#fafafa',
    winnerPillRing: '#a1a1aa',
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
