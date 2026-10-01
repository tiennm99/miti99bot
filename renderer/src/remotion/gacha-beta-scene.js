/**
 * Choreography for the beta wish: coloured meteors glide down a night sky over
 * a mountain ridge, a hero meteor in the rarity colour slows and burns out at
 * the centre, and the result appears where it died. Kept free of React so the
 * motion can be unit tested.
 */

export const gachaBetaTotalSeconds = 8;

/** Seconds from the start of the clip for each beat. */
export const gachaBetaTimeline = Object.freeze({
  fadeInEnd: 0.4,
  heroStart: 1.1,
  heroBurn: 4.4,
  labelIn: 4.5,
  rankIn: 4.9,
});

/** Where the hero meteor burns out and the result is centred, as frame fractions. */
export const revealAnchor = Object.freeze({x: 0.5, y: 0.44});

/** Every meteor travels down and to the right at this angle, as in the source shot. */
export const meteorAngleDegrees = 32;

const angle = (meteorAngleDegrees * Math.PI) / 180;
const direction = Object.freeze({x: Math.cos(angle), y: Math.sin(angle)});

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
 * @typedef {object} SkyMeteor
 * @property {number} start   Seconds at which the head is at its entry point.
 * @property {number} x       Entry point, fraction of width.
 * @property {number} y       Entry point, fraction of height.
 * @property {number} speed   Frame heights per second along the travel direction.
 * @property {number} length  Tail length in frame heights.
 * @property {number} width   Tail thickness in frame heights.
 * @property {string} color   Halo and tail colour; every head burns white.
 */

/**
 * Background meteors in the source shot's colours. They enter from the upper
 * left in a loose staggered volley and keep gliding slowly for the whole clip.
 */
export const skyMeteors = /** @type {readonly SkyMeteor[]} */ (
  Object.freeze([
    {start: 0, x: 0.36, y: -0.08, speed: 0.1, length: 0.5, width: 0.011, color: '#eef4ff'},
    {start: 0, x: 0.1, y: -0.02, speed: 0.11, length: 0.34, width: 0.014, color: '#52e07a'},
    {start: 0.2, x: -0.04, y: 0.12, speed: 0.12, length: 0.42, width: 0.018, color: '#9ff5e4'},
    {start: 0.9, x: -0.06, y: -0.05, speed: 0.12, length: 0.46, width: 0.016, color: '#f4f8ff'},
    {start: 1.3, x: 0.17, y: -0.09, speed: 0.1, length: 0.32, width: 0.012, color: '#ff8a7a'},
    {start: 1.6, x: -0.1, y: 0.06, speed: 0.13, length: 0.5, width: 0.015, color: '#6f9cff'},
    {start: 2.6, x: 0.55, y: -0.1, speed: 0.11, length: 0.4, width: 0.01, color: '#ffe6a8'},
    {start: 3.2, x: -0.08, y: -0.08, speed: 0.12, length: 0.38, width: 0.012, color: '#5fe8d0'},
  ])
);

/**
 * Head position of a background meteor in pixels, or null before it enters.
 *
 * @param {SkyMeteor} meteor
 * @param {number} seconds
 * @param {number} width
 * @param {number} height
 * @returns {{x: number, y: number} | null}
 */
export const getSkyMeteorHead = (meteor, seconds, width, height) => {
  if (seconds < meteor.start) {
    return null;
  }
  const travel = (seconds - meteor.start) * meteor.speed * height;
  return {x: meteor.x * width + direction.x * travel, y: meteor.y * height + direction.y * travel};
};

/** How far up the diagonal the hero meteor enters, in frame heights from the anchor. */
const heroReach = 0.72;

/**
 * The hero meteor streaks down the same diagonal as the volley, slowing until
 * it burns out on the reveal anchor.
 *
 * @param {number} seconds
 * @param {number} width
 * @param {number} height
 * @returns {{x: number, y: number, visible: number, tail: number}}
 *   `visible` fades the meteor out as it burns; `tail` shrinks its trail as
 *   it slows.
 */
export const getHeroMeteor = (seconds, width, height) => {
  const tl = gachaBetaTimeline;
  const t = clamp01((seconds - tl.heroStart) / (tl.heroBurn - tl.heroStart));
  const remaining = heroReach * height * (1 - t) ** 2.4;
  return {
    x: revealAnchor.x * width - direction.x * remaining,
    y: revealAnchor.y * height - direction.y * remaining,
    visible: seconds < tl.heroStart ? 0 : 1 - smoothstep((seconds - (tl.heroBurn - 0.15)) / 0.3),
    tail: 1 - 0.75 * t,
  };
};

/**
 * Brightness of the burn-out glint where the hero meteor dies, peaking at the
 * burn and fading as the result settles.
 *
 * @param {number} seconds
 * @returns {number}
 */
export const getBurnGlint = (seconds) => {
  const elapsed = seconds - gachaBetaTimeline.heroBurn;
  if (elapsed < -0.2 || elapsed > 1) {
    return 0;
  }
  return elapsed < 0 ? 1 + elapsed / 0.2 : (1 - elapsed) ** 2;
};

/** The beta food wish always reveals SSS; rarity still controls the hero meteor's colour. */
export const getBetaRank = () => 'SSS';
