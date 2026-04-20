import { describe, expect, it } from "vitest";
import { CLASSIC_ATTRIBUTES, compareChampions } from "../../../src/modules/loldle/compare.js";

const aatrox = {
  id: "Aatrox",
  gender: "male",
  genre: "Fighter",
  attackType: "close",
  resource: "Blood Well",
  region: "runeterra",
  lane: "top",
  releaseDate: 2013,
};

const ahri = {
  id: "Ahri",
  gender: "female",
  genre: "Mage,Assassin",
  attackType: "range",
  resource: "Mana",
  region: "ionia",
  lane: "mid",
  releaseDate: 2011,
};

const akali = {
  id: "Akali",
  gender: "female",
  genre: "Assassin",
  attackType: "close",
  resource: "Energy",
  region: "ionia",
  lane: "mid,top",
  releaseDate: 2010,
};

function byKey(results, key) {
  return results.find((r) => r.key === key);
}

describe("compareChampions", () => {
  it("all correct when guess === target", () => {
    const r = compareChampions(aatrox, aatrox);
    for (const row of r) expect(row.result).toBe("correct");
  });

  it("exact mismatch is wrong", () => {
    const r = compareChampions(aatrox, ahri);
    expect(byKey(r, "gender").result).toBe("wrong");
    expect(byKey(r, "attackType").result).toBe("wrong");
    expect(byKey(r, "resource").result).toBe("wrong");
    expect(byKey(r, "region").result).toBe("wrong");
  });

  it("multi-value partial overlap is partial", () => {
    const r = compareChampions(akali, ahri);
    expect(byKey(r, "genre").result).toBe("partial");
    expect(byKey(r, "lane").result).toBe("partial");
  });

  it("multi-value identical sets are correct even if order/case differ", () => {
    const r = compareChampions(
      { ...akali, genre: "assassin" },
      { ...akali, genre: "Assassin" },
    );
    expect(byKey(r, "genre").result).toBe("correct");
  });

  it("year direction hints up when guess < target", () => {
    const r = compareChampions(akali, aatrox); // 2010 vs 2013
    const y = byKey(r, "releaseDate");
    expect(y.result).toBe("wrong");
    expect(y.direction).toBe("up");
  });

  it("year direction hints down when guess > target", () => {
    const r = compareChampions(aatrox, akali); // 2013 vs 2010
    const y = byKey(r, "releaseDate");
    expect(y.result).toBe("wrong");
    expect(y.direction).toBe("down");
  });

  it("returns 7 attributes in declared order", () => {
    const r = compareChampions(aatrox, ahri);
    expect(r.map((x) => x.key)).toEqual(CLASSIC_ATTRIBUTES.map((a) => a.key));
  });
});
