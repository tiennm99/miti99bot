import { afterEach, describe, expect, it, vi } from "vitest";
import {
  UpstreamError,
  Word2SimError,
  createClient,
} from "../../../src/modules/semantle/api-client.js";

/**
 * ConceptNet stubs — minimal shape the client cares about.
 */
function conceptResp(edgeCount = 5) {
  return {
    ok: true,
    text: () =>
      Promise.resolve(
        JSON.stringify({
          edges: Array.from({ length: edgeCount }, (_, i) => ({ id: `e${i}` })),
        }),
      ),
  };
}

function relatednessResp(value) {
  return {
    ok: true,
    text: () => Promise.resolve(JSON.stringify({ value })),
  };
}

describe("semantle/api-client", () => {
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

    it("is re-exported as Word2SimError alias for legacy callers", () => {
      expect(Word2SimError).toBe(UpstreamError);
    });
  });

  describe("createClient", () => {
    it("similarity runs concept + relatedness in parallel", async () => {
      const client = createClient("https://api.test", { timeoutMs: 100 });
      const calls = [];
      global.fetch = vi.fn((url) => {
        calls.push(String(url));
        if (url.includes("/relatedness")) return Promise.resolve(relatednessResp(0.45));
        return Promise.resolve(conceptResp(3));
      });
      const res = await client.similarity("apple", "orange");
      expect(res.similarity).toBe(0.45);
      expect(res.in_vocab_b).toBe(true);
      expect(res.canonical_b).toBe("orange");
      expect(global.fetch).toHaveBeenCalledTimes(2);
      expect(calls.some((u) => u.includes("/c/en/orange"))).toBe(true);
      expect(calls.some((u) => u.includes("node1=%2Fc%2Fen%2Fapple"))).toBe(true);
      expect(calls.some((u) => u.includes("node2=%2Fc%2Fen%2Forange"))).toBe(true);
    });

    it("similarity flags OOV when the concept endpoint returns no edges", async () => {
      const client = createClient("https://api.test", { timeoutMs: 100 });
      global.fetch = vi.fn((url) => {
        if (url.includes("/relatedness")) return Promise.resolve(relatednessResp(0.02));
        return Promise.resolve(conceptResp(0));
      });
      const res = await client.similarity("apple", "zzzfoo");
      expect(res.in_vocab_b).toBe(false);
      expect(res.similarity).toBe(null);
    });

    it("similarity returns null when relatedness payload lacks a numeric value", async () => {
      const client = createClient("https://api.test", { timeoutMs: 100 });
      global.fetch = vi.fn((url) => {
        if (url.includes("/relatedness")) {
          return Promise.resolve({ ok: true, text: () => Promise.resolve("{}") });
        }
        return Promise.resolve(conceptResp(5));
      });
      const res = await client.similarity("apple", "orange");
      expect(res.similarity).toBe(null);
    });

    it("similarity distinguishes 0 from null", async () => {
      const client = createClient("https://api.test", { timeoutMs: 100 });
      global.fetch = vi.fn((url) => {
        if (url.includes("/relatedness")) return Promise.resolve(relatednessResp(0));
        return Promise.resolve(conceptResp(5));
      });
      const res = await client.similarity("apple", "orange");
      expect(res.similarity).toBe(0);
      expect(res.in_vocab_b).toBe(true);
    });

    it("randomWord returns a verified pick when edges present", async () => {
      const client = createClient("https://api.test", { timeoutMs: 100 });
      global.fetch = vi.fn(() => Promise.resolve(conceptResp(5)));
      const res = await client.randomWord();
      expect(typeof res.word).toBe("string");
      expect(res.word.length).toBeGreaterThan(0);
      expect(res.verified).toBe(true);
    });

    it("randomWord falls back to unverified pick after max attempts", async () => {
      const client = createClient("https://api.test", { timeoutMs: 100 });
      // Every concept lookup returns zero edges → exhausts retries.
      global.fetch = vi.fn(() => Promise.resolve(conceptResp(0)));
      const res = await client.randomWord();
      expect(res.verified).toBe(false);
      expect(typeof res.word).toBe("string");
    });

    it("randomWord swallows transient fetch errors during verification", async () => {
      const client = createClient("https://api.test", { timeoutMs: 100 });
      let n = 0;
      global.fetch = vi.fn(() => {
        n += 1;
        // Error for the first few attempts, then succeed.
        if (n <= 2) return Promise.reject(new Error("transient"));
        return Promise.resolve(conceptResp(3));
      });
      const res = await client.randomWord();
      expect(res.verified).toBe(true);
    });

    it("concept throws UpstreamError on non-2xx response", async () => {
      const client = createClient("https://api.test", { timeoutMs: 100 });
      global.fetch = vi.fn(() =>
        Promise.resolve({
          ok: false,
          status: 500,
          text: () => Promise.resolve("Internal Server Error"),
        }),
      );
      await expect(client.concept("apple")).rejects.toMatchObject({
        name: "UpstreamError",
        status: 500,
        body: "Internal Server Error",
      });
    });

    it("concept throws UpstreamError when response is not valid JSON", async () => {
      const client = createClient("https://api.test", { timeoutMs: 100 });
      global.fetch = vi.fn(() =>
        Promise.resolve({ ok: true, text: () => Promise.resolve("not json") }),
      );
      await expect(client.concept("apple")).rejects.toMatchObject({ name: "UpstreamError" });
    });

    it("concept throws UpstreamError on fetch failure", async () => {
      const client = createClient("https://api.test", { timeoutMs: 100 });
      global.fetch = vi.fn(() => Promise.reject(new Error("network error")));
      await expect(client.concept("apple")).rejects.toThrow("conceptnet fetch failed");
    });

    it("truncates response body to 500 chars in UpstreamError", async () => {
      const client = createClient("https://api.test", { timeoutMs: 50 });
      const longBody = "x".repeat(600);
      global.fetch = vi.fn(() =>
        Promise.resolve({ ok: false, status: 400, text: () => Promise.resolve(longBody) }),
      );
      try {
        await client.concept("apple");
      } catch (err) {
        expect(err.body.length).toBe(500);
      }
    });

    it("sends User-Agent and Accept headers", async () => {
      const client = createClient("https://api.test", { timeoutMs: 100 });
      global.fetch = vi.fn((_, opts) => {
        expect(opts.headers["User-Agent"]).toContain("miti99bot");
        expect(opts.headers.Accept).toBe("application/json");
        return Promise.resolve(conceptResp(1));
      });
      await client.concept("apple");
    });

    it("strips trailing slashes from the API base URL", async () => {
      const client = createClient("https://api.test///", { timeoutMs: 100 });
      global.fetch = vi.fn((url) => {
        expect(url.startsWith("https://api.test/c/en/")).toBe(true);
        return Promise.resolve(conceptResp(1));
      });
      await client.concept("apple");
    });

    it("URL-encodes the term path segment", async () => {
      const client = createClient("https://api.test", { timeoutMs: 100 });
      global.fetch = vi.fn((url) => {
        expect(url).toContain("/c/en/hello%20world");
        return Promise.resolve(conceptResp(1));
      });
      await client.concept("hello world");
    });

    it("defaults to the public ConceptNet base URL when none provided", async () => {
      const client = createClient();
      global.fetch = vi.fn((url) => {
        expect(url.startsWith("https://api.conceptnet.io/")).toBe(true);
        return Promise.resolve(conceptResp(1));
      });
      await client.concept("apple");
    });
  });
});
