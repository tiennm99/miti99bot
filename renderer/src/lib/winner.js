import {randomInt} from 'node:crypto';

/**
 * @param {number} optionCount
 * @returns {number}
 */
export const pickWinnerIndex = (optionCount) => {
  if (!Number.isInteger(optionCount) || optionCount < 1) {
    throw new RangeError('optionCount must be a positive integer');
  }

  return randomInt(optionCount);
};
