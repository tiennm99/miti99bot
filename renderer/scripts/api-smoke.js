import {buildServer} from '../src/server.js';

const config = {
  apiToken: 'smoke-token',
  host: '127.0.0.1',
  maxConcurrentRenders: 1,
  maxOptionChars: 40,
  maxOptions: 32,
  port: 0,
  requiresApiToken: true,
  renderTimeoutMs: 30000,
};

const app = await buildServer({config, warmRemotionBundle: true});

try {
  const response = await app.inject({
    method: 'POST',
    url: '/api/gif',
    headers: {
      authorization: 'Bearer smoke-token',
    },
    payload: {
      durationMs: 3000,
      fps: 12,
      holdMs: 500,
      options: ['alice', 'bob', 'carol', 'dave'],
      size: 384,
      theme: 'classic',
      winnerIndex: 2,
    },
  });

  if (response.statusCode !== 200) {
    throw new Error(`Expected 200, got ${response.statusCode}: ${response.body}`);
  }

  const magic = response.rawPayload.subarray(0, 6).toString();
  if (magic !== 'GIF89a') {
    throw new Error(`Expected GIF89a header, got ${magic}`);
  }

  if (response.headers['x-wheel-winner-index'] !== '2') {
    throw new Error(`Expected winner index 2, got ${response.headers['x-wheel-winner-index']}`);
  }

  console.log(`api-smoke: ${response.rawPayload.byteLength} bytes`);
} finally {
  await app.close();
}
