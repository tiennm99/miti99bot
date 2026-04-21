import { describe, expect, it } from "vitest";
import { renderHelp } from "../../../src/modules/util/help-command.js";

const noop = async () => {};

/**
 * Build a synthetic registry directly — no loader involved. Lets us test
 * the pure renderer with exact inputs.
 */
function makeRegistry(modules) {
  const publicCommands = new Map();
  const protectedCommands = new Map();
  const privateCommands = new Map();
  const allCommands = new Map();

  for (const mod of modules) {
    for (const cmd of mod.commands) {
      const entry = { module: mod, cmd, visibility: cmd.visibility };
      allCommands.set(cmd.name, entry);
      if (cmd.visibility === "public") publicCommands.set(cmd.name, entry);
      else if (cmd.visibility === "protected") protectedCommands.set(cmd.name, entry);
      else privateCommands.set(cmd.name, entry);
    }
  }

  return {
    publicCommands,
    protectedCommands,
    privateCommands,
    allCommands,
    modules,
  };
}

const cmd = (name, visibility, description) => ({
  name,
  visibility,
  description,
  handler: noop,
});

describe("renderHelp", () => {
  it("groups commands by module in env.MODULES order", () => {
    const reg = makeRegistry([
      { name: "a", commands: [cmd("one", "public", "A-one")] },
      { name: "b", commands: [cmd("two", "public", "B-two")] },
    ]);
    const out = renderHelp(reg);
    const aIdx = out.indexOf("<b>a</b>");
    const bIdx = out.indexOf("<b>b</b>");
    expect(aIdx).toBeGreaterThanOrEqual(0);
    expect(bIdx).toBeGreaterThanOrEqual(0);
    expect(aIdx).toBeLessThan(bIdx);
  });

  it("appends (protected) suffix to protected commands", () => {
    const reg = makeRegistry([{ name: "a", commands: [cmd("admin", "protected", "Admin tool")] }]);
    const out = renderHelp(reg);
    expect(out).toContain("/admin — Admin tool (protected)");
  });

  it("hides modules whose only commands are private", () => {
    const reg = makeRegistry([
      { name: "a", commands: [cmd("visible", "public", "V")] },
      { name: "b", commands: [cmd("hidden", "private", "H")] },
    ]);
    const out = renderHelp(reg);
    expect(out).toContain("<b>a</b>");
    expect(out).not.toContain("<b>b</b>");
    expect(out).not.toContain("hidden");
  });

  it("NEVER leaks private command names into output", () => {
    const reg = makeRegistry([
      {
        name: "a",
        commands: [cmd("show", "public", "Public"), cmd("secret", "private", "Secret")],
      },
    ]);
    const out = renderHelp(reg);
    expect(out).toContain("/show");
    expect(out).not.toContain("/secret");
    expect(out).not.toContain("Secret");
  });

  it("HTML-escapes module name and description", () => {
    const reg = makeRegistry([
      {
        name: "a&b",
        commands: [cmd("foo", "public", "runs <script>")],
      },
    ]);
    const out = renderHelp(reg);
    expect(out).toContain("<b>a&amp;b</b>");
    expect(out).toContain("runs &lt;script&gt;");
    // The literal unescaped sequence must NOT appear in output.
    expect(out).not.toContain("<script>");
  });

  it("returns a placeholder when no commands are visible", () => {
    const reg = makeRegistry([{ name: "a", commands: [cmd("hidden", "private", "H")] }]);
    expect(renderHelp(reg)).toContain("no commands registered");
  });

  it("appends a support footer with the repo link", () => {
    const reg = makeRegistry([{ name: "a", commands: [cmd("one", "public", "A-one")] }]);
    const out = renderHelp(reg);
    expect(out).toContain("star");
    expect(out).toContain("https://github.com/tiennm99/miti99bot");
  });
});
