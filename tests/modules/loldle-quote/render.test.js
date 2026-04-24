import { describe, expect, it } from "vitest";
import { renderBoard } from "../../../src/modules/loldle-quote/render.js";

describe("renderBoard (quote)", () => {
  it("wraps quote in italic with emoji prefix", () => {
    const out = renderBoard("the Demacian", [], 6);
    expect(out).toContain("🎭 <i>the Demacian</i>");
    expect(out).toContain("No guesses yet.");
  });

  it("HTML-escapes quote text before italic wrap", () => {
    const out = renderBoard('She said "<3"', [], 6);
    expect(out).toContain("<i>She said &quot;&lt;3&quot;</i>");
    expect(out).not.toContain('She said "<3"</i>');
  });

  it("lists wrong guesses with counter", () => {
    const out = renderBoard("the X", ["Ahri"], 6);
    expect(out).toContain("Guesses (1/6):");
    expect(out).toContain("• Ahri  ❌");
  });
});
