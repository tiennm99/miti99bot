/**
 * Deterministic confetti field for the winner celebration.
 *
 * Remotion renders every GIF frame independently in headless Chromium, so the
 * animation must be a pure function of the frame number. These helpers turn a
 * seeded random source into stable particle descriptors and evaluate their
 * flight at an arbitrary time, keeping the visual identical on every render of
 * the same seed.
 *
 * Motion is air-drag ballistics evaluated in closed form rather than stepped
 * integration: paper decelerates hard after launch instead of flying a clean
 * parabola, and a closed form keeps every frame independent.
 */

/** Canvas size the physics constants below are tuned for. */
export const CONFETTI_REFERENCE_SIZE = 512;

/** Downward acceleration in px/s^2 at the reference size. */
const gravity = 340;

/**
 * Launch speed range in px/s at the reference size.
 *
 * Capped by the frame rate rather than by how a cannon behaves: at 15fps a
 * particle travelling 800px/s covers 53px per frame, and a GIF has no motion
 * blur to tie those samples together. Past roughly 3x the body length per frame
 * the burst reads as disconnected dots instead of moving paper.
 */
const minSpeed = 600;
const maxSpeed = 820;

/**
 * Drag coefficient range in 1/s. Terminal fall speed is `gravity / drag`, so
 * this range is what keeps particles in frame for the whole celebration. Small
 * chips get more drag than large ones, which reads as depth.
 */
const minDrag = 1.4;
const maxDrag = 2.0;

/**
 * Launch timing. A cannon fires most of its charge at once and then trails, so
 * `firstVolleyShare` of the particles leave at t=0 — which is what makes the
 * opening frame a compact pop rather than an empty canvas — and the rest spread
 * over `launchWindowSeconds`, weighted toward the start.
 */
const launchWindowSeconds = 0.38;
// Raised to offset the share diverted to the second volley below, so the
// opening frame keeps its compact pop.
const firstVolleyShare = 0.5;

/**
 * Second volley. This share of the particles launches partway through the
 * celebration so a long hold does not end on a thinning field. Expressed as a
 * fraction of the window rather than in seconds, so a short hold collapses the
 * second wave into the first instead of firing it after the GIF has ended.
 */
const secondWaveShare = 0.22;
const secondWaveWindowStart = 0.42;
const secondWaveWindowEnd = 0.55;

/**
 * Motion streak. A GIF carries no motion blur, so a fast chip samples as a
 * detached dot. Stretching it along its direction of travel restores the visual
 * link between frames. Capped well short of a rain-streak look.
 */
const maxStretch = 1.8;
// Set above the launch speed range so fast and slow chips streak by different
// amounts. Tuned to the reference speed range: at 620 every particle pinned to
// the cap for the first three frames and the burst lost its size variety.
const stretchReferenceSpeed = 1200;

/** Cannon muzzle positions as a fraction of the canvas. */
const cannonInset = 0.08;
const cannonHeight = 0.8;

/** Body size range in px at the reference size. */
const minWidth = 6;
const maxWidth = 18;

/**
 * Minimum rendered thickness in px, unscaled: below ~3px a chip survives GIF
 * quantisation as a single dim row of pixels, or vanishes.
 */
const minHeight = 3;
const maxHeight = 26;

/** Tumble rate range in deg/s. Capped so 15fps samples a rotation, not a flicker. */
const maxSpin = 300;

/** Lateral flutter, in Hz and px at the reference size. */
const minSwayHz = 0.5;
const maxSwayHz = 0.9;
const minSwayAmplitude = 8;
const maxSwayAmplitude = 20;

/**
 * @param {number} t normalized value in [0, 1]
 * @param {number} min
 * @param {number} max
 * @returns {number}
 */
const lerp = (t, min, max) => min + t * (max - min);

/**
 * @param {number} value
 * @param {number} min
 * @param {number} max
 * @returns {number}
 */
const clamp = (value, min, max) => Math.min(Math.max(value, min), max);

/**
 * @typedef {object} ConfettiParticle
 * @property {string} color
 * @property {number} delay s before this particle launches
 * @property {number} drag 1/s air resistance
 * @property {number} gravity px/s^2 downward acceleration
 * @property {number} height px
 * @property {number} originX px
 * @property {number} originY px
 * @property {number} rotationSpeed deg/s
 * @property {number} startRotation deg
 * @property {number} swayAmplitude px
 * @property {number} swayHz cycles/s
 * @property {number} swayPhase rad
 * @property {number} vx px/s
 * @property {number} vy px/s (negative points up on screen)
 * @property {number} width px
 */

/**
 * Builds the confetti particles for a two-cannon burst framing the wheel.
 *
 * Alternating particles launch from the lower-left and lower-right corners and
 * arc inward across the disc. Launching from the corners rather than the hub
 * keeps the dense opening frames over plain background, where the pop is
 * legible, and clear of the winner pill that covers the hub during the hold.
 *
 * @param {object} params
 * @param {number} params.count number of particles
 * @param {number} params.size canvas edge length in px
 * @param {string[]} params.colors palette to cycle through
 * @param {(seed: string) => number} params.random deterministic [0, 1) source
 * @param {number} [params.windowSeconds] length of the celebration, for pacing
 *   the second volley
 * @returns {ConfettiParticle[]}
 */
