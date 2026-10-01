/**
 * Minimal Chrome DevTools Protocol client over `--remote-debugging-pipe`.
 * Chrome reads NUL-terminated JSON messages on file descriptor 3 and writes
 * them on 4, so no debugging port is opened.
 */

/**
 * @typedef {object} CdpMessage
 * @property {number} [id]
 * @property {string} [method]
 * @property {string} [sessionId]
 * @property {Record<string, any>} [params]
 * @property {Record<string, any>} [result]
 * @property {{message: string}} [error]
 */

/**
 * @typedef {(params: Record<string, any>, sessionId: string | undefined) => void} CdpListener
 */

/**
 * @typedef {object} CdpConnection
 * @property {(method: string, params?: Record<string, unknown>, sessionId?: string) => Promise<Record<string, any>>} send
 * @property {(method: string, listener: CdpListener) => () => void} on  Returns a function that removes the listener.
 * @property {(method: string, sessionId: string) => Promise<Record<string, any>>} once  Next event with this name from one session.
 * @property {(error: Error) => void} fail  Rejects every pending call, for example when Chrome exits.
 */

/**
 * @param {NodeJS.WritableStream} input   Chrome's fd 3.
 * @param {NodeJS.ReadableStream} output  Chrome's fd 4.
 * @returns {CdpConnection}
 */
export const createCdpConnection = (input, output) => {
  let nextId = 0;
  /** @type {Map<number, {resolve: (value: Record<string, any>) => void, reject: (error: Error) => void, method: string}>} */
  const pending = new Map();
  /** @type {Map<string, Set<CdpListener>>} */
  const listeners = new Map();
  let buffered = '';

  output.setEncoding?.('utf8');
  output.on('data', (/** @type {string} */ chunk) => {
    buffered += chunk;
    let end = buffered.indexOf('\0');
    while (end !== -1) {
      const message = /** @type {CdpMessage} */ (JSON.parse(buffered.slice(0, end)));
      buffered = buffered.slice(end + 1);
      end = buffered.indexOf('\0');
      if (message.id !== undefined) {
        const call = pending.get(message.id);
        pending.delete(message.id);
        if (message.error) {
          call?.reject(new Error(`${call.method}: ${message.error.message}`));
        } else {
          call?.resolve(message.result ?? {});
        }
      } else if (message.method) {
        for (const listener of [...(listeners.get(message.method) ?? [])]) {
          listener(message.params ?? {}, message.sessionId);
        }
      }
    }
  });

  /** @type {CdpConnection['on']} */
  const on = (method, listener) => {
    const set = listeners.get(method) ?? new Set();
    set.add(listener);
    listeners.set(method, set);
    return () => set.delete(listener);
  };

  return {
    send: (method, params = {}, sessionId) =>
      new Promise((resolve, reject) => {
        nextId += 1;
        pending.set(nextId, {resolve, reject, method});
        input.write(`${JSON.stringify({id: nextId, method, params, ...(sessionId ? {sessionId} : {})})}\0`);
      }),
    on,
    once: (method, sessionId) =>
      new Promise((resolve) => {
        const off = on(method, (params, from) => {
          if (from === sessionId) {
            off();
            resolve(params);
          }
        });
      }),
    fail: (error) => {
      for (const call of pending.values()) {
        call.reject(error);
      }
      pending.clear();
    },
  };
};
