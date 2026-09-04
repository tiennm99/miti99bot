package alias

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// The kinds an alias can hold. Each maps to exactly one Telegram send method,
// which is the whole reason the kind is stored rather than inferred later: a
// bare file_id does not say which send call will accept it.
const (
	kindSticker   = "sticker"
	kindPhoto     = "photo"
	kindAnimation = "animation"
	kindVideo     = "video"
	kindVideoNote = "video_note"
	kindAudio     = "audio"
	kindVoice     = "voice"
	kindDocument  = "document"
	kindText      = "text"
)

// unsupportedRefusal lists what can be saved, so a refusal teaches rather than
// only denies.
const unsupportedRefusal = "That message cannot be saved. Reply to a sticker, photo, GIF, video, video note, audio, voice message, file, or plain text."

// capture reduces a replied message to a storable alias.
//
// Order matters where Telegram populates more than one field: a GIF arrives as
// an Animation *and* a Document, and a video note as a VideoNote, so the more
// specific kind is claimed first or the alias would come back as a plain file.
func capture(replied *models.Message) (Alias, bool) {
	switch {
	case replied == nil:
		return Alias{}, false
	case replied.Sticker != nil:
		return Alias{Kind: kindSticker, FileID: replied.Sticker.FileID}, true
	case replied.Animation != nil:
		return Alias{Kind: kindAnimation, FileID: replied.Animation.FileID, Text: replied.Caption}, true
	case replied.VideoNote != nil:
		return Alias{Kind: kindVideoNote, FileID: replied.VideoNote.FileID}, true
	case replied.Video != nil:
		return Alias{Kind: kindVideo, FileID: replied.Video.FileID, Text: replied.Caption}, true
	case replied.Voice != nil:
		return Alias{Kind: kindVoice, FileID: replied.Voice.FileID, Text: replied.Caption}, true
	case replied.Audio != nil:
		return Alias{Kind: kindAudio, FileID: replied.Audio.FileID, Text: replied.Caption}, true
	case len(replied.Photo) > 0:
		return Alias{Kind: kindPhoto, FileID: largestPhoto(replied.Photo), Text: replied.Caption}, true
	case replied.Document != nil:
		return Alias{Kind: kindDocument, FileID: replied.Document.FileID, Text: replied.Caption}, true
	case replied.Text != "":
		// Stored as plain text: entities (bold, links, mentions) are dropped,
		// because re-sending them means carrying offsets that no longer line up
		// once the text is repeated in a different message.
		return Alias{Kind: kindText, Text: replied.Text}, true
	}
	return Alias{}, false
}

// largestPhoto picks the best size by file size rather than trusting the
// array's order.
func largestPhoto(sizes []models.PhotoSize) string {
	best := sizes[0]
	for _, size := range sizes[1:] {
		if size.FileSize > best.FileSize {
			best = size
		}
	}
	return best.FileID
}

// send posts a stored alias into the chat msg came from.
//
// MessageThreadID is forwarded on every call for the reason chathelper.Reply
// documents: without it Telegram routes the message to a forum supergroup's
// General topic instead of the topic the command was typed in.
func send(ctx context.Context, b *bot.Bot, msg *models.Message, a Alias) error {
	chatID := msg.Chat.ID
	thread := msg.MessageThreadID
	file := &models.InputFileString{Data: a.FileID}

	var err error
	switch a.Kind {
	case kindSticker:
		// SendStickerParams carries no Caption field — a sticker cannot have one.
		_, err = b.SendSticker(ctx, &bot.SendStickerParams{
			ChatID: chatID, MessageThreadID: thread, Sticker: file,
		})
	case kindPhoto:
		_, err = b.SendPhoto(ctx, &bot.SendPhotoParams{
			ChatID: chatID, MessageThreadID: thread, Photo: file, Caption: a.Text,
		})
	case kindAnimation:
		_, err = b.SendAnimation(ctx, &bot.SendAnimationParams{
			ChatID: chatID, MessageThreadID: thread, Animation: file, Caption: a.Text,
		})
	case kindVideo:
		_, err = b.SendVideo(ctx, &bot.SendVideoParams{
			ChatID: chatID, MessageThreadID: thread, Video: file, Caption: a.Text,
		})
	case kindVideoNote:
		// Nor does a video note: Telegram renders it as a bare round clip.
		_, err = b.SendVideoNote(ctx, &bot.SendVideoNoteParams{
			ChatID: chatID, MessageThreadID: thread, VideoNote: file,
		})
	case kindAudio:
		_, err = b.SendAudio(ctx, &bot.SendAudioParams{
			ChatID: chatID, MessageThreadID: thread, Audio: file, Caption: a.Text,
		})
	case kindVoice:
		_, err = b.SendVoice(ctx, &bot.SendVoiceParams{
			ChatID: chatID, MessageThreadID: thread, Voice: file, Caption: a.Text,
		})
	case kindDocument:
		_, err = b.SendDocument(ctx, &bot.SendDocumentParams{
			ChatID: chatID, MessageThreadID: thread, Document: file, Caption: a.Text,
		})
	case kindText:
		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID, MessageThreadID: thread, Text: a.Text,
		})
	default:
		// A kind written by a newer version of this module, or a corrupted
		// record. Neither is the caller's fault and neither is retryable.
		return errUnknownKind
	}
	return err
}
