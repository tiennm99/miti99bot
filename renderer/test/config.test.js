import {describe, expect, test} from 'vitest';
import {loadConfig, minRenderTimeoutMs} from '../src/config.js';

const validEnv = {
  RENDERER_HOST: '0.0.0.0',
  RENDERER_PORT: '3000',
  RENDERER_MAX_CONCURRENT_RENDERS: '1',
  RENDERER_RENDER_TIMEOUT_MS: '15000',
  RENDERER_MAX_OPTIONS: '32',
  RENDERER_MAX_OPTION_CHARS: '40',
};

describe('loadConfig', () => {
  test('reads every RENDERER_* setting', () => {
    expect(loadConfig(validEnv)).toEqual({
      host: '0.0.0.0',
      port: 3000,
      maxConcurrentRenders: 1,
      renderTimeoutMs: 15000,
      maxOptions: 32,
      maxOptionChars: 40,
    });
  });

  test('keeps render timeout compatible with Remotion browser timeout limits', () => {
    const config = loadConfig({...validEnv, RENDERER_RENDER_TIMEOUT_MS: '500'});

    expect(config.renderTimeoutMs).toBe(minRenderTimeoutMs);
  });

  test('has no fallback: missing settings are all reported', () => {
    expect(() => loadConfig({})).toThrow(
      /RENDERER_HOST is required.*RENDERER_PORT is required.*RENDERER_MAX_OPTION_CHARS is required/,
    );
  });

  test('treats an empty value as missing', () => {
    expect(() => loadConfig({...validEnv, RENDERER_MAX_OPTIONS: ''})).toThrow(
      /RENDERER_MAX_OPTIONS is required/,
    );
  });

  test.each(['0', '-1', '1.5', 'abc'])('rejects non-positive-integer %s', (value) => {
    expect(() => loadConfig({...validEnv, RENDERER_MAX_OPTIONS: value})).toThrow(
      /RENDERER_MAX_OPTIONS must be a positive integer/,
    );
  });
});
