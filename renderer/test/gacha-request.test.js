import {describe, expect, test} from 'vitest';
import {parseGachaRequest} from '../src/schemas/gacha-request.js';

const limits = {maxOptionChars: 12};

describe('parseGachaRequest', () => {
  test('trims the label and applies defaults', () => {
    const request = parseGachaRequest({label: ' Pizza ', rarity: 5}, limits);

    expect(request).toEqual({label: 'Pizza', rarity: 5, fps: 24, width: 640});
  });

  test('accepts the larger frame and fps', () => {
    const request = parseGachaRequest({label: 'Pho', rarity: 4, fps: 30, width: 854}, limits);

    expect(request.fps).toBe(30);
    expect(request.width).toBe(854);
  });

  test.each([
    {label: 'Pizza', rarity: 2},
    {label: 'Pizza', rarity: 6},
    {label: 'Pizza'},
    {label: '   ', rarity: 3},
    {label: 'x'.repeat(13), rarity: 3},
    {label: 'Pizza', rarity: 3, width: 512},
    {label: 'Pizza', rarity: 3, fps: 15},
    {label: 'Pizza', rarity: 3, extra: true},
  ])('rejects %o', (input) => {
    expect(() => parseGachaRequest(input, limits)).toThrow();
  });
});
