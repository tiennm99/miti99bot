import {describe, expect, test} from 'vitest';
import {parseWheelRequest} from '../src/schemas/wheel-request.js';

const limits = {
  maxOptionChars: 12,
  maxOptions: 4,
};

describe('parseWheelRequest', () => {
  test('trims options and preserves provided winner', () => {
    const request = parseWheelRequest(
      {
        options: [' alpha ', ' beta '],
        winnerIndex: 1,
      },
      limits,
      () => 0,
    );

    expect(request.options).toEqual(['alpha', 'beta']);
    expect(request.winnerIndex).toBe(1);
    expect(request.durationMs).toBe(6500);
    expect(request.fps).toBe(15);
  });

  test('rejects blank options so winner index cannot shift after trimming', () => {
    expect(() =>
      parseWheelRequest(
        {
          options: ['', 'alpha', 'beta'],
          winnerIndex: 1,
        },
        limits,
        () => 0,
      ),
    ).toThrow();
  });

  test('selects winner when omitted', () => {
    const request = parseWheelRequest({options: ['a', 'b']}, limits, () => 1);
    expect(request.winnerIndex).toBe(1);
  });

  test('rejects winner outside option range', () => {
    expect(() =>
      parseWheelRequest(
        {
          options: ['a', 'b'],
          winnerIndex: 2,
        },
        limits,
        () => 0,
      ),
    ).toThrow(/winnerIndex/);
  });

  test('rejects too many options', () => {
    expect(() => parseWheelRequest({options: ['a', 'b', 'c', 'd', 'e']}, limits, () => 0)).toThrow();
  });
});
