/**
 * Timing, palette, and geometry for the gacha wish animation. Kept free of
 * React so the choreography can be unit tested.
 */

/** @typedef {3 | 4 | 5} GachaRarity */

/**
 * @typedef {object} RarityPalette
 * @property {string} core   Meteor head and hottest highlights.
 * @property {string} glow   Trail, ring burst, and emblem glow.
 * @property {string} deep   Background tint of the reveal scene.
 * @property {string} emblem Emblem fill gradient, light to dark.
 */

export const gachaTotalSeconds = 7;

/**
 * Seconds from the start of the clip. The meteor lands at `meteorEnd`, the
 * white flash peaks and holds, then fades to reveal the result.
 */
export const gachaTimeline = Object.freeze({
  meteorStart: 0.3,
  meteorEnd: 2.5,
  flashStart: 2.3,
  flashPeak: 2.7,
  flashHoldEnd: 2.95,
  flashEnd: 3.5,
  revealStart: 2.95,
  emblemSettled: 3.7,
  nameIn: 3.6,
  nameSettled: 4.1,
  starsStart: 4.1,
  starStep: 0.18,
});

/** Every rarity reveals its stars in the same gold, as in the source game. */
export const starColor = '#ffcc33';

/** @type {Record<GachaRarity, RarityPalette>} */
const palettes = {
  3: {core: '#f2f8ff', glow: '#6fb6ff', deep: '#163a8a', emblem: '#9fd0ff, #3a6fd8'},
  4: {core: '#fbf2ff', glow: '#c88cff', deep: '#4a1d8f', emblem: '#e2c2ff, #8a4ae6'},
  5: {core: '#fffbe8', glow: '#ffd36b', deep: '#7a4a10', emblem: '#fff0b8, #e2a12e'},
};

/**
 * @param {GachaRarity} rarity
 * @returns {RarityPalette}
 */
export const getRarityPalette = (rarity) => palettes[rarity];

/**
 * @typedef {object} MeteorState
 * @property {number} x   Pixels.
 * @property {number} y   Pixels.
 * @property {number} vx  Pixels per second.
 * @property {number} vy  Pixels per second.
 */

/**
 * Initial vertical speed as a fraction of horizontal speed. Negative launches
 * slightly upward, so gravity visibly bends the path over into a dive.
 */
const meteorEntrySlope = -0.3;

/**
 * Ballistic flight under constant gravity. Horizontal speed stays constant and
 * vertical speed grows linearly, so the path is a true parabola that rises a
 * touch, then bends over and steepens into a dive as the meteor speeds up. Gravity is solved so the
 * meteor lands exactly at the impact point when the fall ends.
 *
 * @param {number} elapsed Seconds since launch; clamped to the fall.
 * @param {number} width
 * @param {number} height
 * @returns {MeteorState}
 */
export const getMeteorState = (elapsed, width, height) => {
  const duration = gachaTimeline.meteorEnd - gachaTimeline.meteorStart;
  const t = Math.min(duration, Math.max(0, elapsed));
  const start = {x: width * 1.05, y: height * 0.1};
  const end = {x: width * 0.4, y: height * 0.72};
  const vx = (end.x - start.x) / duration;
  const vy0 = Math.abs(vx) * meteorEntrySlope;
  const gravity = (2 * (end.y - start.y - vy0 * duration)) / duration ** 2;
  return {
    x: start.x + vx * t,
    y: start.y + vy0 * t + 0.5 * gravity * t * t,
    vx,
    vy: vy0 + gravity * t,
  };
};

/**
 * Seconds at which each reveal star pops in, one per rarity level.
 *
 * @param {GachaRarity} rarity
 * @returns {number[]}
 */
export const getStarRevealTimes = (rarity) =>
  Array.from({length: rarity}, (_, index) => gachaTimeline.starsStart + index * gachaTimeline.starStep);

/**
 * Deterministic pseudo-random generator (mulberry32) so every render of the
 * same request draws the same sky.
 *
 * @param {number} seed
 * @returns {() => number} values in [0, 1)
 */
export const createRandom = (seed) => {
  let state = seed >>> 0;
  return () => {
    state = (state + 0x6d2b79f5) >>> 0;
    let value = state;
    value = Math.imul(value ^ (value >>> 15), value | 1);
    value ^= value + Math.imul(value ^ (value >>> 7), value | 61);
    return ((value ^ (value >>> 14)) >>> 0) / 4294967296;
  };
};

/**
 * @typedef {object} SkyStar
 * @property {number} x       Fraction of width.
 * @property {number} y       Fraction of height.
 * @property {number} radius  Fraction of height.
 * @property {number} phase   Twinkle phase offset in radians.
 */

