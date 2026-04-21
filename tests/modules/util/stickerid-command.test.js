import { describe, expect, it, vi } from "vitest";
import { stickerIdCommand } from "../../../src/modules/util/stickerid-command.js";

function makeCtx({ sticker }) {
  const reply = vi.fn(async () => {});
  return {
    reply,
    message: sticker ? { reply_to_message: { sticker } } : {},
  };
}

describe("/stickerid", () => {
  it("is a private command so it stays out of /help and the Telegram menu", () => {
    expect(stickerIdCommand.visibility).toBe("private");
  });

  it("tells the user how to use it when no sticker is replied to", async () => {
    const ctx = makeCtx({ sticker: null });
    await stickerIdCommand.handler(ctx);
    const [text] = ctx.reply.mock.calls[0];
    expect(text).toMatch(/reply to a sticker/i);
  });

  it("echoes file_id + file_unique_id when replying to a sticker", async () => {
    const ctx = makeCtx({
      sticker: {
        file_id: "CAACAgIAAxkBAAEFAKE_ID",
        file_unique_id: "AgADFAKEUNIQ",
        set_name: "HotCherry",
        emoji: "🔥",
      },
    });
    await stickerIdCommand.handler(ctx);
    const [text, opts] = ctx.reply.mock.calls[0];
    expect(opts).toEqual({ parse_mode: "HTML" });
    expect(text).toContain("CAACAgIAAxkBAAEFAKE_ID");
    expect(text).toContain("AgADFAKEUNIQ");
    expect(text).toContain("HotCherry");
    expect(text).toContain("🔥");
  });

  it("HTML-escapes sticker set name so a malicious set label can't inject tags", async () => {
    const ctx = makeCtx({
      sticker: {
        file_id: "id1",
        file_unique_id: "uid1",
        set_name: "<script>",
        emoji: null,
      },
    });
    await stickerIdCommand.handler(ctx);
    const [text] = ctx.reply.mock.calls[0];
    expect(text).toContain("&lt;script&gt;");
    expect(text).not.toContain("<script>");
  });
});
