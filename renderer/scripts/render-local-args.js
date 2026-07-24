import {parseArgs} from 'node:util';
import {pickWinnerIndex} from '../src/lib/winner.js';
import {parseWheelRequest} from '../src/schemas/wheel-request.js';

const limits = {
  maxOptions: 32,
  maxOptionChars: 40,
};

/**
 * @param {string | undefined} value
 * @param {string} flag
 * @returns {number | undefined}
 */
const parseInteger = (value, flag) => {
  if (value === undefined) {
    return undefined;
  }

  if (!/^\d+$/.test(value)) {
    throw new Error(`--${flag} must be an integer`);
  }

  return Number.parseInt(value, 10);
};

/**
 * @typedef {(
 *   {help: true, output: string, timeoutInMilliseconds: number, request: undefined} |
 *   {help: false, output: string, timeoutInMilliseconds: number, request: import('../src/schemas/wheel-request.js').WheelRenderRequest}
 * )} RenderLocalArguments
 */

/**
 * @param {string[]} args
 * @returns {RenderLocalArguments}
 */
export const parseRenderLocalArgs = (args) => {
  const {values} = parseArgs({
    args,
    allowPositionals: false,
    strict: true,
    options: {
      help: {type: 'boolean', short: 'h'},
      output: {type: 'string', short: 'o', default: 'wheel.gif'},
      option: {type: 'string', multiple: true},
      winner: {type: 'string'},
      duration: {type: 'string'},
      hold: {type: 'string'},
      fps: {type: 'string'},
      size: {type: 'string'},
      theme: {type: 'string'},
      timeout: {type: 'string', default: '30000'},
    },
  });

  const timeoutInMilliseconds = parseInteger(values.timeout, 'timeout');
  if (timeoutInMilliseconds === undefined || timeoutInMilliseconds < 7000) {
    throw new Error('--timeout must be at least 7000 milliseconds');
  }

  if (values.help) {
    return {
      help: true,
      output: values.output,
      request: undefined,
      timeoutInMilliseconds,
    };
  }

  /** @type {Record<string, unknown>} */
  const input = {options: values.option ?? []};
  const winnerIndex = parseInteger(values.winner, 'winner');
  const durationMs = parseInteger(values.duration, 'duration');
  const holdMs = parseInteger(values.hold, 'hold');
  const fps = parseInteger(values.fps, 'fps');
  const size = parseInteger(values.size, 'size');
  if (winnerIndex !== undefined) input.winnerIndex = winnerIndex;
  if (durationMs !== undefined) input.durationMs = durationMs;
  if (holdMs !== undefined) input.holdMs = holdMs;
  if (fps !== undefined) input.fps = fps;
  if (size !== undefined) input.size = size;
  if (values.theme !== undefined) {
    input.theme = values.theme;
  }

  return {
    help: false,
    output: values.output,
    request: parseWheelRequest(input, limits, pickWinnerIndex),
    timeoutInMilliseconds,
  };
};