/**
 * @param {number} count
 * @param {number} seed
 * @returns {SkyStar[]}
 */
export const createStarfield = (count, seed) => {
  const random = createRandom(seed);
  return Array.from({length: count}, () => ({
    x: random(),
    y: random() * 0.8,
    radius: 0.002 + random() * 0.004,
    phase: random() * Math.PI * 2,
  }));
};

/**
 * Font size for the revealed name, shrinking long labels so they fit within
 * two lines on the left half of the frame.
 *
 * @param {string} label
 * @param {number} height
 * @returns {number}
 */
export const getLabelFontSize = (label, height) => {
  const length = Array.from(label).length;
  if (length <= 8) {
    return Math.round(height * 0.12);
  }
  if (length <= 16) {
    return Math.round(height * 0.09);
  }
  if (length <= 28) {
    return Math.round(height * 0.07);
  }
  return Math.round(height * 0.055);
};

/** Rank letter shown on the emblem for each rarity. */
const rankLetters = {3: 'B', 4: 'A', 5: 'S'};

/**
 * @param {GachaRarity} rarity
 * @returns {'B' | 'A' | 'S'}
 */
export const getRankLetter = (rarity) => /** @type {'B' | 'A' | 'S'} */ (rankLetters[rarity]);

/**
 * @typedef {object} TierEffects
 * @property {number} headScale     Meteor head size multiplier.
 * @property {number} trailSamples  Glow blobs drawn along the meteor trail.
 * @property {number} sparks        Sparks shed by the meteor.
 * @property {boolean} halo         Rainbow ring forming around the meteor before landing.
 * @property {number} skyFlood      Peak opacity of the sky tint as the meteor lands.
 * @property {number} shake         Impact camera shake amplitude, as a fraction of height.
 * @property {number} shockwaves    Rings bursting from the emblem on reveal.
 * @property {boolean} starburst    Long light spikes behind the emblem.
 * @property {number} raysOpacity   Peak opacity of the rotating light rays.
 * @property {boolean} counterRays  Second ray layer rotating the other way.
 * @property {number} motes         Glowing motes floating up behind the reveal.
 * @property {number} sparkleRain   Sparkles falling across the reveal.
 * @property {boolean} sheen        Light sweep across the emblem and rank letter.
 * @property {number} starFlare     Size multiplier of each star's pop flare.
 */

/**
 * How loud each tier is. Every beat exists at every tier; higher tiers get
 * more of it, so 4★ clearly beats 3★ and 5★ clearly beats both.
 *
 * @type {Record<GachaRarity, TierEffects>}
 */
const tierEffects = {
  3: {
    headScale: 1,
    trailSamples: 22,
    sparks: 12,
    halo: false,
    skyFlood: 0,
    shake: 0,
    shockwaves: 1,
    starburst: false,
    raysOpacity: 0.16,
    counterRays: false,
    motes: 14,
    sparkleRain: 0,
    sheen: false,
    starFlare: 1,
  },
  4: {
    headScale: 1.3,
    trailSamples: 34,
    sparks: 36,
    halo: false,
    skyFlood: 0.25,
    shake: 0.012,
    shockwaves: 2,
    starburst: false,
    raysOpacity: 0.42,
    counterRays: false,
    motes: 32,
    sparkleRain: 0,
    sheen: false,
    starFlare: 1.4,
  },
  5: {
    headScale: 1.75,
    trailSamples: 48,
    sparks: 80,
    halo: true,
    skyFlood: 0.7,
    shake: 0.03,
    shockwaves: 3,
    starburst: true,
    raysOpacity: 0.7,
    counterRays: true,
    motes: 56,
    sparkleRain: 40,
    sheen: true,
    starFlare: 2,
  },
};

/**
 * @param {GachaRarity} rarity
 * @returns {TierEffects}
 */
export const getTierEffects = (rarity) => tierEffects[rarity];

/**
 * Decaying impact shake offset in pixels, zero before `start` and after it
 * settles.
 *
 * @param {number} seconds
 * @param {number} start
 * @param {number} amplitude Pixels at the moment of impact.
 * @returns {{x: number, y: number}}
 */
export const getShakeOffset = (seconds, start, amplitude) => {
  const elapsed = seconds - start;
  if (amplitude <= 0 || elapsed < 0 || elapsed > 0.8) {
    return {x: 0, y: 0};
  }
  const decay = amplitude * (1 - elapsed / 0.8) ** 2;
  return {x: Math.sin(elapsed * 71) * decay, y: Math.cos(elapsed * 53) * decay};
};
