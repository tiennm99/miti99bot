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
