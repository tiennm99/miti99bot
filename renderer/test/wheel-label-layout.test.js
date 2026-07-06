import {describe, expect, test} from 'vitest';
import {
  estimateTextWidth,
  getContrastingTextColor,
  getLabelFontSize,
  getLabelTrack,
  getRadialLabelLayout,
} from '../src/remotion/wheel-label-layout.js';

describe('wheel label layout', () => {
  test('centers radial label tracks on the slice center angle', () => {
    const center = 256;
    const radius = 210;
    const hubRadius = 46;
    const centerDegrees = 45;
    const layout = getRadialLabelLayout({
      center,
      radius,
      hubRadius,
      centerDegrees,
      optionCount: 8,
      text: 'alice',
    });
    const track = getLabelTrack(radius, hubRadius);
    const radians = (centerDegrees * Math.PI) / 180;

    expect(layout.x).toBeCloseTo(center + Math.cos(radians) * track.midRadius);
    expect(layout.y).toBeCloseTo(center + Math.sin(radians) * track.midRadius);
    expect(layout.width).toBe(track.width);
    expect(layout.rotation).toBe(centerDegrees);
  });

  test('keeps left-side text aligned with the slice instead of flipping it upright', () => {
    const layout = getRadialLabelLayout({
      center: 256,
      radius: 210,
      hubRadius: 46,
      centerDegrees: 180,
      optionCount: 8,
      text: 'left side',
    });

    expect(layout.rotation).toBe(180);
  });

  test('shrinks long labels to fit the radial track', () => {
    const shortSize = getLabelFontSize(210, 8, 'alice', 140);
    const longSize = getLabelFontSize(210, 8, 'a much longer wheel entry label', 140);

    expect(longSize).toBeLessThan(shortSize);
    expect(estimateTextWidth('alice', shortSize)).toBeLessThan(140);
  });

  test('uses contrast text colors for light and dark slices', () => {
    expect(getContrastingTextColor('#facc15')).toBe('#111827');
    expect(getContrastingTextColor('#7f1d1d')).toBe('#ffffff');
  });
});
