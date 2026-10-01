import {describe, expect, test} from 'vitest';
import {
  gachaBetaTimeline,
  gachaBetaTotalSeconds,
  getBetaRank,
  getBurnGlint,
  getHeroMeteor,
  getSkyMeteorHead,
  meteorAngleDegrees,
  revealAnchor,
  skyMeteors,
} from '../src/remotion/gacha-beta-scene.js';

const width = 640;
const height = 360;

describe('gacha beta meteor scene', () => {
  test('beats run in order and the result holds before the clip ends', () => {
    const tl = gachaBetaTimeline;
    expect(tl.fadeInEnd).toBeLessThan(tl.heroStart);
    expect(tl.heroStart).toBeLessThan(tl.heroBurn);
    expect(tl.heroBurn).toBeLessThan(tl.labelIn);
    expect(tl.labelIn).toBeLessThan(tl.rankIn);
    expect(tl.rankIn + 2).toBeLessThan(gachaBetaTotalSeconds);
  });

  test('background meteors appear on their cue and glide down to the right', () => {
    for (const meteor of skyMeteors) {
      expect(getSkyMeteorHead(meteor, meteor.start - 0.01, width, height)).toBeNull();
      const a = getSkyMeteorHead(meteor, meteor.start + 0.5, width, height);
      const b = getSkyMeteorHead(meteor, meteor.start + 1.5, width, height);
      expect(a && b).toBeTruthy();
      if (a && b) {
        expect(b.x).toBeGreaterThan(a.x);
        expect(b.y).toBeGreaterThan(a.y);
        expect((Math.atan2(b.y - a.y, b.x - a.x) * 180) / Math.PI).toBeCloseTo(meteorAngleDegrees);
      }
    }
  });

  test('the volley stays in the upper sky so the result area is clear at the reveal', () => {
    for (const meteor of skyMeteors) {
      const head = getSkyMeteorHead(meteor, gachaBetaTimeline.labelIn, width, height);
      if (head) {
        expect(head.y).toBeLessThan(height * 0.62);
      }
    }
  });

  test('hero meteor is hidden before its cue, then slows to burn out on the anchor', () => {
    const tl = gachaBetaTimeline;
    expect(getHeroMeteor(tl.heroStart - 0.1, width, height).visible).toBe(0);
    const early = getHeroMeteor(tl.heroStart + 0.5, width, height);
    const mid = getHeroMeteor(tl.heroStart + 1, width, height);
    const late = getHeroMeteor(tl.heroBurn - 0.5, width, height);
    const late2 = getHeroMeteor(tl.heroBurn, width, height);
    expect(Math.hypot(mid.x - early.x, mid.y - early.y)).toBeGreaterThan(Math.hypot(late2.x - late.x, late2.y - late.y));
    expect(late2.x).toBeCloseTo(revealAnchor.x * width);
    expect(late2.y).toBeCloseTo(revealAnchor.y * height);
    expect(getHeroMeteor(tl.heroBurn + 0.2, width, height).visible).toBe(0);
  });

  test('burn glint fires only around the burn-out', () => {
    expect(getBurnGlint(gachaBetaTimeline.heroBurn - 1)).toBe(0);
    expect(getBurnGlint(gachaBetaTimeline.heroBurn)).toBe(1);
    expect(getBurnGlint(gachaBetaTimeline.heroBurn + 2)).toBe(0);
  });

  test('beta food reveal displays SSS', () => {
    expect(getBetaRank()).toBe('SSS');
  });
});
