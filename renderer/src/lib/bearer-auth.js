/**
 * @param {import('fastify').FastifyRequest} request
 * @param {string | undefined} apiToken
 * @returns {boolean} true when no token is configured or the request carries it.
 */
export const isAuthorized = (request, apiToken) =>
  !apiToken || request.headers.authorization === `Bearer ${apiToken}`;
