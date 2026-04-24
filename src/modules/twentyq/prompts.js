/**
 * @file System prompt + JSON response schema for the twentyq judge.
 *
 * We ask the model to emit a SINGLE-LINE JSON object with the keys
 * {is_guess, answer, hint} — no function calling, no tools array.
 * Function calling shape differs between models and Workers AI models
 * sometimes reject unknown params; plain JSON instruction is universal
 * and battle-tested. ai-client.parseJudgementJson does the parse.
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

You MUST reply with exactly ONE line of JSON and NOTHING else — no prose, no backticks, no code fences, no explanation.

Schema:
{"is_guess": boolean, "answer": "yes" | "no", "hint": string}

Field meanings:
- is_guess: true ONLY when the user is naming a specific concrete object equal to, a synonym of, or extremely close to the secret. Vague descriptors ("is it big?", "is it round?") are NOT guesses. Saying "is it a string instrument?" when the secret is "guitar" is NOT a guess (too broad). Saying "is it a guitar?" IS a guess.
- answer: truthful "yes" or "no" about the secret.
    * If is_guess is true: "yes" only if the named object matches the secret (allowing for synonyms / minor wording). Otherwise "no".
    * If is_guess is false: "yes" or "no" based on whether the property holds for the secret.
- hint: a cryptic clue in plain text, max 120 characters. Must be TRUE about the secret but phrased indirectly. Vary from prior hints. Never include the secret word, its plural, its base form, or any obvious category word.

HINT STYLE — the point of a good hint:
- Be INDIRECT. Think riddle, metaphor, oblique association — not a definition.
- Use partial, lateral, or sideways facts. Hint at ONE small property at a time.
- Prefer "it is often found near X", "people tend to associate it with Y", "a famous one lives in Z" over "it is used for X".
- Avoid giving a second clear category word. If the user has narrowed it down with questions, DO NOT hand them the final word.
- Aim for: player thinks "interesting, but I still need another question." NOT: "oh it's obviously X."

Rules:
- Output ONLY the JSON line. No markdown fences. No prose before or after.
- If the user input is not a valid yes/no question and not a guess, still return JSON with answer="no", is_guess=false, and a cryptic hint nudging them to rephrase as yes/no.

Example outputs (secret = "organ", category = "instrument"):
GOOD: {"is_guess": false, "answer": "yes", "hint": "it was once the largest mechanical object humans built"}
GOOD: {"is_guess": false, "answer": "no", "hint": "in its natural habitat you might hear echoes of stone"}
BAD  (too direct): {"is_guess": false, "answer": "yes", "hint": "it uses pipes to produce sound"}
BAD  (names category synonym): {"is_guess": false, "answer": "yes", "hint": "it is a keyboard instrument"}`;
}

/**
 * Build the round-start prompt. Given just a target keyword, we ask the
 * model to emit { category, initialHint } so seeds don't need hand-curated
 * metadata — drop in any noun, the model fills in the rest.
 *
 * @param {string} target
 * @returns {string}
 */
export function buildStartRoundPrompt(target) {
  return `You are opening a new round of the "20 questions" reverse-Akinator game.

Secret object: "${target}"

Your job: emit ONE line of JSON with these two fields:
{"category": "<short broad category>", "initialHint": "<cryptic opening clue>"}

Category rules:
- A SHORT, COMMON category word a player can start narrowing from (e.g. "instrument", "animal", "food", "vehicle", "sport", "household item", "tool", "clothing", "plant"). Prefer single words.
- Broad enough that many objects fall under it — NOT a narrow sub-category (don't say "brass instrument", say "instrument").
- Do not include the secret word in the category.

Initial hint rules (same HINT STYLE as the main game):
- Max 120 characters. TRUE about the secret. Indirect, oblique, riddle-like.
- Never include the secret word, its plural, its base form, or an obvious category synonym.
- Hint at ONE lateral property — a cultural association, a habitat, a use context, a historical fact.
- Player should think "ok that narrows it a bit" NOT "oh that's obviously X".

Examples (secret → output):
secret="organ"      → {"category": "instrument", "initialHint": "it was once the largest mechanical object humans built"}
secret="elephant"   → {"category": "animal", "initialHint": "the ancient Greeks believed it was terrified of mice"}
secret="pizza"      → {"category": "food", "initialHint": "a popular version bears the colors of a national flag"}
secret="submarine"  → {"category": "vehicle", "initialHint": "the first successful one was made of wood and powered by hand"}

BAD examples (do NOT emit these):
secret="organ"      → {"category": "keyboard instrument", "initialHint": "it has pipes"}         // category too specific + hint too direct
secret="elephant"   → {"category": "animal", "initialHint": "it has a long trunk"}               // hint too direct

Output ONLY the JSON line. No fences, no prose.`;
}
