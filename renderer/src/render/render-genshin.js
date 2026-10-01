import {renderComposition} from './render-composition.js';

/**
 * @typedef {import('../schemas/gacha-request.js').GachaRenderRequest} GachaRenderRequest
 * @typedef {import('./render-composition.js').RenderResult} RenderResult
 */

/**
 * Renders the Genshin-style meteor wish as a silent H.264 MP4, which Telegram
 * plays as a looping animation and which keeps the glow gradients free of GIF
 * banding.
 *
 * @param {GachaRenderRequest} inputProps
 * @param {{timeoutInMilliseconds: number}} options
 * @returns {Promise<RenderResult>}
 */
export const renderGenshinVideo = (inputProps, options) =>
  renderComposition({
    compositionId: 'GenshinWish',
    filename: 'genshin.mp4',
    inputProps: {...inputProps},
    media: {codec: 'h264', crf: 23, imageFormat: 'jpeg', jpegQuality: 90, muted: true},
    timeoutInMilliseconds: options.timeoutInMilliseconds,
  });
