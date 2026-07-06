import {readFile} from 'node:fs/promises';
import path from 'node:path';
import {makeCancelSignal, renderMedia, selectComposition} from '@remotion/renderer';
import {minRenderTimeoutMs} from '../config.js';
import {RenderTimeoutError} from '../lib/render-errors.js';
import {cleanupTempDir, createRenderTempDir} from '../lib/tmp-files.js';
import {getRemotionServeUrl} from './remotion-bundle.js';

/**
 * @typedef {import('../schemas/wheel-request.js').WheelRenderRequest} WheelRenderRequest
 */

/**
 * @typedef {object} RenderWheelGifOptions
 * @property {number} timeoutInMilliseconds
 */

/**
 * @typedef {object} RenderGifResult
 * @property {Buffer} buffer
 * @property {number} durationMs
 * @property {number} byteLength
 */

/**
 * @param {WheelRenderRequest} inputProps
 * @param {RenderWheelGifOptions} options
 * @returns {Promise<RenderGifResult>}
 */
export const renderWheelGif = async (inputProps, options) => {
  const startedAt = Date.now();
  const tempDir = await createRenderTempDir();
  const outputLocation = path.join(tempDir, 'wheelofnames.gif');
  const {cancel, cancelSignal} = makeCancelSignal();
  let timedOut = false;
  const timeoutId = setTimeout(() => {
    timedOut = true;
    cancel();
  }, options.timeoutInMilliseconds);
  const rendererTimeout = Math.max(minRenderTimeoutMs, options.timeoutInMilliseconds);

  try {
    const serveUrl = await getRemotionServeUrl();
    const composition = await selectComposition({
      id: 'WheelGif',
      inputProps,
      serveUrl,
      timeoutInMilliseconds: rendererTimeout,
    });

    await renderMedia({
      cancelSignal,
      codec: 'gif',
      composition,
      imageFormat: 'png',
      inputProps,
      outputLocation,
      overwrite: true,
      serveUrl,
      timeoutInMilliseconds: rendererTimeout,
    });

    const buffer = await readFile(outputLocation);
    return {
      buffer,
      byteLength: buffer.byteLength,
      durationMs: Date.now() - startedAt,
    };
  } catch (error) {
    if (timedOut || Date.now() - startedAt >= options.timeoutInMilliseconds) {
      throw new RenderTimeoutError(options.timeoutInMilliseconds, {cause: error});
    }

    throw error;
  } finally {
    clearTimeout(timeoutId);
    await cleanupTempDir(tempDir);
  }
};
