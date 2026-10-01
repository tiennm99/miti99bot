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

/** Centre of the rank emblem on the reveal, as fractions of the frame. */
export const emblemCenter = Object.freeze({x: 0.66, y: 0.47});

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
 * Point on the meteor's curved flight from the upper left toward the rank
 * emblem on the right, as a quadratic Bézier. It lands exactly on the emblem,
 * so the impact burst becomes the badge reveal.
 *
 * @param {number} progress 0 at launch, 1 at landing; values outside clamp.
 * @param {number} width
 * @param {number} height
 * @returns {{x: number, y: number}}
 */
export const getMeteorPoint = (progress, width, height) => {
  const t = Math.min(1, Math.max(0, progress));
  const start = {x: -width * 0.08, y: -height * 0.18};
  const control = {x: width * 0.22, y: height * 0.38};
  const end = {x: width * emblemCenter.x, y: height * emblemCenter.y};
  const u = 1 - t;
  return {
    x: u * u * start.x + 2 * u * t * control.x + t * t * end.x,
    y: u * u * start.y + 2 * u * t * control.y + t * t * end.y,
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
 * @property {number} speed   Twinkle speed in radians per second.
 * @property {boolean} glint  Draws a cross-shaped glint when it flares.
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
    speed: 2 + random() * 5,
    glint: random() < 0.18,
  }));
};

/**
 * Twinkle brightness in [0, 1]. Cubing the sine keeps a star dim most of the
 * time with short bright flares, which reads as twinkling rather than a slow
 * pulse.
 *
 * @param {number} seconds
 * @param {SkyStar} star
 * @returns {number}
 */
export const getTwinkle = (seconds, star) => (0.5 + 0.5 * Math.sin(seconds * star.speed + star.phase)) ** 3;

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
 * @property {boolean} halo         Rainbow sunburst bursting from the meteor before landing.
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
