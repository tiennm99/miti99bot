export const fullTurnDegrees = 360;
export const rightPointerClipPath = 'polygon(0 50%, 100% 0, 100% 100%)';

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
export const getFinalWheelRotationDegrees = (optionCount, winnerIndex, fullTurns = 7) =>
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
 * @param {number} frame
 * @param {number} spinFrames
 * @returns {number}
 */
export const getSpinProgress = (frame, spinFrames) => {
  const safeSpinFrames = Math.max(1, spinFrames);
  const progress = clamp(frame / safeSpinFrames, 0, 1);

  return 1 - (1 - progress) ** 3;
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
