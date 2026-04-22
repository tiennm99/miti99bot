import { describe, expect, it, vi } from "vitest";
import { UpstreamError, createClient } from "../../../src/modules/doantu/api-client.js";

/**
 * Build a deterministic 1024-dim vector from a seed so cosine scores are
 * reproducible in tests without hardcoding floats. bge-m3 produces 1024-dim
 * vectors; tests use the same width for realism.
 */
function fakeVector(seed, dim = 1024) {
  const out = new Array(dim);
  for (let i = 0; i < dim; i++) out[i] = Math.sin(seed * (i + 1));
  return out;
}

/**
 * Minimal Workers AI binding fake. `impl(model, input)` returns the payload
 * `env.AI.run()` would normally resolve to.
 */
function fakeAi(impl) {
  return { run: vi.fn(impl) };
}

describe("doantu/api-client", () => {
  describe("UpstreamError", () => {
    it("stores status and body metadata", () => {
      const err = new UpstreamError("test", { status: 404, body: "not found" });
      expect(err.message).toBe("test");
      expect(err.status).toBe(404);
      expect(err.body).toBe("not found");
      expect(err.name).toBe("UpstreamError");
    });

    it("stores cause when provided", () => {
      const cause = new Error("underlying");
      const err = new UpstreamError("wrapper", { cause });
      expect(err.cause).toBe(cause);
    });
  });

  describe("createClient", () => {
    it("throws without a valid AI binding", () => {
      expect(() => createClient(null)).toThrow(TypeError);
      expect(() => createClient({})).toThrow(TypeError);
      expect(() => createClient({ run: "not a function" })).toThrow(TypeError);
    });

    it("similarity batches target + guess in a single run() call with bge-m3", async () => {
      const ai = fakeAi(async (_model, { text }) => ({
        shape: [text.length, 1024],
        data: text.map((_, i) => fakeVector(i + 1)),
      }));
      const client = createClient(ai);
      await client.similarity("chó", "mèo");
      expect(ai.run).toHaveBeenCalledTimes(1);
      const [model, input] = ai.run.mock.calls[0];
      expect(model).toBe("@cf/baai/bge-m3");
      expect(input).toEqual({ text: ["chó", "mèo"] });
    });

    it("similarity returns cosine score for an in-vocab Vietnamese guess", async () => {
      const ai = fakeAi(async (_model, { text }) => ({
        data: text.map((_, i) => fakeVector(i + 1)),
      }));
      const client = createClient(ai);
      const res = await client.similarity("chó", "mèo");
      expect(res.in_vocab_a).toBe(true);
      expect(res.in_vocab_b).toBe(true);
      expect(res.canonical_a).toBe("chó");
      expect(res.canonical_b).toBe("mèo");
      expect(typeof res.similarity).toBe("number");
      expect(res.similarity).toBeGreaterThan(-1);
      expect(res.similarity).toBeLessThanOrEqual(1);
    });

    it('similarity accepts multi-syllable Vietnamese words in vocab ("a dua")', async () => {
      const ai = fakeAi(async () => ({ data: [fakeVector(1), fakeVector(2)] }));
      const client = createClient(ai);
      const res = await client.similarity("chó", "a dua");
      expect(res.in_vocab_b).toBe(true);
      expect(res.similarity).not.toBeNull();
    });

    it("similarity returns 1 for identical vectors", async () => {
      const vec = fakeVector(7);
      const ai = fakeAi(async () => ({ data: [vec, vec] }));
      const client = createClient(ai);
      const res = await client.similarity("chó", "mèo");
      expect(res.similarity).toBeCloseTo(1, 10);
    });

    it("similarity skips AI call for OOV guess and flags in_vocab_b:false", async () => {
      const ai = fakeAi(async () => ({ data: [fakeVector(1), fakeVector(2)] }));
      const client = createClient(ai);
      const res = await client.similarity("chó", "zzzkhôngcótrongtừđiển");
      expect(res.in_vocab_b).toBe(false);
      expect(res.similarity).toBe(null);
      expect(ai.run).not.toHaveBeenCalled();
    });

    it("similarity wraps AI.run rejection as UpstreamError", async () => {
      const ai = fakeAi(async () => {
        throw new Error("boom");
      });
      const client = createClient(ai);
      await expect(client.similarity("chó", "mèo")).rejects.toMatchObject({
        name: "UpstreamError",
      });
    });

    it("similarity throws UpstreamError on malformed payload", async () => {
      const ai = fakeAi(async () => ({ data: [fakeVector(1)] }));
      const client = createClient(ai);
      await expect(client.similarity("chó", "mèo")).rejects.toMatchObject({
        name: "UpstreamError",
      });
    });

    it("similarity returns null score when a vector norm is zero", async () => {
      const zero = new Array(1024).fill(0);
      const ai = fakeAi(async () => ({ data: [zero, fakeVector(1)] }));
      const client = createClient(ai);
      const res = await client.similarity("chó", "mèo");
      expect(res.in_vocab_b).toBe(true);
      expect(res.similarity).toBe(null);
    });

    it("randomWord returns a verified pick from the local pool", async () => {
      const ai = fakeAi(async () => ({ data: [] }));
      const client = createClient(ai);
      const res = await client.randomWord();
      expect(typeof res.word).toBe("string");
      expect(res.word.length).toBeGreaterThan(0);
      expect(res.verified).toBe(true);
      expect(ai.run).not.toHaveBeenCalled();
    });

    it("supports model override via options", async () => {
      const ai = fakeAi(async () => ({ data: [fakeVector(1), fakeVector(2)] }));
      const client = createClient(ai, { model: "@cf/baai/bge-large-en-v1.5" });
      await client.similarity("chó", "mèo");
      expect(ai.run.mock.calls[0][0]).toBe("@cf/baai/bge-large-en-v1.5");
    });
  });
});
