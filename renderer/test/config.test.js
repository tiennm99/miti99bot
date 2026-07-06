import {describe, expect, test} from 'vitest';
import {loadConfig, minRenderTimeoutMs} from '../src/config.js';

describe('loadConfig', () => {
  test('keeps render timeout compatible with Remotion browser timeout limits', () => {
    const config = loadConfig({RENDER_TIMEOUT_MS: '500'});

    expect(config.renderTimeoutMs).toBe(minRenderTimeoutMs);
  });

  test('requires API_TOKEN in production', () => {
    expect(() => loadConfig({NODE_ENV: 'production'})).toThrow(/API_TOKEN/);
  });

  test('accepts API_TOKEN in production', () => {
    const config = loadConfig({NODE_ENV: 'production', API_TOKEN: 'secret'});

    expect(config.requiresApiToken).toBe(true);
    expect(config.apiToken).toBe('secret');
  });
});
