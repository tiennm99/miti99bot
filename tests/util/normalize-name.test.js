import { describe, expect, it } from "vitest";
import { normalize } from "../../src/util/normalize-name.js";

describe("normalize", () => {
  it("lowercases and strips non-alphanumerics", () => {
    expect(normalize("Ahri")).toBe("ahri");
    expect(normalize("Miss Fortune")).toBe("missfortune");
    expect(normalize("Kai'Sa")).toBe("kaisa");
    expect(normalize("Dr. Mundo")).toBe("drmundo");
  });

  it("handles empty and null input", () => {
    expect(normalize("")).toBe("");
    expect(normalize(null)).toBe("");
    expect(normalize(undefined)).toBe("");
  });

  it("coerces non-strings", () => {
    expect(normalize(42)).toBe("42");
  });
});
