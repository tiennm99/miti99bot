import {renderComposition} from './render-composition.js';

/**
 * @typedef {import('../schemas/wheel-request.js').WheelRenderRequest} WheelRenderRequest
 * @typedef {import('./render-composition.js').RenderResult} RenderGifResult
 */

/**
 * @typedef {object} RenderWheelGifOptions
 * @property {number} timeoutInMilliseconds
 */

/**
 * @param {WheelRenderRequest} inputProps
 * @param {RenderWheelGifOptions} options
 * @returns {Promise<RenderGifResult>}
 */
export const renderWheelGif = (inputProps, options) =>
  renderComposition({
    compositionId: 'WheelGif',
    filename: 'wheelofnames.gif',
    inputProps: {...inputProps},
    media: {codec: 'gif', imageFormat: 'png'},
    timeoutInMilliseconds: options.timeoutInMilliseconds,
  });
