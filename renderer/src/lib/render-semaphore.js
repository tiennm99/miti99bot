/**
 * @param {number} max
 */
export const createRenderSemaphore = (max) => {
  let active = 0;

  return {
    /**
     * @returns {boolean}
     */
    tryAcquire() {
      if (active >= max) {
        return false;
      }

      active += 1;
      return true;
    },

    release() {
      active = Math.max(0, active - 1);
    },

    /**
     * @returns {{active: number, max: number}}
     */
    state() {
      return {active, max};
    },
  };
};
