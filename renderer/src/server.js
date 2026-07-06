import Fastify from 'fastify';
import {loadConfig} from './config.js';
import {createRenderSemaphore} from './lib/render-semaphore.js';
import {getRemotionServeUrl} from './render/remotion-bundle.js';
import {renderWheelGif} from './render/render-gif.js';
import {registerGifRoute} from './routes/gif.js';
import {registerHealthRoute} from './routes/health.js';

/**
 * @typedef {import('./config.js').AppConfig} AppConfig
 * @typedef {import('./schemas/wheel-request.js').WheelRenderRequest} WheelRenderRequest
 */

/**
 * @param {object} [options]
 * @param {AppConfig} [options.config]
 * @param {(request: WheelRenderRequest, renderOptions: {timeoutInMilliseconds: number}) => Promise<{buffer: Buffer, durationMs: number, byteLength: number}>} [options.renderGif]
 * @param {boolean} [options.warmRemotionBundle]
 */
export const buildServer = async (options = {}) => {
  const config = options.config ?? loadConfig();
  const warmRemotionBundle = options.warmRemotionBundle ?? process.env.NODE_ENV !== 'test';

  if (warmRemotionBundle) {
    await getRemotionServeUrl();
  }

  const app = Fastify({
    bodyLimit: 32 * 1024,
    logger: process.env.NODE_ENV !== 'test',
  });

  app.setErrorHandler(async (error, _request, reply) => {
    app.log.error(error);
    return reply.code(500).send({error: 'internal_error'});
  });

  await registerHealthRoute(app);
  await registerGifRoute(app, {
    config,
    renderGif: options.renderGif ?? renderWheelGif,
    semaphore: createRenderSemaphore(config.maxConcurrentRenders),
  });

  return app;
};

if (import.meta.url === `file://${process.argv[1]}`) {
  const config = loadConfig();
  const app = await buildServer({config});
  await app.listen({host: config.host, port: config.port});
}
