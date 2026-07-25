export const fullTurnDegrees = 360;

/** Turns the wheel makes before landing on the winner. */
export const defaultFullTurns = 5;

/**
 * @param {number} optionCount
 * @returns {number}
 */
export const getSliceDegrees = (optionCount) => fullTurnDegrees / optionCount;

/**
 * Angles use CSS coordinates: 0deg points to 3h and positive values rotate
 * clockwise on screen.
 *
 * @param {number} optionCount
 * @param {number} index
 * @returns {number}
 */
export const getSliceCenterDegrees = (optionCount, index) =>
  index * getSliceDegrees(optionCount) + getSliceDegrees(optionCount) / 2;

/**
 * @param {number} optionCount
 * @param {number} winnerIndex
 * @param {number} fullTurns
 * @returns {number}
 */
export const getFinalWheelRotationDegrees = (optionCount, winnerIndex, fullTurns = defaultFullTurns) =>
  fullTurns * fullTurnDegrees - getSliceCenterDegrees(optionCount, winnerIndex);

/**
 * Converts CSS angle convention to @remotion/shapes Pie radians. Pie rotation
 * 0 starts at 12h, so CSS 0deg at 3h needs a quarter-turn offset.
 *
 * @param {number} cssDegrees
 * @returns {number}
 */
export const cssDegreesToPieRadians = (cssDegrees) => ((cssDegrees - 270) * Math.PI) / 180;

/**
 * @param {number} value
 * @param {number} min
 * @param {number} max
 * @returns {number}
 */
const clamp = (value, min, max) => Math.min(Math.max(value, min), max);

/**
 * Fraction of the spin spent accelerating from rest.
 *
 * A pure ease-out starts at maximum speed, which pushes the wheel past one
 * slice per frame at 15fps and reads as a strobe or a backwards spin. A short
 * launch ramp keeps peak speed under the slice width.
 */
const accelFraction = 0.12;

/** Deceleration curve exponent; higher means a longer, softer approach. */
const decelExponent = 2.2;

/**
 * Speed at the handoff between launch and deceleration, in progress units.
 *
 * Solved so the two segments share a slope there. Without this the wheel jumps
 * from ~18 to ~41 degrees per frame in a single frame and looks like it gets a
 * second push.
 */
const handoffSlope = decelExponent / (1 - accelFraction + (decelExponent * accelFraction) / 2);

/** Progress already covered when the launch ramp ends. */
const handoffProgress = (handoffSlope * accelFraction) / 2;

/** Widest turn count allowed, so a sparse wheel does not spin absurdly. */
const maxFullTurns = 7;

/**
 * Turns to spin so the wheel never advances more than one slice per frame.
 *
 * Peak speed is `handoffSlope * turns * 360 / spinFrames` degrees per frame.
 * Holding that at or under one slice (`360 / optionCount`) means a viewer can
 * always track which wedge is passing; above it the wheel aliases and appears to
 * jitter or run backwards. Solving for turns gives the expression below.
 *
 * Floors rather than rounds: rounding 5.63 up to 6 turns on an 8-option wheel
 * would push peak speed to 47.9deg against a 45deg slice, i.e. introduce the
 * very aliasing this prevents.
 *
 * The result must stay an integer. The winner lands under the pointer only
 * because `turns * 360` is a whole number of revolutions; 2.8 turns would leave
 * the wheel 288 degrees from its detent.
 *
 * @param {number} spinFrames frames available before the hold
 * @param {number} optionCount number of slices
 * @returns {number} integer turn count
 */
export const getSpinTurns = (spinFrames, optionCount) =>
  Math.max(
    1,
    Math.min(maxFullTurns, Math.floor(spinFrames / (handoffSlope * Math.max(1, optionCount)))),
  );

/**
 * @param {number} frame
 * @param {number} spinFrames
 * @returns {number}
 */
export const getSpinProgress = (frame, spinFrames) => {
  const progress = clamp(frame / Math.max(1, spinFrames), 0, 1);

  if (progress < accelFraction) {
    return (handoffSlope * progress * progress) / (2 * accelFraction);
  }

  const decelProgress = (progress - accelFraction) / (1 - accelFraction);

  return handoffProgress + (1 - (1 - decelProgress) ** decelExponent) * (1 - handoffProgress);
};

/**
 * @param {object} input
 * @param {number} input.frame
 * @param {number} input.spinFrames
 * @param {number} input.startRotationDegrees
 * @param {number} input.finalRotationDegrees
 * @returns {number}
 */
export const getWheelRotationDegrees = ({frame, spinFrames, startRotationDegrees, finalRotationDegrees}) =>
  startRotationDegrees +
  (finalRotationDegrees - startRotationDegrees) * getSpinProgress(frame, spinFrames);

/** Portion of the spin over which the wheel rocks into its detent. */
const recoilFraction = 0.18;

/** Peak rock in degrees, before the slice-width cap below. */
const maxRecoilDegrees = 2.2;

/**
 * Rotation offset that rocks the wheel into its final position.
 *
 * A pure ease-out creeps to a halt, which reads as the wheel being switched off
 * rather than coming to rest. This overlays one damped swing over the tail of
 * the spin: the wheel drifts a little past its detent, comes back, and settles.
 *
 * Kept as an offset on top of the rotation rather than folded into
 * `getSpinProgress` so progress stays monotonic in [0, 1]. The damping term
 * forces the offset to exactly 0 at `spinFrames`, so the winner still lands
 * precisely under the pointer and the flapper still parks neutral.
 *
 * @param {number} frame
 * @param {number} spinFrames
 * @param {number} sliceDegrees width of one slice
 * @returns {number} degrees to add to the wheel rotation
 */
export const getSpinRecoilDegrees = (frame, spinFrames, sliceDegrees) => {
  const safeSpinFrames = Math.max(1, spinFrames);
  const startFrame = safeSpinFrames * (1 - recoilFraction);

  if (frame <= startFrame || frame >= safeSpinFrames) {
    return 0;
  }

  const progress = (frame - startFrame) / (safeSpinFrames - startFrame);
  // Capped against the slice so a 2-option wheel does not visibly swing and a
  // dense one never rocks a neighbouring wedge under the pointer.
  const amplitude = Math.min(maxRecoilDegrees, sliceDegrees * 0.25);

  return amplitude * Math.sin(progress * Math.PI * 2) * (1 - progress) ** 1.5;
};

/**
 * Deflection of the pointer flapper as slices pass under it.
 *
 * The flapper is pushed aside as a slice edge arrives and springs back over the
 * rest of the slice, which is the cue that sells the wheel as a physical
 * object. Resolves to exactly 0 at rest: the final rotation always leaves a
 * remainder of half a slice, so `phase` is 0.5 and the flapper parks neutral
 * for every winner and option count.
 *
 * @param {number} rotationDegrees current wheel rotation
 * @param {number} sliceDegrees width of one slice
 * @param {number} [maxDeflection] peak deflection in degrees
 * @returns {number}
 */
export const getPointerDeflectionDegrees = (rotationDegrees, sliceDegrees, maxDeflection = 9) => {
  const safeSliceDegrees = Math.max(1e-6, sliceDegrees);
  const phase = (((rotationDegrees % safeSliceDegrees) + safeSliceDegrees) % safeSliceDegrees) / safeSliceDegrees;
  const push = Math.max(0, 1 - phase * 3);

  return push === 0 ? 0 : -maxDeflection * push;
};
