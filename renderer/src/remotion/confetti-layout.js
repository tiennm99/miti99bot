/**
 * Deterministic confetti field for the winner celebration.
 *
 * Remotion renders every GIF frame independently in headless Chromium, so the
 * animation must be a pure function of the frame number. These helpers turn a
 * seeded random source into stable particle descriptors and evaluate their
 * ballistic path at an arbitrary time, keeping the visual identical on every
 * render of the same seed.
 */

/** Canvas size the physics constants below are tuned for. */
export const CONFETTI_REFERENCE_SIZE = 512;

/**
 * @param {number} t normalized value in [0, 1]
 * @param {number} min
 * @param {number} max
 * @returns {number}
 */
const lerp = (t, min, max) => min + t * (max - min);

/**
 * @typedef {object} ConfettiParticle
 * @property {string} color
 * @property {number} gravity px/s^2 downward acceleration
 * @property {number} height px
 * @property {number} originX px
 * @property {number} originY px
 * @property {number} rotationSpeed deg/s
 * @property {number} startRotation deg
 * @property {number} vx px/s
 * @property {number} vy px/s (negative points up on screen)
 * @property {number} width px
 */

/**
 * Builds the confetti particles bursting outward and upward from the wheel
 * center. Physics scale with `size` relative to {@link CONFETTI_REFERENCE_SIZE}.
 *
 * @param {object} params
 * @param {number} params.count number of particles
 * @param {number} params.size canvas edge length in px
 * @param {string[]} params.colors palette to cycle through
 * @param {(seed: string) => number} params.random deterministic [0, 1) source
 * @returns {ConfettiParticle[]}
 */
export const createConfettiParticles = ({count, size, colors, random}) => {
  const scale = size / CONFETTI_REFERENCE_SIZE;
  const center = size / 2;
  const palette = colors.length > 0 ? colors : ['#f97316'];

  return Array.from({length: count}, (_, index) => {
    const rand = (/** @type {string} */ key) => random(`${index}-${key}`);
    // Fan upward: -160deg..-20deg in screen coords (0deg right, -90deg up).
    const angleRadians = (lerp(rand('angle'), -160, -20) * Math.PI) / 180;
    const speed = lerp(rand('speed'), 320, 820) * scale;
    const width = lerp(rand('width'), 6, 12) * scale;

    return {
      color: palette[Math.floor(rand('color') * palette.length)] ?? palette[0] ?? '#f97316',
      gravity: 1500 * scale,
      height: width * lerp(rand('ratio'), 0.4, 1),
      originX: center + lerp(rand('originX'), -0.06, 0.06) * size,
      originY: center + lerp(rand('originY'), -0.04, 0.02) * size,
      rotationSpeed: lerp(rand('spin'), -540, 540),
      startRotation: lerp(rand('rotation'), 0, 360),
      vx: Math.cos(angleRadians) * speed,
      vy: Math.sin(angleRadians) * speed,
      width,
    };
  });
};

/**
 * Evaluates a particle's position and rotation at `seconds` after the burst.
 *
 * @param {ConfettiParticle} particle
 * @param {number} seconds elapsed since the burst started
 * @returns {{rotation: number, x: number, y: number}}
 */
export const getConfettiParticleState = (particle, seconds) => ({
  rotation: particle.startRotation + particle.rotationSpeed * seconds,
  x: particle.originX + particle.vx * seconds,
  y: particle.originY + particle.vy * seconds + 0.5 * particle.gravity * seconds * seconds,
});
