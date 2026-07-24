import {describe, expect, test} from 'vitest';
import {pickWinnerIndex} from '../src/lib/winner.js';

describe('pickWinnerIndex', () => {
  test('returns an index within range across many draws', () => {
    for (let i = 0; i < 100; i += 1) {
      const index = pickWinnerIndex(4);
      expect(Number.isInteger(index)).toBe(true);
      expect(index).toBeGreaterThanOrEqual(0);
      expect(index).toBeLessThan(4);
    }
  });

  test('always returns 0 for a single option', () => {
    expect(pickWinnerIndex(1)).toBe(0);
  });

  test('throws RangeError for non-positive or non-integer counts', () => {
    expect(() => pickWinnerIndex(0)).toThrow(RangeError);
    expect(() => pickWinnerIndex(-3)).toThrow(RangeError);
    expect(() => pickWinnerIndex(2.5)).toThrow(RangeError);
  });
});
