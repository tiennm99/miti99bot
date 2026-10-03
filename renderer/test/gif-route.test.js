import {describe, expect, test} from 'vitest';
import {RenderTimeoutError} from '../src/lib/render-errors.js';
import {buildServer} from '../src/server.js';

const config = {
  host: '127.0.0.1',
  maxConcurrentRenders: 1,
  maxOptionChars: 40,
  maxOptions: 32,
  port: 0,
  renderTimeoutMs: 15000,
};

describe('POST /api/gif', () => {
  test('returns gif bytes and winner metadata', async () => {
    const app = await buildServer({
      config,
      renderGif: async () => ({
        buffer: Buffer.from('GIF89a-test'),
        byteLength: 11,
        durationMs: 12,
      }),
    });

    const response = await app.inject({
      method: 'POST',
      url: '/api/gif',
      payload: {
        options: ['alpha', 'beta', 'gamma'],
        winnerIndex: 1,
      },
    });

    expect(response.statusCode).toBe(200);
    expect(response.headers['content-type']).toBe('image/gif');
    expect(response.headers['x-wheel-winner-index']).toBe('1');
    expect(response.headers['x-wheel-winner']).toBe('beta');
    expect(response.rawPayload.subarray(0, 6).toString()).toBe('GIF89a');

    await app.close();
  });

  test('rejects invalid payload', async () => {
    const app = await buildServer({
      config,
      renderGif: async () => ({
        buffer: Buffer.from('GIF89a-test'),
        byteLength: 11,
        durationMs: 12,
      }),
    });

    const response = await app.inject({
      method: 'POST',
      url: '/api/gif',
      payload: {
        options: [],
      },
    });

    expect(response.statusCode).toBe(400);
    expect(response.json().error).toBe('invalid_request');

    await app.close();
  });

  test('returns timeout response when rendering exceeds configured limit', async () => {
    const app = await buildServer({
      config: {...config, renderTimeoutMs: 7},
      renderGif: async (_request, renderOptions) => {
        expect(renderOptions.timeoutInMilliseconds).toBe(7);
        throw new RenderTimeoutError(renderOptions.timeoutInMilliseconds);
      },
    });

    const response = await app.inject({
      method: 'POST',
      url: '/api/gif',
      payload: {
        options: ['alpha', 'beta'],
      },
    });

    expect(response.statusCode).toBe(504);
    expect(response.json().error).toBe('render_timeout');

    await app.close();
  });
});
