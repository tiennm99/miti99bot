import {renderComposition} from './render-composition.js';

/**
 * @typedef {import('../schemas/gacha-request.js').GachaRenderRequest} GachaRenderRequest
 * @typedef {import('./render-composition.js').RenderResult} RenderGachaResult
 */

/**
 * Builds a renderer for one wish composition. Output is a silent H.264 MP4,
 * which Telegram plays as a looping animation and which keeps the glow
 * gradients free of GIF banding.
 *
 * @param {string} compositionId
 * @returns {(inputProps: GachaRenderRequest, options: {timeoutInMilliseconds: number}) => Promise<RenderGachaResult>}
 */
const createGachaRenderer = (compositionId) => (inputProps, options) =>
  renderComposition({
    compositionId,
    filename: 'gacha.mp4',
    inputProps: {...inputProps},
    media: {codec: 'h264', crf: 23, imageFormat: 'jpeg', jpegQuality: 90, muted: true},
    timeoutInMilliseconds: options.timeoutInMilliseconds,
  });

export const renderGachaVideo = createGachaRenderer('GachaWish');

/** The beta wish: toon-shaded 3D-perspective sky, comet through a cloud, SSS reveal. */
export const renderGachaBetaVideo = createGachaRenderer('GachaBetaWish');
