import {describe, expect, test} from 'vitest';
import {
  createStarfield,
  gachaTimeline,
  gachaTotalSeconds,
  getEmblemGlyph,
  getLabelFontSize,
  getMeteorPoint,
  getRarityPalette,
  getStarRevealTimes,
} from '../src/remotion/gacha-timeline.js';

describe('gacha timeline', () => {
  test('phases run in order and finish before the clip ends', () => {
    const tl = gachaTimeline;
    expect(tl.meteorStart).toBeLessThan(tl.meteorEnd);
    expect(tl.flashStart).toBeLessThan(tl.flashPeak);
    expect(tl.meteorEnd).toBeLessThanOrEqual(tl.flashPeak);
    expect(tl.flashPeak).toBeLessThan(tl.flashHoldEnd);
    expect(tl.revealStart).toBeLessThanOrEqual(tl.flashHoldEnd);
    expect(tl.flashHoldEnd).toBeLessThan(tl.flashEnd);
    expect(tl.nameIn).toBeLessThan(tl.nameSettled);
    const lastStar = getStarRevealTimes(5).at(-1) ?? 0;
    expect(lastStar + 1).toBeLessThan(gachaTotalSeconds);
  });

  test.each([3, 4, 5])('reveals one star per rarity level for %i stars', (rarity) => {
    const times = getStarRevealTimes(/** @type {3 | 4 | 5} */ (rarity));
    expect(times).toHaveLength(rarity);
    expect(times[0]).toBe(gachaTimeline.starsStart);
    expect([...times].sort((a, b) => a - b)).toEqual(times);
  });

  test('each rarity has a distinct meteor colour', () => {
    const glows = [3, 4, 5].map((rarity) => getRarityPalette(/** @type {3 | 4 | 5} */ (rarity)).glow);
    expect(new Set(glows).size).toBe(3);
  });

  test('meteor falls from off-screen upper right toward the lower left', () => {
    const start = getMeteorPoint(0, 640, 360);
    const end = getMeteorPoint(1, 640, 360);
    expect(start.x).toBeGreaterThan(640);
    expect(start.y).toBeLessThan(0);
    expect(end.x).toBeLessThan(start.x);
    expect(end.y).toBeGreaterThan(start.y);
    expect(getMeteorPoint(2, 640, 360)).toEqual(end);
    expect(getMeteorPoint(-1, 640, 360)).toEqual(start);
  });

  test('starfield is deterministic per seed', () => {
    expect(createStarfield(5, 7)).toEqual(createStarfield(5, 7));
    expect(createStarfield(5, 7)).not.toEqual(createStarfield(5, 8));
  });

  test('long labels get smaller text', () => {
    expect(getLabelFontSize('Pizza', 360)).toBeGreaterThan(getLabelFontSize('x'.repeat(40), 360));
  });

  test('emblem glyph keeps multi-unit characters whole', () => {
    expect(getEmblemGlyph(' pizza')).toBe('P');
    expect(getEmblemGlyph('🍕 time')).toBe('🍕');
    expect(getEmblemGlyph('ăn')).toBe('Ă');
  });
});
