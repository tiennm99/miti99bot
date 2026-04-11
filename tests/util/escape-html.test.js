import { describe, expect, it } from "vitest";
import { escapeHtml } from "../../src/util/escape-html.js";

describe("escapeHtml", () => {
  it('escapes &, <, >, "', () => {
    expect(escapeHtml('&<>"')).toBe("&amp;&lt;&gt;&quot;");
  });

  it("leaves safe characters alone", () => {
    expect(escapeHtml("hello world 123 /cmd — émoji 🎉")).toBe("hello world 123 /cmd — émoji 🎉");
  });

  it("escapes in order so ampersand doesn't double-escape later entities", () => {
    // Input has literal & and <, output should have &amp; and &lt; — NOT &amp;lt;
    expect(escapeHtml("a & <b>")).toBe("a &amp; &lt;b&gt;");
  });

  it("coerces non-strings", () => {
    expect(escapeHtml(42)).toBe("42");
    expect(escapeHtml(null)).toBe("null");
  });
});
