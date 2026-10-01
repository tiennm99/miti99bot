import Fastify from 'fastify';
import {loadConfig} from './config.js';
import {createRenderSemaphore} from './lib/render-semaphore.js';
import {getRemotionServeUrl} from './render/remotion-bundle.js';
import {closeGachaBetaBrowser, renderGachaBetaVideo, warmGachaBetaBrowser} from './render/render-gacha-beta.js';
import {renderGachaVideo} from './render/render-gacha.js';
import {renderWheelGif} from './render/render-gif.js';
import {registerGachaRoute} from './routes/gacha.js';
import {registerGifRoute} from './routes/gif.js';
import {registerHealthRoute} from './routes/health.js';

/**
 * @typedef {import('./config.js').AppConfig} AppConfig
 * @typedef {import('./schemas/wheel-request.js').WheelRenderRequest} WheelRenderRequest
 * @typedef {import('./schemas/gacha-request.js').GachaRenderRequest} GachaRenderRequest
 */

/**
 * @param {object} [options]
 * @param {AppConfig} [options.config]
 * @param {(request: WheelRenderRequest, renderOptions: {timeoutInMilliseconds: number}) => Promise<{buffer: Buffer, durationMs: number, byteLength: number}>} [options.renderGif]
 * @param {(request: GachaRenderRequest, renderOptions: {timeoutInMilliseconds: number}) => Promise<{buffer: Buffer, durationMs: number, byteLength: number}>} [options.renderGacha]
 * @param {(request: GachaRenderRequest, renderOptions: {timeoutInMilliseconds: number}) => Promise<{buffer: Buffer, durationMs: number, byteLength: number}>} [options.renderGachaBeta]
 * @param {boolean} [options.warmRenderers]  Prepare the Remotion bundle and the beta wish browser before serving.
 */
export const buildServer = async (options = {}) => {
  const config = options.config ?? loadConfig();
  const warmRenderers = options.warmRenderers ?? process.env.NODE_ENV !== 'test';

  if (warmRenderers) {
    await getRemotionServeUrl();
    await warmGachaBetaBrowser();
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
  // One semaphore across both routes: they share the same browser and CPU.
  const semaphore = createRenderSemaphore(config.maxConcurrentRenders);
  await registerGifRoute(app, {
    config,
    renderGif: options.renderGif ?? renderWheelGif,
    semaphore,
  });
  await registerGachaRoute(app, {
    config,
    renderGacha: options.renderGacha ?? renderGachaVideo,
    semaphore,
  });
  await registerGachaRoute(app, {
    config,
    path: '/api/gachabeta',
    renderGacha: options.renderGachaBeta ?? renderGachaBetaVideo,
    semaphore,
  });
  // The beta wish keeps one headless Chrome alive between renders.
  app.addHook('onClose', closeGachaBetaBrowser);

  return app;
};

if (import.meta.url === `file://${process.argv[1]}`) {
  const config = loadConfig();
  const app = await buildServer({config});
  await app.listen({host: config.host, port: config.port});
}
