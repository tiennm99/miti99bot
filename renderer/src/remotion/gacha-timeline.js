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
 * Point on the meteor's curved fall from the upper right toward the lower
 * left of the frame, as a quadratic Bézier.
 *
 * @param {number} progress 0 at launch, 1 at landing; values outside clamp.
 * @param {number} width
 * @param {number} height
 * @returns {{x: number, y: number}}
 */
export const getMeteorPoint = (progress, width, height) => {
  const t = Math.min(1, Math.max(0, progress));
  const start = {x: width * 1.08, y: -height * 0.18};
  const control = {x: width * 0.8, y: height * 0.3};
  const end = {x: width * 0.42, y: height * 0.6};
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

/**
 * The emblem shows the label's first character, keeping emoji and other
 * multi-unit code points intact.
 *
 * @param {string} label
 * @returns {string}
 */
export const getEmblemGlyph = (label) => (Array.from(label.trim())[0] ?? '?').toUpperCase();
