import { describe, expect, it } from "vitest";
import { renderBoard } from "../../../src/modules/loldle-emoji/render.js";

describe("renderBoard (emoji)", () => {
  it("shows an empty-state hint when no guesses yet", () => {
    const out = renderBoard("🦊 🏯 🔮", [], 5);
    expect(out).toContain("🎭 🦊 🏯 🔮");
    expect(out).toContain("No guesses yet.");
  });

  it("lists guesses with wrong markers and guess counter", () => {
    const out = renderBoard("🦊 🏯 🔮", ["Akali", "Aatrox"], 5);
    expect(out).toContain("Guesses (2/5):");
    expect(out).toContain("• Akali  ❌");
    expect(out).toContain("• Aatrox  ❌");
  });

  it("HTML-escapes champion names", () => {
    const out = renderBoard("🦊", ["<script>"], 5);
    expect(out).toContain("&lt;script&gt;");
    expect(out).not.toContain("<script>");
  });
});
