import {describe, expect, test} from 'vitest';
import {
  estimateTextWidth,
  getContrastingTextColor,
  getLabelFontSize,
  getLabelLines,
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

  test('keeps short labels on one line at the base size', () => {
    const layout = getRadialLabelLayout({
      center: 256,
      radius: 210,
      hubRadius: 46,
      centerDegrees: 0,
      optionCount: 8,
      text: 'alice',
    });

    expect(layout.lines).toEqual(['alice']);
    expect(layout.fontSize).toBe(getLabelFontSize(210, 8));
  });

  test('fits smoke-fixture labels on one line at the minimum supported size', () => {
    const size = 384;
    const radius = size * 0.41;
    const hubRadius = size * 0.09;
    const expectedFontSizes = new Map([
      ['alpha', 21],
      ['beta', 22],
      ['gamma', 20],
      ['delta', 21],
    ]);

    for (const [text, expectedFontSize] of expectedFontSizes) {
      const layout = getRadialLabelLayout({
        center: size / 2,
        radius,
        hubRadius,
        centerDegrees: 0,
        optionCount: 4,
        text,
      });

      expect(layout.lines).toEqual([text]);
      expect(layout.fontSize).toBe(expectedFontSize);
    }
  });

  test('wraps long phrases into balanced word-boundary lines', () => {
    const lines = getLabelLines('a much longer wheel entry label', 24, 140, 3);
    const widths = lines.map((line) => estimateTextWidth(line, 24));

    expect(lines).toHaveLength(3);
    expect(lines.join(' ')).toBe('a much longer wheel entry label');
    expect(Math.max(...widths) - Math.min(...widths)).toBeLessThan(80);
  });

  test('sizes wrapped labels from their longest rendered line', () => {
    const layout = getRadialLabelLayout({
      center: 256,
      radius: 210,
      hubRadius: 46,
      centerDegrees: 0,
      optionCount: 8,
      text: 'a much longer wheel entry label',
    });
    const longestLine = layout.lines.reduce((longest, line) =>
      estimateTextWidth(line, 24) > estimateTextWidth(longest, 24) ? line : longest,
    '');

    expect(layout.lines.length).toBeGreaterThan(1);
    expect(layout.fontSize).toBe(getLabelFontSize(210, 8, longestLine, layout.contentWidth));
    expect(layout.fontSize).toBeGreaterThan(
      getLabelFontSize(210, 8, 'a much longer wheel entry label', layout.contentWidth),
    );
    expect(layout.height).toBeLessThanOrEqual((2 * Math.PI * getLabelTrack(210, 46).midRadius) / 8);
  });

  test('keeps supported dense layouts to one bounded line', () => {
    const size = 384;
    const radius = size * 0.41;
    const hubRadius = size * 0.09;
    const optionCount = 32;
    const layout = getRadialLabelLayout({
      center: size / 2,
      radius,
      hubRadius,
      centerDegrees: 0,
      optionCount,
      text: 'a label that reaches the configured limit',
    });
    const availableHeight = (2 * Math.PI * getLabelTrack(radius, hubRadius).midRadius) / optionCount;

    expect(layout.lines).toEqual(['a label that reaches the configured limit']);
    expect(layout.fontSize).toBe(8);
    expect(layout.height).toBeLessThanOrEqual(availableHeight);
  });

  test('keeps the border box on the radial track and fits text to its padded content width', () => {
    const radius = 384 * 0.41;
    const hubRadius = 384 * 0.09;
    const layout = getRadialLabelLayout({
      center: 192,
      radius,
      hubRadius,
      centerDegrees: 0,
      optionCount: 4,
      text: 'alpha',
    });

    const track = getLabelTrack(radius, hubRadius);

    expect(layout.width).toBe(track.width);
    expect(layout.horizontalPadding).toBe(Math.max(4, radius * 0.02));
    expect(layout.contentWidth).toBe(layout.width - layout.horizontalPadding * 2);
    expect(estimateTextWidth(layout.lines[0] ?? '', layout.fontSize)).toBeLessThanOrEqual(layout.contentWidth);
  });

  test('normalizes whitespace and splits unbroken text without breaking graphemes', () => {
    expect(getLabelLines('  alpha   beta\n gamma  ', 24, 90, 3).join(' ')).toBe('alpha beta gamma');

    const token = 'winner🏆winner🏆winner';
    const lines = getLabelLines(token, 24, 90, 3);

    expect(lines.length).toBeGreaterThan(1);
    expect(lines.join('')).toBe(token);
    expect(lines.every((line) => {
      const firstCodeUnit = line.charCodeAt(0);
      const lastCodeUnit = line.charCodeAt(line.length - 1);

      return !(firstCodeUnit >= 0xdc00 && firstCodeUnit <= 0xdfff) && !(lastCodeUnit >= 0xd800 && lastCodeUnit <= 0xdbff);
    })).toBe(true);
  });

  test('uses contrast text colors for light and dark slices', () => {
    expect(getContrastingTextColor('#facc15')).toBe('#111827');
    expect(getContrastingTextColor('#7f1d1d')).toBe('#ffffff');
  });
});
