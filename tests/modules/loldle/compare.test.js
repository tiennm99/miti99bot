import { describe, expect, it } from "vitest";
import { CLASSIC_ATTRIBUTES, compareChampions } from "../../../src/modules/loldle/compare.js";

const aatrox = {
  championName: "Aatrox",
  gender: "Male",
  species: ["Darkin"],
  range_type: ["Melee"],
  resource: "Manaless",
  regions: ["Runeterra", "Shurima"],
  positions: ["Top"],
  release_date: "2013-06-13",
};

const ahri = {
  championName: "Ahri",
  gender: "Female",
  species: ["Vastayan"],
  range_type: ["Ranged"],
  resource: "Mana",
  regions: ["Ionia"],
  positions: ["Middle"],
  release_date: "2011-12-14",
};

const akali = {
  championName: "Akali",
  gender: "Female",
  species: ["Human"],
  range_type: ["Melee"],
  resource: "Energy",
  regions: ["Ionia"],
  positions: ["Middle", "Top"],
  release_date: "2010-05-11",
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
    expect(byKey(r, "resource").result).toBe("wrong");
  });

  it("multi-value partial overlap is partial", () => {
    const r = compareChampions(akali, { ...ahri, positions: ["Middle"] });
    expect(byKey(r, "positions").result).toBe("partial");
    const r2 = compareChampions(aatrox, { ...ahri, regions: ["Runeterra", "Ionia"] });
    expect(byKey(r2, "regions").result).toBe("partial");
  });

  it("multi-value identical sets are correct even if order/case differ", () => {
    const r = compareChampions(
      { ...akali, species: ["Human", "Ninja"] },
      { ...akali, species: ["ninja", "HUMAN"] },
    );
    expect(byKey(r, "species").result).toBe("correct");
  });

  it("year direction hints up when guess < target", () => {
    const r = compareChampions(akali, aatrox); // 2010 vs 2013
    const y = byKey(r, "release_date");
    expect(y.result).toBe("wrong");
    expect(y.direction).toBe("up");
  });

  it("year direction hints down when guess > target", () => {
    const r = compareChampions(aatrox, akali); // 2013 vs 2010
    const y = byKey(r, "release_date");
    expect(y.result).toBe("wrong");
    expect(y.direction).toBe("down");
  });

  it("returns 7 attributes in declared order", () => {
    const r = compareChampions(aatrox, ahri);
    expect(r.map((x) => x.key)).toEqual(CLASSIC_ATTRIBUTES.map((a) => a.key));
  });
});
