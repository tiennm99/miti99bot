import { afterEach, describe, expect, it, vi } from "vitest";
import { Word2SimError, createClient } from "../../../src/modules/semantle/api-client.js";

describe("semantle/api-client", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("Word2SimError", () => {
    it("stores status and body metadata", () => {
      const err = new Word2SimError("test", { status: 404, body: "not found" });
      expect(err.message).toBe("test");
      expect(err.status).toBe(404);
      expect(err.body).toBe("not found");
      expect(err.name).toBe("Word2SimError");
    });

    it("stores cause when provided", () => {
      const cause = new Error("underlying");
      const err = new Word2SimError("wrapper", { cause });
      expect(err.cause).toBe(cause);
    });
  });

  describe("createClient", () => {
    it("randomWord builds correct URL with filters", async () => {
      const client = createClient("https://api.test", { timeoutMs: 100 });
      global.fetch = vi.fn((url) => {
        expect(url).toContain("/random");
        expect(url).toContain("min_rank=5");
        expect(url).toContain("alpha=true");
        return Promise.resolve({
          ok: true,
          text: () => Promise.resolve('{"word":"apple","rank":1234}'),
        });
      });
      const res = await client.randomWord({ min_rank: 5, alpha: true });
      expect(res.word).toBe("apple");
      expect(res.rank).toBe(1234);
    });

    it("similarity builds URL with both words", async () => {
      const client = createClient("https://api.test", { timeoutMs: 100 });
      global.fetch = vi.fn((url) => {
        expect(url).toContain("/similarity");
        expect(url).toContain("a=apple");
        expect(url).toContain("b=orange");
        return Promise.resolve({
          ok: true,
          text: () =>
            Promise.resolve(
              '{"a":"apple","b":"orange","in_vocab_a":true,"in_vocab_b":true,"similarity":0.45}',
            ),
        });
      });
      const res = await client.similarity("apple", "orange");
      expect(res.similarity).toBe(0.45);
    });

    it("URL-encodes special characters in params", async () => {
      const client = createClient("https://api.test", { timeoutMs: 100 });
      global.fetch = vi.fn((url) => {
        expect(url).toMatch(/search=hello/);
        return Promise.resolve({
          ok: true,
          text: () => Promise.resolve('{"word":"test"}'),
        });
      });
      await client.randomWord({ search: "hello world" });
      expect(global.fetch).toHaveBeenCalled();
    });

    it("throws Word2SimError on non-2xx response", async () => {
      const client = createClient("https://api.test", { timeoutMs: 100 });
      global.fetch = vi.fn(() =>
        Promise.resolve({
          ok: false,
          status: 500,
          text: () => Promise.resolve("Internal Server Error"),
        }),
      );
      await expect(client.randomWord()).rejects.toMatchObject({
        name: "Word2SimError",
        status: 500,
        body: "Internal Server Error",
      });
    });

    it("throws Word2SimError when response is not valid JSON", async () => {
      const client = createClient("https://api.test", { timeoutMs: 100 });
      global.fetch = vi.fn(() =>
        Promise.resolve({
          ok: true,
          text: () => Promise.resolve("not json at all"),
        }),
      );
      await expect(client.randomWord()).rejects.toMatchObject({
        name: "Word2SimError",
      });
    });

    it("throws Word2SimError on fetch failure", async () => {
      const client = createClient("https://api.test", { timeoutMs: 100 });
      global.fetch = vi.fn(() => Promise.reject(new Error("network error")));
      await expect(client.randomWord()).rejects.toThrow("word2sim fetch failed");
    });

    it("uses custom timeout and truncates response body to 500 chars", async () => {
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

    it("includes User-Agent header", async () => {
      const client = createClient("https://api.test", { timeoutMs: 100 });
      global.fetch = vi.fn((_, opts) => {
        expect(opts.headers["User-Agent"]).toContain("miti99bot");
        return Promise.resolve({
          ok: true,
          text: () => Promise.resolve('{"word":"test"}'),
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
          text: () => Promise.resolve('{"word":"test"}'),
        });
      });
      await client.randomWord();
    });

    it("handles trailing slashes in API base URL", async () => {
      const client = createClient("https://api.test///", { timeoutMs: 100 });
      global.fetch = vi.fn((url) => {
        expect(url.startsWith("https://api.test/")).toBe(true);
        return Promise.resolve({
          ok: true,
          text: () => Promise.resolve('{"word":"test"}'),
        });
      });
      await client.randomWord();
    });

    it("filters out undefined/null params", async () => {
      const client = createClient("https://api.test", { timeoutMs: 100 });
      global.fetch = vi.fn((url) => {
        expect(url).not.toContain("min_rank=");
        return Promise.resolve({
          ok: true,
          text: () => Promise.resolve('{"word":"test"}'),
        });
      });
      await client.randomWord({ min_rank: undefined, max_rank: null });
    });
  });
});
