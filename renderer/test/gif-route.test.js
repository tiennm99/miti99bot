import {describe, expect, test} from 'vitest';
import {RenderTimeoutError} from '../src/lib/render-errors.js';
import {buildServer} from '../src/server.js';

const config = {
  apiToken: undefined,
  host: '127.0.0.1',
  maxConcurrentRenders: 1,
  maxOptionChars: 40,
  maxOptions: 32,
  port: 0,
  requiresApiToken: false,
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
        options: ['only one'],
      },
    });

    expect(response.statusCode).toBe(400);
    expect(response.json().error).toBe('invalid_request');

    await app.close();
  });

  test('enforces bearer token when configured', async () => {
    const app = await buildServer({
      config: {...config, apiToken: 'secret'},
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
        options: ['a', 'b'],
      },
    });

    expect(response.statusCode).toBe(401);

    await app.close();
  });

  test('rejects concurrent renders beyond the limit and frees the slot afterward', async () => {
    /** @type {(value?: void) => void} */
    let releaseRender = () => {};
    /** @type {(value?: void) => void} */
    let markRenderStarted = () => {};
    const renderStarted = new Promise((resolve) => {
      markRenderStarted = resolve;
    });
    const renderGate = new Promise((resolve) => {
      releaseRender = resolve;
    });

    const app = await buildServer({
      config,
      renderGif: async () => {
        markRenderStarted();
        await renderGate;
        return {buffer: Buffer.from('GIF89a-test'), byteLength: 11, durationMs: 1};
      },
    });

    const payload = {options: ['alpha', 'beta'], winnerIndex: 0};
    const first = app.inject({method: 'POST', url: '/api/gif', payload});
    // The route acquires the semaphore before calling renderGif, so once the
    // render body runs the single slot is held.
    await renderStarted;

    const rejected = await app.inject({method: 'POST', url: '/api/gif', payload});
    expect(rejected.statusCode).toBe(429);
    expect(rejected.json().error).toBe('too_many_renders');

    releaseRender();
    expect((await first).statusCode).toBe(200);

    // The slot released in `finally` must let a later request through.
    const afterRelease = await app.inject({method: 'POST', url: '/api/gif', payload});
    expect(afterRelease.statusCode).toBe(200);

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
