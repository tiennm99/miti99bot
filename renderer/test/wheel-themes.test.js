import {describe, expect, test} from 'vitest';
import {themes} from '../src/remotion/themes.js';

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
});
