/**
 * @file fake-ai — minimal stub for the Workers AI binding (env.AI).
 *
 * Real shape: `{ run(modelId, body) -> Promise<any> }`. Tests configure the
 * mock via `mockJudgement(ai, { is_guess, answer, hint })` to return a
 * Workers-AI-traditional { response: "<json-line>" } payload that
 * ai-client.extractText + parseJudgementJson consume.
 */

import { vi } from "vitest";

export function makeFakeAi() {
  return { run: vi.fn() };
}

/**
 * Configure the next ai.run call to return a response whose `.response`
 * string contains the canonical one-line JSON the judge expects.
 */
export function mockJudgement(ai, { is_guess = false, answer = "no", hint = "default hint" } = {}) {
  ai.run.mockResolvedValueOnce({
    response: JSON.stringify({ is_guess, answer, hint }),
  });
}

/**
 * Configure the next ai.run call to return a round-start response
 * with { category, initialHint } JSON.
 */
export function mockRoundStart(ai, { category = "object", initialHint = "cryptic clue" } = {}) {
  ai.run.mockResolvedValueOnce({
    response: JSON.stringify({ category, initialHint }),
  });
}

/** Configure the next call to throw (simulate Workers AI outage). */
export function mockFailure(ai, err = new Error("AI down")) {
  ai.run.mockRejectedValueOnce(err);
}
