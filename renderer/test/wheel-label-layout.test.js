import {describe, expect, test} from 'vitest';
import {
  estimateTextWidth,
  getLabelFontSize,
  getLabelLines,
  getLabelTrack,
  getRadialLabelLayout,
  labelLineHeightRatio,
} from '../src/remotion/wheel-label-layout.js';
import {getFinalWheelRotationDegrees, getSliceCenterDegrees} from '../src/remotion/wheel-layout.js';

describe('wheel label layout', () => {
  const readableExample = 'Chiều nay uống CraneTea';
  const supportedSize = 512;
  const supportedRadius = supportedSize * 0.41;
  const supportedHubRadius = supportedSize * 0.09;

  /**
   * @param {object} [overrides]
   * @param {number} [overrides.optionCount]
   * @param {string} [overrides.text]
   */
  const get512Layout = ({optionCount = 8, text = readableExample} = {}) =>
    getRadialLabelLayout({
      center: supportedSize / 2,
      radius: supportedRadius,
      hubRadius: supportedHubRadius,
      centerDegrees: 0,
      optionCount,
      text,
    });

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

  test('flips labels that would rest mirrored so the poster frame reads upright', () => {
    const layout = getRadialLabelLayout({
      center: 256,
      radius: 210,
      hubRadius: 46,
      centerDegrees: 180,
      optionCount: 8,
      text: 'left side',
      winnerCenterDegrees: 0,
    });

    expect(layout.rotation).toBe(360);
  });

  test('gives both exactly-vertical labels the same reading direction', () => {
    // Option counts divisible by four always land two labels at exactly 90 and
    // 270 degrees. Radial layout cannot make those horizontal, but it can stop
    // them pointing opposite ways.
    for (const optionCount of [4, 8, 12, 16, 20, 24, 32]) {
      const winnerIndex = 1;
      const winnerCenterDegrees = getSliceCenterDegrees(optionCount, winnerIndex);
      const restRotation = getFinalWheelRotationDegrees(optionCount, winnerIndex);
      /** @type {number[]} */
      const verticalScreenAngles = [];

      for (let index = 0; index < optionCount; index += 1) {
        const layout = getRadialLabelLayout({
          center: 256,
          radius: 210,
          hubRadius: 46,
          centerDegrees: getSliceCenterDegrees(optionCount, index),
          optionCount,
          text: 'name',
          winnerCenterDegrees,
        });
        const screenAngle = (((layout.rotation + restRotation) % 360) + 360) % 360;

        if (Math.abs(screenAngle - 90) < 0.001 || Math.abs(screenAngle - 270) < 0.001) {
          verticalScreenAngles.push(Math.round(screenAngle));
        }
      }

      expect(verticalScreenAngles).toHaveLength(2);
      expect(new Set(verticalScreenAngles).size).toBe(1);
    }
  });

  test('leaves every label upright once the wheel rests on the winner', () => {
    for (const optionCount of [2, 3, 6, 8, 9, 16]) {
      for (const winnerIndex of [0, 1, optionCount - 1]) {
        const winnerCenterDegrees = getSliceCenterDegrees(optionCount, winnerIndex);

        for (let index = 0; index < optionCount; index += 1) {
          const layout = getRadialLabelLayout({
            center: 256,
            radius: 210,
            hubRadius: 46,
            centerDegrees: getSliceCenterDegrees(optionCount, index),
            optionCount,
            text: 'name',
            winnerCenterDegrees,
          });
          // Angle the text actually sits at once the wheel has stopped.
          const restRotation = getFinalWheelRotationDegrees(optionCount, winnerIndex);
          const screenAngle = (((layout.rotation + restRotation) % 360) + 360) % 360;

          expect(screenAngle > 90 && screenAngle < 270).toBe(false);
        }
      }
    }
  });

  test('fits the taller line box instead of cropping the last line', () => {
    // Vietnamese stacks a mark above and below the same vowel, so the rendered
    // block is taller than fontSize x lines and must still fit the wedge.
    for (const optionCount of [2, 4, 8, 12, 16, 24, 32]) {
      const layout = get512Layout({optionCount});
      const availableHeight =
        (2 * Math.PI * getLabelTrack(supportedRadius, supportedHubRadius).midRadius) / optionCount;
      const renderedHeight = layout.fontSize * labelLineHeightRatio * layout.lines.length;

      expect(renderedHeight).toBeLessThanOrEqual(availableHeight);
      expect(layout.height).toBeLessThanOrEqual(availableHeight);
    }
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

  test.each([2, 8, 12, 16])(
    'wraps the Vietnamese readability example when geometry permits at %i options',
    (optionCount) => {
      const layout = get512Layout({optionCount});
      const availableHeight =
        (2 * Math.PI * getLabelTrack(supportedRadius, supportedHubRadius).midRadius) / optionCount;

      expect(layout.lines.length).toBeGreaterThanOrEqual(2);
      expect(layout.lines.join(' ')).toBe(readableExample);
      expect(layout.fontSize).toBeGreaterThanOrEqual(14);
      expect(layout.height).toBeLessThanOrEqual(availableHeight);
      expect(layout.lines.every((line) => estimateTextWidth(line, layout.fontSize) <= layout.contentWidth)).toBe(true);
    },
  );

  test.each([
    [9, 'WWWWWWWWWWWWWWWW'],
    [10, 'WWWWWWWWWWWWWW'],
    [11, 'WWWWWWWWWWWWW'],
    [12, 'WWWWWWWWWWWW'],
    [13, 'WWWWWWWWWWW'],
  ])('wraps an unbroken token whose one-line fit is %ipx', (oneLineFontSize, text) => {
    const layout = get512Layout({text});
    const track = getLabelTrack(supportedRadius, supportedHubRadius);
    const contentWidth = track.width - Math.max(4, supportedRadius * 0.02) * 2;

    expect(getLabelFontSize(supportedRadius, 8, text, contentWidth)).toBe(oneLineFontSize);
    expect(layout.lines.length).toBeGreaterThanOrEqual(2);
    expect(layout.lines.join('')).toBe(text);
    expect(layout.fontSize).toBeGreaterThanOrEqual(14);
    expect(layout.lines.every((line) => estimateTextWidth(line, layout.fontSize) <= layout.contentWidth)).toBe(true);
  });

  test('keeps an exactly 14px one-line fit on one line', () => {
    const text = 'WWWWWWWWWW';
    const layout = get512Layout({text});

    expect(getLabelFontSize(supportedRadius, 8, text, layout.contentWidth)).toBe(14);
    expect(layout.lines).toEqual([text]);
    expect(layout.fontSize).toBe(14);
  });

  test.each([24, 32])('keeps the complete Vietnamese example bounded on one line at %i options', (optionCount) => {
    const layout = get512Layout({optionCount});
    const availableHeight =
      (2 * Math.PI * getLabelTrack(supportedRadius, supportedHubRadius).midRadius) / optionCount;

    expect(layout.lines).toEqual([readableExample]);
    expect(layout.fontSize).toBeLessThan(14);
    expect(layout.height).toBeLessThanOrEqual(availableHeight);
    expect(estimateTextWidth(layout.lines[0] ?? '', layout.fontSize)).toBeLessThanOrEqual(layout.contentWidth);
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
});
