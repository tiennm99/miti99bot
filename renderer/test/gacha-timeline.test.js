import {describe, expect, test} from 'vitest';
import {
  createStarfield,
  gachaTimeline,
  gachaTotalSeconds,
  getLabelFontSize,
  getMeteorState,
  getRankLetter,
  getRarityPalette,
  getShakeOffset,
  getStarRevealTimes,
  getTierEffects,
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

  test('meteor follows a ballistic arc from off-screen upper right to the impact point', () => {
    const duration = gachaTimeline.meteorEnd - gachaTimeline.meteorStart;
    const start = getMeteorState(0, 640, 360);
    const end = getMeteorState(duration, 640, 360);
    expect(start.x).toBeGreaterThan(640);
    expect(start.vy).toBeLessThan(0);
    expect(end.x).toBeCloseTo(640 * 0.4);
    expect(end.y).toBeCloseTo(360 * 0.72);
    expect(getMeteorState(duration + 1, 640, 360)).toEqual(end);
    expect(getMeteorState(-1, 640, 360)).toEqual(start);
  });

  test('meteor keeps horizontal speed and gains vertical speed at a constant rate', () => {
    const step = 0.2;
    const samples = [0, 1, 2, 3, 4, 5].map((index) => getMeteorState(index * step, 640, 360));
    for (const sample of samples) {
      expect(sample.vx).toBeCloseTo(samples[0]?.vx ?? NaN);
      expect(sample.vx).toBeLessThan(0);
    }
    const gravity = ((samples[1]?.vy ?? 0) - (samples[0]?.vy ?? 0)) / step;
    expect(gravity).toBeGreaterThan(0);
    for (let index = 1; index < samples.length; index += 1) {
      expect(((samples[index]?.vy ?? 0) - (samples[index - 1]?.vy ?? 0)) / step).toBeCloseTo(gravity);
    }
  });

  test('meteor bends from a slight climb into a steep dive', () => {
    const duration = gachaTimeline.meteorEnd - gachaTimeline.meteorStart;
    const early = getMeteorState(0, 640, 360);
    const late = getMeteorState(duration, 640, 360);
    const climbDegrees = (Math.atan2(-early.vy, Math.abs(early.vx)) * 180) / Math.PI;
    const diveDegrees = (Math.atan2(late.vy, Math.abs(late.vx)) * 180) / Math.PI;
    expect(climbDegrees).toBeGreaterThan(10);
    expect(diveDegrees).toBeGreaterThan(45);
  });

  test('starfield is deterministic per seed', () => {
    expect(createStarfield(5, 7)).toEqual(createStarfield(5, 7));
    expect(createStarfield(5, 7)).not.toEqual(createStarfield(5, 8));
  });

  test('long labels get smaller text', () => {
    expect(getLabelFontSize('Pizza', 360)).toBeGreaterThan(getLabelFontSize('x'.repeat(40), 360));
  });

  test('emblem shows B, A, S rank letters by rarity', () => {
    expect([3, 4, 5].map((rarity) => getRankLetter(/** @type {3 | 4 | 5} */ (rarity)))).toEqual(['B', 'A', 'S']);
  });

  test('every numeric effect grows with rarity and 5★ unlocks the extras', () => {
    const [three, four, five] = [3, 4, 5].map((rarity) => getTierEffects(/** @type {3 | 4 | 5} */ (rarity)));
    for (const key of /** @type {const} */ ([
      'headScale',
      'trailSamples',
      'sparks',
      'shockwaves',
      'raysOpacity',
      'motes',
      'starFlare',
    ])) {
      expect(four?.[key], key).toBeGreaterThan(three?.[key] ?? Infinity);
      expect(five?.[key], key).toBeGreaterThan(four?.[key] ?? Infinity);
    }
    expect(four?.shake).toBeGreaterThan(three?.shake ?? Infinity);
    expect(five?.shake).toBeGreaterThan(four?.shake ?? Infinity);
    expect(five?.skyFlood).toBeGreaterThan(four?.skyFlood ?? Infinity);
    expect([three?.halo, four?.halo, five?.halo]).toEqual([false, false, true]);
    expect(five).toMatchObject({starburst: true, counterRays: true, sheen: true});
    expect(three?.sparkleRain).toBe(0);
    expect(five?.sparkleRain).toBeGreaterThan(0);
  });

  test('impact shake decays to rest', () => {
    expect(getShakeOffset(1, 2, 10)).toEqual({x: 0, y: 0});
    expect(getShakeOffset(3, 2, 10)).toEqual({x: 0, y: 0});
    expect(getShakeOffset(2.1, 2, 0)).toEqual({x: 0, y: 0});
    const early = getShakeOffset(2.05, 2, 10);
    const late = getShakeOffset(2.7, 2, 10);
    expect(Math.hypot(early.x, early.y)).toBeGreaterThan(Math.hypot(late.x, late.y));
  });
});
