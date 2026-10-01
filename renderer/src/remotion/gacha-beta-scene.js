/**
 * Shot timing and motion for the beta wish, cut like the six-star wish
 * sequence of an anime gacha game: a dive through a cloud vortex, a beam
 * pass, a meteor that slows over a sea of clouds inside a rainbow halo and
 * bursts, a red flash, a volley of falling crystal comets joined by a red
 * one, a black silhouette on a red disc that sheds shards, and the splash
 * card with its name plate and six stars. Kept free of React so the
 * choreography can be unit tested.
 */

export const gachaBetaTotalSeconds = 10.5;

/** Seconds from the start of the clip for each cut and beat. */
export const gachaBetaTimeline = Object.freeze({
  vortexEnd: 1.1,
  beamEnd: 1.45,
  meteorStop: 2.9,
  haloEnd: 3.7,
  burstEnd: 4.5,
  redFlashEnd: 4.8,
  volleyEnd: 6.45,
  redStarIn: 5.4,
  silhouetteEnd: 7.7,
  plateIn: 7.8,
  starsStart: 8.2,
  starStep: 0.14,
});

/** The beta always reveals six stars and rank SSS. */
export const betaStarCount = 6;

/** @typedef {'vortex' | 'beam' | 'sky' | 'volley' | 'silhouette' | 'reveal'} BetaShot */

/**
 * @param {number} value
 * @returns {number}
 */
const clamp01 = (value) => Math.min(1, Math.max(0, value));

/**
 * @param {number} t
 * @returns {number}
 */
const smoothstep = (t) => {
  const x = clamp01(t);
  return x * x * (3 - 2 * x);
};

/**
 * @param {number} seconds
 * @param {number} start
 * @param {number} end
 * @returns {number}
 */
export const progress = (seconds, start, end) => clamp01((seconds - start) / (end - start));

/**
 * The shot on screen. Every change is a hard cut; flashes cover the
 * louder ones.
 *
 * @param {number} seconds
 * @returns {BetaShot}
 */
export const getShot = (seconds) => {
  const tl = gachaBetaTimeline;
  if (seconds < tl.vortexEnd) {
    return 'vortex';
  }
  if (seconds < tl.beamEnd) {
    return 'beam';
  }
  if (seconds < tl.redFlashEnd) {
    return 'sky';
  }
  if (seconds < tl.volleyEnd) {
    return 'volley';
  }
  if (seconds < tl.silhouetteEnd) {
    return 'silhouette';
  }
  return 'reveal';
};

/**
 * Full-frame flash colour and opacity: white out of the opening, red then
 * white after the burst, white on the red meteor's impact, and a pale pink
 * wash that hands the silhouette over to the splash card.
 *
 * @param {number} seconds
 * @returns {{color: string, opacity: number}}
 */
export const getFlash = (seconds) => {
  const tl = gachaBetaTimeline;
  /** @type {Array<[number, number, number, string, number?]>} start, peak, end, colour, peak opacity */
  const flashes = [
    [-1, 0, 0.45, '#ffffff'],
    [tl.burstEnd - 0.2, tl.burstEnd, tl.burstEnd + 0.12, '#ff5a6e', 0.8],
    [tl.burstEnd + 0.1, tl.redFlashEnd, tl.redFlashEnd + 0.3, '#ffffff'],
    [tl.volleyEnd - 0.12, tl.volleyEnd, tl.volleyEnd + 0.3, '#ffffff'],
    [tl.silhouetteEnd - 0.3, tl.silhouetteEnd, tl.silhouetteEnd + 0.45, '#ffe6f0'],
  ];
  for (const [start, peak, end, color, strength = 1] of flashes) {
    if (seconds >= start && seconds <= end) {
      const shape =
        seconds <= peak ? smoothstep(progress(seconds, start, peak)) : 1 - smoothstep(progress(seconds, peak, end));
      return {color, opacity: shape * strength};
    }
  }
  return {color: '#ffffff', opacity: 0};
};

/**
 * Camera dive into the cloud vortex: the ring of clouds swells past the
 * frame while the dark eye opens up.
 *
 * @param {number} seconds
 * @returns {{zoom: number, spin: number}}
 */
export const getVortexState = (seconds) => {
  const t = progress(seconds, 0, gachaBetaTimeline.vortexEnd);
  return {zoom: 1 + 1.4 * t ** 2.2, spin: 40 * t};
};

/**
 * Meteor head over the sea of clouds, as frame fractions. It enters from the
 * upper left and decelerates to a stop just right of centre, where the halo
 * and burst happen.
 *
 * @param {number} seconds
 * @returns {{x: number, y: number, speed: number}}
 *   `speed` is 1 on entry and 0 once the meteor has stopped.
 */
export const getMeteorHead = (seconds) => {
  const tl = gachaBetaTimeline;
  const t = progress(seconds, tl.beamEnd, tl.meteorStop);
  const glide = 1 - (1 - t) ** 3;
  return {
    x: 0.02 + 0.54 * glide,
    y: 0.12 + 0.44 * glide,
    speed: (1 - t) ** 2,
  };
};

/**
 * Rainbow halo around the stopped meteor: it blooms large, then contracts
 * while the head warms from white to pink.
 *
 * @param {number} seconds
 * @returns {{radius: number, opacity: number, warmth: number}}
 *   `radius` is in frame heights.
 */
