import { afterEach, describe, expect, it, vi } from "vitest";
import { UpstreamError, createClient } from "../../../src/modules/doantu/api-client.js";

describe("doantu/api-client", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

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
    it("randomWord builds correct URL with filters", async () => {
      const client = createClient("https://api.test", { timeoutMs: 100 });
      global.fetch = vi.fn((url) => {
        expect(url).toContain("/random");
        expect(url).toContain("min_len=2");
        return Promise.resolve({
          ok: true,
          text: () => Promise.resolve('{"word":"chó"}'),
        });
      });
      const res = await client.randomWord({ min_len: 2 });
      expect(res.word).toBe("chó");
    });

    it("similarity builds URL with both words", async () => {
      const client = createClient("https://api.test", { timeoutMs: 100 });
      global.fetch = vi.fn((url) => {
        expect(url).toContain("/similarity");
        expect(url).toMatch(/a=ch%C3%B3/);
        expect(url).toMatch(/b=m%C3%A8o/);
        return Promise.resolve({
          ok: true,
          text: () =>
            Promise.resolve(
              '{"a":"chó","b":"mèo","in_vocab_a":true,"in_vocab_b":true,"canonical_a":"chó","canonical_b":"mèo","similarity":0.42}',
            ),
        });
      });
      const res = await client.similarity("chó", "mèo");
      expect(res.similarity).toBe(0.42);
      expect(res.canonical_b).toBe("mèo");
    });

    it("URL-encodes multi-syllable Vietnamese guesses", async () => {
      const client = createClient("https://api.test", { timeoutMs: 100 });
      global.fetch = vi.fn((url) => {
        expect(url).toMatch(/b=con\+ch%C3%B3|b=con%20ch%C3%B3/);
        return Promise.resolve({
          ok: true,
          text: () =>
            Promise.resolve(
              '{"a":"mèo","b":"con chó","in_vocab_a":true,"in_vocab_b":true,"similarity":0.3}',
            ),
        });
      });
      await client.similarity("mèo", "con chó");
      expect(global.fetch).toHaveBeenCalled();
    });

    it("throws UpstreamError on non-2xx response", async () => {
      const client = createClient("https://api.test", { timeoutMs: 100 });
      global.fetch = vi.fn(() =>
        Promise.resolve({
          ok: false,
          status: 500,
          text: () => Promise.resolve("Internal Server Error"),
        }),
      );
      await expect(client.randomWord()).rejects.toMatchObject({
        name: "UpstreamError",
        status: 500,
        body: "Internal Server Error",
      });
    });

    it("throws UpstreamError when response is not valid JSON", async () => {
      const client = createClient("https://api.test", { timeoutMs: 100 });
      global.fetch = vi.fn(() =>
        Promise.resolve({
          ok: true,
          text: () => Promise.resolve("not json at all"),
        }),
      );
      await expect(client.randomWord()).rejects.toMatchObject({
        name: "UpstreamError",
      });
    });

    it("throws UpstreamError on fetch failure", async () => {
      const client = createClient("https://api.test", { timeoutMs: 100 });
      global.fetch = vi.fn(() => Promise.reject(new Error("network error")));
      await expect(client.randomWord()).rejects.toThrow("phow2sim fetch failed");
    });

    it("truncates response body to 500 chars on non-OK", async () => {
      const client = createClient("https://api.test", { timeoutMs: 50 });
      const longBody = "x".repeat(600);
      global.fetch = vi.fn(() =>
        Promise.resolve({
          ok: false,
          status: 400,
          text: () => Promise.resolve(longBody),
        }),
      );
      try {
        await client.randomWord();
      } catch (err) {
        expect(err.body.length).toBe(500);
      }
    });

    it("includes User-Agent header identifying doantu", async () => {
      const client = createClient("https://api.test", { timeoutMs: 100 });
      global.fetch = vi.fn((_, opts) => {
        expect(opts.headers["User-Agent"]).toContain("miti99bot");
        expect(opts.headers["User-Agent"]).toContain("doantu");
        return Promise.resolve({
          ok: true,
          text: () => Promise.resolve('{"word":"chó"}'),
        });
      });
      await client.randomWord();
    });

    it("includes Accept header", async () => {
      const client = createClient("https://api.test", { timeoutMs: 100 });
      global.fetch = vi.fn((_, opts) => {
        expect(opts.headers.Accept).toBe("application/json");
        return Promise.resolve({
          ok: true,
          text: () => Promise.resolve('{"word":"chó"}'),
        });
      });
      await client.randomWord();
    });

    it("handles trailing slashes in API base URL", async () => {
      const client = createClient("https://api.test///", { timeoutMs: 100 });
      global.fetch = vi.fn((url) => {
        expect(url.startsWith("https://api.test/")).toBe(true);
        expect(url.startsWith("https://api.test////")).toBe(false);
        return Promise.resolve({
          ok: true,
          text: () => Promise.resolve('{"word":"chó"}'),
        });
      });
      await client.randomWord();
    });

    it("filters out undefined/null params", async () => {
      const client = createClient("https://api.test", { timeoutMs: 100 });
      global.fetch = vi.fn((url) => {
        expect(url).not.toContain("min_len=");
        expect(url).not.toContain("max_len=");
        return Promise.resolve({
          ok: true,
          text: () => Promise.resolve('{"word":"chó"}'),
        });
      });
      await client.randomWord({ min_len: undefined, max_len: null });
    });
  });
});
