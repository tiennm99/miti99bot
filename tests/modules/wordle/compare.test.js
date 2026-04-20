import { describe, expect, it } from "vitest";
import { compareWords } from "../../../src/modules/wordle/compare.js";

function letters(results) {
  return results.map((r) => r.result).join(",");
}

describe("compareWords", () => {
  it("all correct when guess === target", () => {
    const r = compareWords("crane", "crane");
    expect(letters(r)).toBe("correct,correct,correct,correct,correct");
  });

  it("all wrong when no letters overlap", () => {
    const r = compareWords("abcde", "fghij");
    expect(letters(r)).toBe("wrong,wrong,wrong,wrong,wrong");
  });

  it("marks correct position over partial position", () => {
    // guess "slate" vs target "shale" → s correct, l partial, a correct,
    // t wrong, e correct
    const r = compareWords("slate", "shale");
    expect(letters(r)).toBe("correct,partial,correct,wrong,correct");
  });

  it("duplicate letters: excess guess letters mark wrong, not partial", () => {
    // target "abbey", guess "babes"
    // positions: b@0≠a (pool gets a,b,b,e,y minus correct slots)
    // correct pass: b@2=b → consume b; e@3=e → consume e. pool = [a,b,y]
    // partial pass:
    //   b@0 → in pool (b), consume → partial
    //   a@1 → in pool (a), consume → partial
    //   s@4 → not in pool → wrong
    const r = compareWords("babes", "abbey");
    expect(letters(r)).toBe("partial,partial,correct,correct,wrong");
  });

  it("duplicate letter in guess when target has only one: only first is partial", () => {
    // target "abide", guess "aahed"
    // correct: a@0=a (consume a), no other positional match
    // pool = [b,i,d,e]
    // partial pass: a@1 → not in pool → wrong; h@2 → wrong; e@3 → partial (consume e); d@4 → partial (consume d)
    const r = compareWords("aahed", "abide");
    expect(letters(r)).toBe("correct,wrong,wrong,partial,partial");
  });

  it("duplicate in guess, duplicate in target: both can mark", () => {
    // target "lever", guess "ebbed"
    // correct: e@2=v? no. Actually lever[2]='v'. ebbed = e,b,b,e,d
    //   e@0 vs l → no; b@1 vs e → no; b@2 vs v → no; e@3 vs e → correct (consume e);
    //   d@4 vs r → no. pool = [l,e,v,r]
    // partial pass:
    //   e@0 → partial (consume remaining e); b@1 → wrong; b@2 → wrong; d@4 → wrong
    const r = compareWords("ebbed", "lever");
    expect(letters(r)).toBe("partial,wrong,wrong,correct,wrong");
  });

  it("returns letters alongside results", () => {
    const r = compareWords("crane", "cloud");
    expect(r.map((x) => x.letter)).toEqual(["c", "r", "a", "n", "e"]);
  });
});
