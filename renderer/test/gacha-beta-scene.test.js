import {describe, expect, test} from 'vitest';
import {
  createShards,
  gachaBetaTimeline,
  gachaBetaTotalSeconds,
  getFlash,
  getHaloState,
  getMeteorHead,
  getRedMeteor,
  getShardOffset,
  getShot,
  getStarRevealTimes,
  getVolleyCometHead,
  getVortexState,
  volleyComets,
} from '../src/remotion/gacha-beta-scene.js';
import {createRandom} from '../src/remotion/gacha-timeline.js';

describe('gacha beta wish scene', () => {
  test('shots cut in order: vortex, beam, sky, volley, silhouette, reveal', () => {
    const tl = gachaBetaTimeline;
    /** @type {string[]} */
    const order = [];
    for (let seconds = 0; seconds < gachaBetaTotalSeconds; seconds += 0.05) {
      const shot = getShot(seconds);
      if (order.at(-1) !== shot) {
        order.push(shot);
      }
    }
    expect(order).toEqual(['vortex', 'beam', 'sky', 'volley', 'silhouette', 'reveal']);
    expect(tl.meteorStop).toBeLessThan(tl.haloEnd);
    expect(tl.haloEnd).toBeLessThan(tl.burstEnd);
    expect(tl.redStarIn).toBeLessThan(tl.volleyEnd);
  });

  test('one star per rarity level, finishing with time to hold the card', () => {
    for (const rarity of /** @type {const} */ ([3, 4, 5])) {
      const times = getStarRevealTimes(rarity);
      expect(times).toHaveLength(rarity);
      expect(times[0]).toBeGreaterThan(gachaBetaTimeline.plateIn);
      expect((times.at(-1) ?? Infinity) + 1).toBeLessThan(gachaBetaTotalSeconds);
    }
  });

  test('flashes open the clip, cover the burst in red, and hand over at each loud cut', () => {
    const tl = gachaBetaTimeline;
    expect(getFlash(0).opacity).toBe(1);
    expect(getFlash(tl.burstEnd)).toMatchObject({color: '#ff5a6e'});
    expect(getFlash(tl.redFlashEnd).opacity).toBe(1);
    expect(getFlash(tl.volleyEnd).opacity).toBe(1);
    expect(getFlash(tl.silhouetteEnd).opacity).toBe(1);
    expect(getFlash(2).opacity).toBe(0);
    expect(getFlash(9.5).opacity).toBe(0);
  });

  test('the camera dives into the vortex', () => {
    expect(getVortexState(gachaBetaTimeline.vortexEnd).zoom).toBeGreaterThan(getVortexState(0).zoom);
  });

  test('the meteor decelerates to a stop and holds through the halo', () => {
    const tl = gachaBetaTimeline;
    const a = getMeteorHead(tl.beamEnd);
    const b = getMeteorHead(tl.beamEnd + 0.3);
    const c = getMeteorHead(tl.meteorStop - 0.3);
    const stop = getMeteorHead(tl.meteorStop);
    expect(b.x - a.x).toBeGreaterThan(stop.x - c.x);
    expect(stop.speed).toBe(0);
    expect(getMeteorHead(tl.haloEnd)).toEqual(stop);
  });

  test('the rainbow halo blooms after the stop and closes before the burst', () => {
    const tl = gachaBetaTimeline;
    expect(getHaloState(tl.beamEnd).opacity).toBe(0);
    expect(getHaloState(tl.meteorStop + 0.3).opacity).toBeGreaterThan(0.8);
    expect(getHaloState(tl.haloEnd).opacity).toBe(0);
  });

  test('volley comets fall and drift right', () => {
    for (const comet of volleyComets) {
      const early = getVolleyCometHead(comet, comet.start + 0.2);
      const late = getVolleyCometHead(comet, gachaBetaTimeline.volleyEnd);
      expect(late.y).toBeGreaterThan(early.y);
      expect(late.x).toBeGreaterThan(early.x);
    }
  });

  test('the red meteor flares, then drops toward the centre by the impact', () => {
    const tl = gachaBetaTimeline;
    expect(getRedMeteor(tl.redStarIn).flare).toBe(0);
    const impact = getRedMeteor(tl.volleyEnd);
    expect(impact.falling).toBe(1);
    expect(impact.y).toBeGreaterThan(getRedMeteor(tl.redStarIn + 0.3).y);
  });

  test('silhouette shards burst outward from the centre', () => {
    const shards = createShards(createRandom(3), 12);
    for (const shard of shards) {
      const mid = getShardOffset(shard, gachaBetaTimeline.volleyEnd + 0.7);
      const end = getShardOffset(shard, gachaBetaTimeline.silhouetteEnd);
      expect(Math.hypot(end.dx, end.dy)).toBeGreaterThanOrEqual(Math.hypot(mid.dx, mid.dy));
      expect(Math.hypot(end.dx, end.dy)).toBeCloseTo(shard.reach);
    }
  });
});
