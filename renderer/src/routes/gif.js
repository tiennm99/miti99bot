import {ZodError} from 'zod';
import {isRenderTimeoutError} from '../lib/render-errors.js';
import {pickWinnerIndex} from '../lib/winner.js';
import {parseWheelRequest} from '../schemas/wheel-request.js';

/**
 * @typedef {import('../config.js').AppConfig} AppConfig
 * @typedef {import('../lib/render-semaphore.js').createRenderSemaphore} CreateRenderSemaphore
 * @typedef {import('../schemas/wheel-request.js').WheelRenderRequest} WheelRenderRequest
 */

/**
 * @typedef {object} GifRouteDeps
 * @property {AppConfig} config
 * @property {{tryAcquire: () => boolean, release: () => void}} semaphore
 * @property {(request: WheelRenderRequest, options: {timeoutInMilliseconds: number}) => Promise<{buffer: Buffer, durationMs: number, byteLength: number}>} renderGif
 */

/**
 * @param {unknown} issues
 */
const formatValidationError = (issues) => ({
  error: 'invalid_request',
  issues,
});

/**
 * @param {import('fastify').FastifyInstance} app
 * @param {GifRouteDeps} deps
 */
export const registerGifRoute = async (app, deps) => {
  app.post('/api/gif', async (request, reply) => {
    let wheelRequest;
    try {
      wheelRequest = parseWheelRequest(
        request.body,
        {
          maxOptionChars: deps.config.maxOptionChars,
          maxOptions: deps.config.maxOptions,
        },
        pickWinnerIndex,
      );
    } catch (error) {
      if (error instanceof ZodError) {
        return reply.code(400).send(formatValidationError(error.issues));
      }
      throw error;
    }

    if (!deps.semaphore.tryAcquire()) {
      return reply.code(429).send({error: 'too_many_renders'});
    }

    try {
      const result = await deps.renderGif(wheelRequest, {
        timeoutInMilliseconds: deps.config.renderTimeoutMs,
      });
      const winner = wheelRequest.options[wheelRequest.winnerIndex];
      if (winner === undefined) {
        throw new Error('winner index resolved outside options range');
      }

      return reply
        .header('Content-Type', 'image/gif')
        .header('Content-Disposition', 'inline; filename="wheelofnames.gif"')
        .header('Cache-Control', 'no-store')
        .header('X-Wheel-Winner-Index', String(wheelRequest.winnerIndex))
        .header('X-Wheel-Winner', encodeURIComponent(winner))
        .header('X-Render-Duration-Ms', String(result.durationMs))
        .send(result.buffer);
    } catch (error) {
      if (isRenderTimeoutError(error)) {
        return reply.code(504).send({error: 'render_timeout'});
      }

      throw error;
    } finally {
      deps.semaphore.release();
    }
  });
};
