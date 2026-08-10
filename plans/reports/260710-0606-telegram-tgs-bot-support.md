---
type: researcher
date: 2026-07-10
conducted_at: 2026-07-10T06:06:00Z
---

# Research Report: Telegram Bot `.tgs` Support

## Navigation

- [Summary](#summary)
- [Methodology](#methodology)
- [Findings](#findings)
- [Implementation Recommendation](#implementation-recommendation)
- [References](#references)
- [Next Steps](#next-steps)
- [Unresolved Questions](#unresolved-questions)

## Summary

Yes. Telegram Bot API `sendSticker` accepts a new animated `.TGS` sticker as a
multipart upload. It also accepts an existing Telegram `file_id`, which is the
recommended path after first upload. A `.TGS` URL is not accepted for an
animated sticker.

This repo already has everything needed. `github.com/go-telegram/bot` v1.20.0
provides `SendSticker` and `models.InputFileUpload`; no dependency upgrade or
new package required.

## Methodology

- Sources consulted: Telegram Bot API, Telegram sticker specification, pinned
  Go module source, current repo implementation
- Research date: 2026-07-10
- Terms: `sendSticker`, `.TGS`, `InputFile`, multipart upload, animated sticker
- Boundary: send an existing `.TGS`; sticker-set creation excluded

## Findings

### API Support

Use `sendSticker`, with the `sticker` parameter as an `InputFile` multipart
upload. Telegram explicitly lists `.WEBP`, `.TGS`, and `.WEBM` as supported
uploads. For `.TGS`, do not pass an HTTP URL; upload bytes or reuse a `file_id`.
The file does not need to be added to a sticker set first.

`emoji` is optional and applies to a newly uploaded sticker. Telegram returns a
`Message`; its sticker contains the reusable `file_id`.

### `.TGS` Requirements

Telegram's animated-sticker requirements:

- 512 x 512 canvas
- maximum 3 seconds
- looped animation
- maximum 64 KB after rendering
- 60 FPS
- supported Bodymovin-TG/Lottie features only

An arbitrary Lottie JSON renamed to `.tgs` is not sufficient. Telegram `.TGS`
is its constrained, gzip-compressed Lottie format.

### Repo Compatibility

The repo pins `github.com/go-telegram/bot` v1.20.0. It currently sends stickers
by `models.InputFileString` in `internal/modules/loldle/handlers.go`, and already
uses `models.InputFileUpload` for animation bytes in
`internal/modules/misc/wheelofnames_command.go`. Direct `.TGS` upload combines
those existing patterns.

### `sendSticker` vs `sendDocument`

`sendSticker` makes Telegram render the file as a sticker. `sendDocument` can
transport the `.tgs` as a generic downloadable file (currently up to 50 MB),
but that is not the right method when sticker rendering is desired. A document
`file_id` cannot later be used with `sendSticker`, because Telegram does not
allow changing the media type represented by a `file_id`.

### Security and Performance

- Treat local/embedded `.TGS` assets as untrusted input until validated against
  Telegram's format and size constraints.
- Avoid fetching user-supplied URLs server-side; `.TGS` sticker URLs are not
  supported by `sendSticker` anyway.
- Upload once, persist the returned bot-scoped `file_id`, then resend by
  `file_id` to avoid repeated multipart transfers.

## Implementation Recommendation

For in-memory bytes, matching current repo conventions:

```go
sent, err := b.SendSticker(ctx, &bot.SendStickerParams{
	ChatID:          msg.Chat.ID,
	MessageThreadID: msg.MessageThreadID,
	Sticker: &models.InputFileUpload{
		Filename: "celebration.tgs",
		Data:     bytes.NewReader(tgsData),
	},
	Emoji: "🎉",
})
if err != nil {
	return err
}

fileID := sent.Sticker.FileID
```

For later sends, use the repo's existing pattern:

```go
Sticker: &models.InputFileString{Data: fileID}
```

Common pitfalls:

- Passing an HTTPS `.tgs` URL instead of multipart-uploading it
- Using `sendAnimation`; `.TGS` is an animated sticker, not a GIF-style
  animation payload
- Sending invalid or oversized Lottie data
- Omitting `MessageThreadID` in forum-topic replies
- Assuming another bot's `file_id` is reusable

## References

- [Telegram Bot API: `sendSticker`](https://core.telegram.org/bots/api#sendsticker)
- [Telegram Bot API: sending files](https://core.telegram.org/bots/api#sending-files)
- [Telegram Bot API: `sendDocument`](https://core.telegram.org/bots/api#senddocument)
- [Telegram animated sticker requirements](https://core.telegram.org/stickers#animated-stickers-and-emoji)
- [`SendStickerParams` in go-telegram/bot v1.20.0](https://github.com/go-telegram/bot/blob/v1.20.0/methods_params.go)
- [`InputFileUpload` in go-telegram/bot v1.20.0](https://github.com/go-telegram/bot/blob/v1.20.0/models/input_file.go)

## Next Steps

1. Add the `.tgs` asset only if a concrete command/feature needs it.
2. Validate it against Telegram's limits.
3. Upload with `SendSticker`; retain the returned `file_id` for repeated sends.
4. Add focused handler and multipart-call tests if implementing a command.

## Unresolved Questions

- Which command or event should send the sticker?
- Should the asset ship with the binary or be configured by `file_id`?