export const createConfettiParticles = ({count, size, colors, random, windowSeconds = 0}) => {
  const scale = size / CONFETTI_REFERENCE_SIZE;
  const palette = colors.length > 0 ? colors : ['#f97316'];

  return Array.from({length: count}, (_, index) => {
    const rand = (/** @type {string} */ key) => random(`${index}-${key}`);
    const fromLeft = index % 2 === 0;
    // Bias size small so a few large chips anchor the burst at 15fps while most
    // particles stay light enough to flutter.
    const sizeNorm = rand('width') ** 1.7;
    const width = lerp(sizeNorm, minWidth, maxWidth) * scale;
    // Mirror the left cannon's fan: 0deg points right, negative points up.
    const spread = rand('angle');
    const angleDegrees = fromLeft ? lerp(spread, -78, -40) : lerp(spread, -140, -102);
    const angleRadians = (angleDegrees * Math.PI) / 180;
    const speed = lerp(rand('speed'), minSpeed, maxSpeed) * scale;
    const delayRoll = rand('delay');
    const firstWaveDelay =
      delayRoll < firstVolleyShare
        ? 0
        : ((delayRoll - firstVolleyShare) / (1 - firstVolleyShare)) ** 1.5 * launchWindowSeconds;
    const secondWaveDelay =
      windowSeconds * lerp(rand('wave'), secondWaveWindowStart, secondWaveWindowEnd);
    // A particle joins the second volley only when that actually lands later
    // than the normal spread; on a short hold the two waves coincide, which is
    // the right behaviour rather than a case to special-case.
    const inSecondWave = rand('waveRoll') < secondWaveShare;

    return {
      color: palette[Math.floor(rand('color') * palette.length)] ?? palette[0] ?? '#f97316',
      delay: inSecondWave ? Math.max(firstWaveDelay, secondWaveDelay) : firstWaveDelay,
      drag: lerp(1 - sizeNorm, minDrag, maxDrag),
      gravity: gravity * scale,
      height: clamp(width * lerp(rand('ratio'), 0.25, 2.6), minHeight, maxHeight * scale),
      originX: (fromLeft ? cannonInset : 1 - cannonInset) * size,
      originY: cannonHeight * size,
      rotationSpeed: lerp(rand('spin'), -maxSpin, maxSpin),
      startRotation: lerp(rand('rotation'), 0, 360),
      swayAmplitude: lerp(rand('sway'), minSwayAmplitude, maxSwayAmplitude) * scale,
      swayHz: lerp(rand('hz'), minSwayHz, maxSwayHz),
      swayPhase: rand('phase') * Math.PI * 2,
      vx: Math.cos(angleRadians) * speed,
      vy: Math.sin(angleRadians) * speed,
      width,
    };
  });
};

/**
 * Evaluates a particle's position, orientation and streak at `seconds` after the
 * burst.
 *
 * Closed-form solution of `v' = g - drag * v`, so position depends only on the
 * elapsed time and never on a previous frame. Before its launch delay a
 * particle sits at the muzzle.
 *
 * `stretch` and `rotation` together fake motion blur. While a chip is moving
 * fast it is drawn elongated and aligned with its travel direction; as it slows
 * the stretch relaxes to 1 and the orientation blends back to its own tumble.
 * Without this a chip crossing 50px between frames reads as two unrelated dots.
 *
 * @param {ConfettiParticle} particle
 * @param {number} seconds elapsed since the burst started
 * @returns {{rotation: number, stretch: number, x: number, y: number}}
 */
export const getConfettiParticleState = (particle, seconds) => {
  const t = Math.max(0, seconds - particle.delay);
  const fade = Math.exp(-particle.drag * t);
  const decay = 1 - fade;
  const terminalSpeed = particle.gravity / particle.drag;
  // Sway eases in so particles do not start mid-swing at the muzzle.
  const sway =
    Math.sin(particle.swayPhase + t * particle.swayHz * Math.PI * 2) *
    particle.swayAmplitude *
    Math.min(1, t * 2);
  // Velocity of the same closed form, differentiated.
  const velocityX = particle.vx * fade;
  const velocityY = particle.vy * fade + terminalSpeed * decay;
  const speedNow = Math.hypot(velocityX, velocityY);
  const stretch = 1 + Math.min(maxStretch - 1, speedNow / stretchReferenceSpeed);
  // Blend from travel-aligned while streaking to free tumble once settled.
  const alignment = Math.min(1, Math.max(0, (stretch - 1) / (maxStretch - 1)));
  const travelDegrees = (Math.atan2(velocityY, velocityX) * 180) / Math.PI;
  const tumbleDegrees = particle.startRotation + particle.rotationSpeed * t;
  // Shortest path: tumble grows without bound while travel wraps at +/-180, so
  // a raw difference would swing the chip the long way around.
  const towardTravel = (((travelDegrees - tumbleDegrees) % 360) + 540) % 360 - 180;

  return {
    rotation: tumbleDegrees + alignment * towardTravel,
    stretch,
    x: particle.originX + (particle.vx / particle.drag) * decay + sway,
    y:
      particle.originY +
      ((particle.vy - terminalSpeed) / particle.drag) * decay +
      terminalSpeed * t,
  };
};
