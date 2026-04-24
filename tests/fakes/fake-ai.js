/**
 * @file fake-ai — minimal stub for the Workers AI binding (env.AI).
 *
 * Real shape: `{ run(modelId, body) -> Promise<any> }`. Tests configure the
 * mock via `mockJudgement(ai, { is_guess, answer, hint })` to return the
 * structured tool-call response that ai-client.parseToolCall consumes.
 */

import { vi } from "vitest";

export function makeFakeAi() {
  return { run: vi.fn() };
}

/**
 * Configure the next ai.run call to return a Cloudflare-traditional
 * tool_calls response with the given submit_answer arguments.
 */
export function mockJudgement(ai, { is_guess = false, answer = "no", hint = "default hint" } = {}) {
  ai.run.mockResolvedValueOnce({
    tool_calls: [
      {
        name: "submit_answer",
        arguments: { is_guess, answer, hint },
      },
    ],
  });
}

/** Configure the next call to throw (simulate Workers AI outage). */
export function mockFailure(ai, err = new Error("AI down")) {
  ai.run.mockRejectedValueOnce(err);
}
