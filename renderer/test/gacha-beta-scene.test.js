import {describe, expect, test} from 'vitest';
import {
  gachaBetaTimeline,
  gachaBetaTotalSeconds,
  getBetaRank,
  getBetaShake,
  getCamera,
  getCometGlow,
  getCometPosition,
  heroCloud,
  pierceTime,
  project,
} from '../src/remotion/gacha-beta-scene.js';

describe('gacha beta scene', () => {
  test('phases run in order inside the clip', () => {
    const tl = gachaBetaTimeline;
    expect(tl.cometAppear).toBeLessThan(tl.pierceStart);
    expect(tl.pierceStart).toBeLessThan(pierceTime);
    expect(pierceTime).toBeLessThan(tl.pierceEnd);
    expect(tl.pierceEnd).toBeLessThan(tl.flightEnd);
    expect(tl.flightEnd).toBeLessThan(tl.flashPeak);
    expect(tl.cardIn).toBeLessThan(tl.flashEnd);
    expect(tl.rankIn + 1).toBeLessThan(gachaBetaTotalSeconds);
  });

  test('comet waits behind the hero cloud, then crosses its plane at the pierce', () => {
    expect(getCometPosition(gachaBetaTimeline.cometAppear).z).toBeGreaterThan(heroCloud.z);
    expect(getCometPosition(pierceTime).z).toBeCloseTo(heroCloud.z);
    expect(getCometPosition(gachaBetaTimeline.pierceEnd).z).toBeLessThan(heroCloud.z);
  });

  test('comet is hidden behind the cloud: projected inside its silhouette', () => {
    const seconds = gachaBetaTimeline.pierceStart;
    const camera = getCamera(seconds);
    const comet = project(getCometPosition(seconds), camera, 640, 360);
    const cloud = project(heroCloud, camera, 640, 360);
    expect(comet && cloud).toBeTruthy();
    if (comet && cloud) {
      expect(Math.hypot(comet.x - cloud.x, comet.y - cloud.y)).toBeLessThan(heroCloud.radius * cloud.scale);
      expect(comet.depth).toBeGreaterThan(cloud.depth);
    }
  });

  test('approaching comet is visible beside the cloud before passing behind it', () => {
    const seconds = 1.4;
    const camera = getCamera(seconds);
    const comet = project(getCometPosition(seconds), camera, 640, 360);
    const cloud = project(heroCloud, camera, 640, 360);
    expect(comet && cloud).toBeTruthy();
    if (comet && cloud) {
      expect(comet.x).toBeGreaterThan(0);
      expect(comet.x).toBeLessThan(640);
      expect(comet.y).toBeGreaterThan(0);
      expect(comet.y).toBeLessThan(360);
      expect(Math.hypot(comet.x - cloud.x, comet.y - cloud.y)).toBeGreaterThan(heroCloud.radius * cloud.scale);
      expect(comet.depth).toBeGreaterThan(cloud.depth);
    }
  });

  test('camera dollies toward the cloud, then chases the comet and keeps it in frame', () => {
    expect(getCamera(1).z).toBeGreaterThan(getCamera(0).z);
    for (const seconds of [3, 3.5, 4, 4.5]) {
      const camera = getCamera(seconds);
      const comet = project(getCometPosition(seconds), camera, 640, 360);
      expect(comet).not.toBeNull();
      if (comet) {
        expect(comet.x).toBeGreaterThan(640 * 0.2);
        expect(comet.x).toBeLessThan(640 * 0.8);
        expect(comet.y).toBeGreaterThan(360 * 0.2);
        expect(comet.y).toBeLessThan(360 * 0.8);
      }
    }
  });

  test('camera never moves backward', () => {
    let previous = -Infinity;
    for (let seconds = 0; seconds <= gachaBetaTimeline.flightEnd; seconds += 0.05) {
      const {z} = getCamera(seconds);
      expect(z).toBeGreaterThanOrEqual(previous - 1e-6);
      previous = z;
    }
  });

  test('projection drops points behind the camera', () => {
    expect(project({x: 0, y: 0, z: 10}, {x: 0, y: 0, z: 0, roll: 0}, 640, 360)).toBeNull();
    const centre = project({x: 0, y: 0, z: 500}, {x: 0, y: 0, z: 0, roll: 0}, 640, 360);
    expect(centre).toMatchObject({x: 320, y: 180});
  });

  test('comet glow builds to full by the burst', () => {
    expect(getCometGlow(0)).toBe(0);
    expect(getCometGlow(3)).toBeGreaterThan(getCometGlow(2));
    expect(getCometGlow(gachaBetaTimeline.flightEnd)).toBeCloseTo(1);
  });

  test('shake fires at the pierce and the burst only', () => {
    expect(getBetaShake(1, 360)).toEqual({x: 0, y: 0});
    expect(Math.hypot(...Object.values(getBetaShake(pierceTime + 0.05, 360)))).toBeGreaterThan(0);
    expect(Math.hypot(...Object.values(getBetaShake(gachaBetaTimeline.flightEnd + 0.05, 360)))).toBeGreaterThan(0);
  });

  test('beta food reveal displays SSS', () => {
    expect(getBetaRank()).toBe('SSS');
  });
});
