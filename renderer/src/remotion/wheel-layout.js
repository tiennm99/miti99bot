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
 * Converts CSS angle convention to @remotion/shapes Pie convention, where 0deg
 * starts at 12h.
 *
 * @param {number} cssDegrees
 * @returns {number}
 */
export const cssDegreesToPieRadians = (cssDegrees) => ((cssDegrees - 90) * Math.PI) / 180;
