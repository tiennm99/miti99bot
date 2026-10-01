import {readFile} from 'node:fs/promises';
import path from 'node:path';
import {makeCancelSignal, renderMedia, selectComposition} from '@remotion/renderer';
import {minRenderTimeoutMs} from '../config.js';
import {RenderTimeoutError} from '../lib/render-errors.js';
import {cleanupTempDir, createRenderTempDir} from '../lib/tmp-files.js';
import {getRemotionServeUrl} from './remotion-bundle.js';

/**
 * @typedef {object} RenderCompositionOptions
 * @property {string} compositionId
 * @property {Record<string, unknown>} inputProps
 * @property {string} filename
 * @property {{codec: import('@remotion/renderer').Codec} & Partial<import('@remotion/renderer').RenderMediaOptions>} media Codec and encoder settings.
 * @property {number} timeoutInMilliseconds
 */

/**
 * @typedef {object} RenderResult
 * @property {Buffer} buffer
 * @property {number} durationMs
 * @property {number} byteLength
 */

/**
 * Renders one composition from the shared bundle into memory, cancelling the
 * render once the total timeout elapses.
 *
 * @param {RenderCompositionOptions} options
 * @returns {Promise<RenderResult>}
 */
export const renderComposition = async (options) => {
  const startedAt = Date.now();
  const tempDir = await createRenderTempDir();
  const outputLocation = path.join(tempDir, options.filename);
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
      id: options.compositionId,
      inputProps: options.inputProps,
      serveUrl,
      timeoutInMilliseconds: rendererTimeout,
    });

    await renderMedia({
      ...options.media,
      cancelSignal,
      composition,
      inputProps: options.inputProps,
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
