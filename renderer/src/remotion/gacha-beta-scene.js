/**
 * Scene math for the beta wish: a toon-shaded night sky viewed through a
 * perspective camera that dollies toward a hero cloud, follows a comet that
 * pierces it, and holds on the comet as it flares and bursts. Kept free of
 * React so the choreography can be unit tested.
 */

/**
 * @typedef {object} Vec3
 * @property {number} x
 * @property {number} y
 * @property {number} z
 */

/**
 * @typedef {object} Camera
 * @property {number} x
 * @property {number} y
 * @property {number} z
 * @property {number} roll  Screen roll in degrees.
 */

/**
 * @typedef {object} Projection
 * @property {number} x      Screen pixels.
 * @property {number} y      Screen pixels.
 * @property {number} scale  Pixels per world unit at that depth.
 * @property {number} depth  World units in front of the camera.
 */

export const gachaBetaTotalSeconds = 8;

/** Seconds from the start of the clip for each beat of the three phases. */
export const gachaBetaTimeline = Object.freeze({
  dollyEnd: 2,
  cometAppear: 1.2,
  pierceStart: 1.9,
  pierceEnd: 2.6,
  flightEnd: 4.8,
  flashPeak: 5.05,
  flashEnd: 5.45,
  cardIn: 5.2,
  rankIn: 5.6,
});

/** The cloud the camera dollies toward and the comet pierces. */
export const heroCloud = Object.freeze({x: 0, y: 30, z: 800, radius: 170});

/** The approach behind the cloud, the entry point, and the exit in front. */
const cometHidden = Object.freeze({x: -1050, y: -430, z: 1500});
const cometPierceFrom = Object.freeze({x: 40, y: 20, z: 1300});
const cometPierceTo = Object.freeze({x: 0, y: 30, z: 500});

/** The camera trails the comet from behind and slightly above it. */
const cameraFollowOffset = Object.freeze({x: 0, y: -30, z: -420});

/**
 * Seconds at which the comet crosses the hero cloud's plane. The pierce moves
 * linearly in depth, so the crossing is a fixed fraction of the segment.
 */
export const pierceTime =
  gachaBetaTimeline.pierceStart +
  ((gachaBetaTimeline.pierceEnd - gachaBetaTimeline.pierceStart) * (cometPierceFrom.z - heroCloud.z)) /
    (cometPierceFrom.z - cometPierceTo.z);

/**
 * @param {number} value
 * @returns {number}
 */
const clamp01 = (value) => Math.min(1, Math.max(0, value));

/**
 * @param {number} t
 * @returns {number}
 */
const easeInOutSine = (t) => -(Math.cos(Math.PI * clamp01(t)) - 1) / 2;

/**
 * @param {number} t
 * @returns {number}
 */
const easeOutCubic = (t) => 1 - (1 - clamp01(t)) ** 3;

/**
 * @param {Vec3} a
 * @param {Vec3} b
 * @param {number} t
 * @returns {Vec3}
 */
const lerp3 = (a, b, t) => ({x: a.x + (b.x - a.x) * t, y: a.y + (b.y - a.y) * t, z: a.z + (b.z - a.z) * t});

/**
 * Comet position in world units. It approaches behind the hero cloud, punches
 * straight through it toward the camera, then flies off on a rising arc and
 * slows as it gathers light for the burst.
 *
 * @param {number} seconds
 * @returns {Vec3}
 */
export const getCometPosition = (seconds) => {
  const tl = gachaBetaTimeline;
  if (seconds < tl.pierceStart) {
    const t = clamp01((seconds - tl.cometAppear) / (tl.pierceStart - tl.cometAppear));
    return lerp3(cometHidden, cometPierceFrom, t);
  }
  if (seconds < tl.pierceEnd) {
    return lerp3(cometPierceFrom, cometPierceTo, (seconds - tl.pierceStart) / (tl.pierceEnd - tl.pierceStart));
  }
  const u = easeOutCubic((seconds - tl.pierceEnd) / (tl.flightEnd - tl.pierceEnd));
  return {
    x: cometPierceTo.x + 600 * u,
    y: cometPierceTo.y - 300 * u + 120 * Math.sin(Math.PI * u),
    z: cometPierceTo.z + 2600 * u,
  };
};

/**
 * Action camera: a slow dolly toward the hero cloud, then a chase that locks
 * onto the comet from behind with a little lag and banks into its turn.
 *
 * @param {number} seconds
 * @returns {Camera}
 */
export const getCamera = (seconds) => {
  const tl = gachaBetaTimeline;
  const dolly = {x: 0, y: 0, z: 250 * easeInOutSine(seconds / tl.dollyEnd)};
  const lagged = getCometPosition(Math.max(tl.pierceEnd, seconds - 0.12));
  const chase = {
    x: lagged.x + cameraFollowOffset.x,
    y: lagged.y + cameraFollowOffset.y,
    z: lagged.z + cameraFollowOffset.z,
  };
  const follow = easeInOutSine((seconds - pierceTime) / (tl.pierceEnd + 0.4 - pierceTime));
  const position = lerp3(dolly, {...chase, z: Math.max(dolly.z, chase.z)}, follow);
  const bank = seconds > tl.pierceEnd ? 7 * Math.sin(Math.PI * clamp01((seconds - tl.pierceEnd) / 1.6)) : 0;
  return {...position, roll: bank * follow};
};

/**
 * Perspective projection onto the frame. Returns null for points behind or
 * too close to the camera.
 *
 * @param {Vec3} point
 * @param {Camera} camera
 * @param {number} width
 * @param {number} height
 * @returns {Projection | null}
 */
export const project = (point, camera, width, height) => {
  const depth = point.z - camera.z;
  if (depth < 30) {
    return null;
  }
  const focal = height * 1.2;
  const scale = focal / depth;
  return {
    x: width / 2 + (point.x - camera.x) * scale,
    y: height / 2 + (point.y - camera.y) * scale,
    scale,
    depth,
  };
};

/**
 * Comet glow from 0 to 1: it ignites behind the cloud, then builds steadily
 * through the flight until the burst.
 *
 * @param {number} seconds
 * @returns {number}
 */
export const getCometGlow = (seconds) => {
  const tl = gachaBetaTimeline;
  if (seconds < tl.pierceEnd) {
    return 0.35 * clamp01((seconds - tl.cometAppear) / 0.4);
  }
  return 0.35 + 0.65 * clamp01((seconds - tl.pierceEnd) / (tl.flightEnd - tl.pierceEnd)) ** 2;
};

/**
 * Decaying shake in pixels for the pierce and the burst.
 *
 * @param {number} seconds
 * @param {number} height
 * @returns {{x: number, y: number}}
 */
export const getBetaShake = (seconds, height) => {
  let x = 0;
  let y = 0;
  for (const [start, amplitude] of [
    [pierceTime, 0.02],
    [gachaBetaTimeline.flightEnd, 0.035],
  ]) {
    const elapsed = seconds - (start ?? 0);
    if (elapsed >= 0 && elapsed < 0.6) {
      const decay = (amplitude ?? 0) * height * (1 - elapsed / 0.6) ** 2;
      x += Math.sin(elapsed * 77) * decay;
      y += Math.cos(elapsed * 59) * decay;
    }
  }
  return {x, y};
};

/** The beta food wish always reveals SSS; rarity still controls its light palette. */
export const getBetaRank = () => 'SSS';
