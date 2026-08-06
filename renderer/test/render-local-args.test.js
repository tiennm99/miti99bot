import {describe, expect, test} from 'vitest';
import {parseRenderLocalArgs} from '../scripts/render-local-args.js';

describe('parseRenderLocalArgs', () => {
  test('preserves repeated Unicode options and parses render settings', () => {
    const parsed = parseRenderLocalArgs([
      '--output',
      'custom.gif',
      '--option',
      'Chiều nay uống CraneTea',
      '--option',
      'Chiều nay uống CraneTea',
      '--option',
      'Cà phê',
      '--winner',
      '1',
      '--duration',
      '7000',
      '--hold',
      '1500',
      '--fps',
      '20',
      '--size',
      '480',
      '--theme',
      'festival',
    ]);

    expect(parsed.output).toBe('custom.gif');
    expect(parsed.request).toEqual({
      options: ['Chiều nay uống CraneTea', 'Chiều nay uống CraneTea', 'Cà phê'],
      winnerIndex: 1,
      durationMs: 7000,
      holdMs: 1500,
      fps: 20,
      size: 480,
      theme: 'festival',
    });
  });

  test('uses render defaults', () => {
    const parsed = parseRenderLocalArgs([
      '--option',
      'alpha',
      '--option',
      'beta',
      '--winner',
      '0',
    ]);

    expect(parsed.output).toBe('wheel.gif');
    expect(parsed.timeoutInMilliseconds).toBe(30000);
    expect(parsed.request).toMatchObject({
      options: ['alpha', 'beta'],
      winnerIndex: 0,
      durationMs: 6500,
      holdMs: 1200,
      fps: 15,
      size: 512,
      theme: 'classic',
    });
  });

  test('rejects missing options and invalid integer flags', () => {
    expect(() => parseRenderLocalArgs([])).toThrow();
    expect(() =>
      parseRenderLocalArgs(['--option', 'alpha', '--option', 'beta', '--winner', '1.5']),
    ).toThrow('--winner must be an integer');
  });

  test('returns help without requiring options', () => {
    expect(parseRenderLocalArgs(['--help'])).toMatchObject({help: true, request: undefined});
  });
});

