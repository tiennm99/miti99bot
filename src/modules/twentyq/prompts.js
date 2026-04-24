/**
 * @file System prompt + function-calling schema for the twentyq judge.
 *
 * The model receives the secret target + history each turn and emits a single
 * structured `submit_answer({ is_guess, answer, hint })` call. We never let
 * the model reply in free prose — function calling guarantees parseable shape.
 */

/** @typedef {import("./state.js").TwentyqGameState} TwentyqGameState */

const HISTORY_WINDOW = 5;

/**
 * Build the system prompt. Includes the secret + last N turns so the model
 * stays consistent with its prior answers and varies the hint.
 *
 * @param {TwentyqGameState} state
 * @returns {string}
 */
export function buildSystemPrompt(state) {
  const recent = state.turns.slice(-HISTORY_WINDOW);
  const historyText = recent.length
    ? recent.map((t, i) => `${i + 1}. Q: ${t.text}\n   A: ${t.answer}. Hint: ${t.hint}`).join("\n")
    : "(no questions yet)";

  return `You are the judge for a "20 questions" reverse-Akinator game.
The user is trying to guess a secret object. You must answer truthfully based on what the secret actually is.

Secret object: "${state.target}"
Category: ${state.category}
Initial hint already given: ${state.initialHint}

Question history so far:
${historyText}

The user will send a single message — either a yes/no question (e.g. "is it big?", "does it have wheels?") or a final guess of a specific noun (e.g. "is it an organ?", "is it a piano?").

You MUST call the submit_answer function with:
  - is_guess (boolean): true ONLY when the user is naming a specific concrete object that is the same as, a synonym of, or extremely close to the secret. Vague descriptors like "is it big?", "is it round?" are NOT guesses. Saying "is it a string instrument?" when the secret is "guitar" is NOT a guess (too broad). Saying "is it a guitar?" IS a guess.
  - answer ("yes" or "no"): truthful answer about the secret.
      * If is_guess is true: "yes" only if the named object matches the secret (allowing for synonyms / minor wording). Otherwise "no".
      * If is_guess is false: "yes" or "no" based on whether the property holds for the secret.
  - hint (string, max 120 chars): a NEW useful clue. Vary it from prior hints. Never include the secret word, its plural, or its base form. Never reveal the answer in the hint.

Rules:
- ALWAYS call submit_answer exactly once. Never reply in free text.
- Stay consistent with prior answers above.
- If the user input is not a yes/no question and not a guess (e.g. open-ended), still call submit_answer with answer="no", is_guess=false, and a hint asking them to rephrase as a yes/no question.`;
}

/**
 * Function-calling schema. Traditional Workers-AI format
 * (https://developers.cloudflare.com/workers-ai/features/function-calling/traditional/).
 */
export const ANSWER_FUNCTION_SCHEMA = {
  name: "submit_answer",
  description: "Submit the truthful yes/no answer to the user's question along with a fresh hint.",
  parameters: {
    type: "object",
    properties: {
      is_guess: {
        type: "boolean",
        description:
          "True ONLY if the user named a specific concrete object that matches or is a synonym of the secret.",
      },
      answer: {
        type: "string",
        enum: ["yes", "no"],
        description: "Truthful yes/no answer about the secret.",
      },
      hint: {
        type: "string",
        description: "A new useful clue (max 120 chars) that does not contain the secret word.",
      },
    },
    required: ["is_guess", "answer", "hint"],
  },
};
