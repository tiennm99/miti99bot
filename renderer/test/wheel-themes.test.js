import {describe, expect, test} from 'vitest';
import {getSliceColor, getSliceColorIndex, getWinnerPalette, themes} from '../src/remotion/themes.js';

/**
 * @param {string} color
 */
const getRelativeLuminance = (color) => {
  const channelHexValues = color.slice(1).match(/.{2}/gu) ?? [];
  const channels = channelHexValues
    .map((channel) => Number.parseInt(channel, 16) / 255)
    .map((channel) => (channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4));
  const [red = 0, green = 0, blue = 0] = channels;

  return 0.2126 * red + 0.7152 * green + 0.0722 * blue;
};

/**
 * @param {string} foreground
 * @param {string} background
 */
const getContrastRatio = (foreground, background) => {
  const luminances = [getRelativeLuminance(foreground), getRelativeLuminance(background)].sort(
    (left, right) => right - left,
  );
  const [lighter = 0, darker = 0] = luminances;

  return (lighter + 0.05) / (darker + 0.05);
};

describe('wheel themes', () => {
  test.each(Object.entries(themes))('%s uses one readable label color across every slice', (_name, theme) => {
    expect(theme.slices.every((slice) => getContrastRatio(theme.text, slice) >= 4.5)).toBe(true);
  });

  test.each(Object.entries(themes))('%s keeps confetti visible against the background', (_name, theme) => {
    // Slice colors are tuned to carry dark text over a large area, which leaves
    // several of them indistinguishable from the background at particle size.
    for (const color of theme.confetti) {
      expect(getContrastRatio(color, theme.background)).toBeGreaterThanOrEqual(2);
    }
  });

  test.each(Object.entries(themes))('%s gives the wheel a visible silhouette', (_name, theme) => {
    // Either the slices separate from the background on their own, or the
    // divider stroke has to carry the outline.
    for (const slice of theme.slices) {
      const sliceSeparates = getContrastRatio(slice, theme.background) >= 1.2;
      const strokeSeparates = getContrastRatio(theme.sliceStroke, theme.background) >= 3;

      expect(sliceSeparates || strokeSeparates).toBe(true);
    }
  });

  test.each(Object.entries(themes))('%s reads the winner name over any winning slice', (_name, theme) => {
    for (const slice of theme.slices) {
      const palette = getWinnerPalette(theme, slice);

      expect(getContrastRatio(palette.text, palette.background)).toBeGreaterThanOrEqual(4.5);
    }
  });

  test.each(Object.entries(themes))('%s frames the winner in its own hue', (_name, theme) => {
    for (const slice of theme.slices) {
      const palette = getWinnerPalette(theme, slice);

      // The outline separates the pill from the wheel behind it and the winning
      // wedge from its neighbours, so it has to clear both the slice it frames
      // and the page.
      expect(getContrastRatio(palette.outline, slice)).toBeGreaterThanOrEqual(3);
      expect(getContrastRatio(palette.outline, theme.background)).toBeGreaterThanOrEqual(3);
      // Derived from the slice, not a fixed chip: two different winners must not
      // produce the same frame.
      expect(palette.outline).not.toBe(theme.text);
    }
  });

  test.each(Object.entries(themes))('%s paints the winner pill in the winning slice color', (_name, theme) => {
    for (let optionCount = 2; optionCount <= 12; optionCount += 1) {
      for (let winnerIndex = 0; winnerIndex < optionCount; winnerIndex += 1) {
        const slice = getSliceColor(theme, winnerIndex, optionCount);

        expect(theme.slices).toContain(slice);
        expect(getWinnerPalette(theme, slice).background).toBe(slice);
      }
    }
  });

  test.each(Object.entries(themes))('%s never puts one color on adjacent slices', (_name, theme) => {
    for (let optionCount = 2; optionCount <= 32; optionCount += 1) {
      const colors = Array.from({length: optionCount}, (_unused, index) =>
        theme.slices[getSliceColorIndex(index, optionCount, theme.slices.length)],
      );

      for (let index = 0; index < optionCount; index += 1) {
        // Cyclic: the last slice touches the first one on the wheel.
        expect(colors[index]).not.toBe(colors[(index + 1) % optionCount]);
      }
    }
  });
});