export const getHaloState = (seconds) => {
  const tl = gachaBetaTimeline;
  const grow = smoothstep(progress(seconds, tl.meteorStop - 0.3, tl.meteorStop + 0.25));
  const shrink = smoothstep(progress(seconds, tl.meteorStop + 0.4, tl.haloEnd));
  return {
    radius: 0.06 + 0.3 * grow - 0.24 * shrink,
    opacity: grow * (1 - shrink),
    warmth: smoothstep(progress(seconds, tl.meteorStop, tl.haloEnd)),
  };
};

/**
 * The burst that follows the halo: a swelling starburst and anamorphic flare
 * while the sky tints violet.
 *
 * @param {number} seconds
 * @returns {{glow: number, flare: number, tint: number}}
 */
export const getBurstState = (seconds) => {
  const tl = gachaBetaTimeline;
  const t = progress(seconds, tl.haloEnd - 0.2, tl.burstEnd);
  return {glow: t ** 1.6, flare: smoothstep(t * 1.4), tint: smoothstep(t)};
};

/**
 * @typedef {object} VolleyComet
 * @property {number} x      Head x as a frame fraction.
 * @property {number} start  Seconds at which the head enters from the top.
 * @property {number} fall   Frame heights travelled until the volley ends.
 * @property {number} size   Head size in frame heights.
 */

/** Crystal comets falling nearly straight down after the red flash. */
export const volleyComets = /** @type {readonly VolleyComet[]} */ (
  Object.freeze([
    {x: 0.04, start: 4.7, fall: 0.95, size: 0.1},
    {x: 0.24, start: 4.78, fall: 1.05, size: 0.12},
    {x: 0.43, start: 4.74, fall: 0.85, size: 0.09},
    {x: 0.62, start: 4.82, fall: 1.0, size: 0.11},
  ])
);

/** The volley drifts right as it falls, as in the source shot. */
const volleyDrift = 0.12;

/**
 * Head of a volley comet in frame fractions. It falls fast, then eases.
 *
 * @param {VolleyComet} comet
 * @param {number} seconds
 * @returns {{x: number, y: number}}
 */
export const getVolleyCometHead = (comet, seconds) => {
  const t = progress(seconds, comet.start, gachaBetaTimeline.volleyEnd);
  const fall = comet.fall * (1 - (1 - t) ** 2);
  return {x: comet.x + volleyDrift * fall, y: -0.1 + fall};
};

/**
 * The red six-star meteor: a red star flares in the upper right, then drops
 * and strikes just below centre as the volley ends.
 *
 * @param {number} seconds
 * @returns {{x: number, y: number, flare: number, falling: number}}
 *   `flare` scales the star glint, `falling` runs from 0 to 1 over the drop.
 */
export const getRedMeteor = (seconds) => {
  const tl = gachaBetaTimeline;
  const appear = smoothstep(progress(seconds, tl.redStarIn, tl.redStarIn + 0.3));
  const falling = progress(seconds, tl.redStarIn + 0.35, tl.volleyEnd) ** 1.8;
  return {
    x: 0.68 - 0.2 * falling,
    y: 0.08 + 0.55 * falling,
    flare: appear * (1 + 1.5 * falling),
    falling,
  };
};

/**
 * @typedef {object} Shard
 * @property {number} angle   Flight direction in degrees.
 * @property {number} reach   Distance travelled in frame heights.
 * @property {number} size    Edge length in frame heights.
 * @property {number} spin    Rotation in degrees over the shot.
 * @property {number} start   Delay in seconds after the silhouette appears.
 */

/**
 * Seeded shards that burst off the silhouette and drift outward.
 *
 * @param {() => number} random
 * @param {number} count
 * @returns {Shard[]}
 */
export const createShards = (random, count) =>
  Array.from({length: count}, () => ({
    angle: random() * 360,
    reach: 0.25 + random() * 0.55,
    size: 0.015 + random() * 0.05,
    spin: (random() - 0.5) * 540,
    start: random() * 0.35,
  }));

/**
 * Offset of a shard from the silhouette centre, in frame heights.
 *
 * @param {Shard} shard
 * @param {number} seconds
 * @returns {{dx: number, dy: number, rotate: number, opacity: number}}
 */
export const getShardOffset = (shard, seconds) => {
  const tl = gachaBetaTimeline;
  const t = progress(seconds, tl.volleyEnd + 0.15 + shard.start, tl.silhouetteEnd);
  const travel = shard.reach * (1 - (1 - t) ** 3);
  const radians = (shard.angle * Math.PI) / 180;
  return {
    dx: Math.cos(radians) * travel,
    dy: Math.sin(radians) * travel,
    rotate: shard.spin * t,
    opacity: t > 0 ? 1 : 0,
  };
};

/**
 * Seconds at which each reveal star pops in, left to right.
 *
 * @returns {number[]}
 */
export const getStarRevealTimes = () =>
  Array.from({length: betaStarCount}, (_, index) => gachaBetaTimeline.starsStart + index * gachaBetaTimeline.starStep);

/** The beta food wish always reveals SSS; rarity tints the meteor and splash. */
export const getBetaRank = () => 'SSS';
