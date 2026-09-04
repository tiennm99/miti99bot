package alias

import (
	"context"
	"sort"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/log"
)

const (
	// maxInlineResults is Telegram's cap on results per answerInlineQuery.
	maxInlineResults = 50

	// inlineCacheSeconds is how long Telegram may reuse an answer for the same
	// query. Kept short because the namespace is shared and writable: a name
	// saved now should show up in the picker within seconds, not minutes.
	inlineCacheSeconds = 5
)

// handleInline answers "@botname <prefix>" with the matching aliases.
//
// This is the reason the module stores a file_id rather than bytes: every
// result below is a "Cached" inline type, which takes an id Telegram already
// holds. Nothing is uploaded, and the picker renders real previews.
func (s *state) handleInline(ctx context.Context, b *bot.Bot, update *models.Update) error {
	query := update.InlineQuery
	if query == nil {
		return nil
	}

	names, err := s.store.List(ctx, "")
	if err != nil {
		log.Error("alias_inline_list", "err", err)
		// Answer with nothing rather than leaving the client spinning. An empty
		// answer is also what Telegram expects when a query has no matches.
		return s.answer(ctx, b, query.ID, nil)
	}

	prefix := strings.ToLower(strings.TrimSpace(query.Query))
	matches := make([]string, 0, len(names))
	for _, name := range names {
		if prefix == "" || strings.HasPrefix(name, prefix) {
			matches = append(matches, name)
		}
	}
	sort.Strings(matches)
	if len(matches) > maxInlineResults {
		matches = matches[:maxInlineResults]
	}

	results := make([]models.InlineQueryResult, 0, len(matches))
	for _, name := range matches {
		entry, found, err := s.get(ctx, name)
		if err != nil {
			// One unreadable record must not blank the whole picker.
			log.Error("alias_inline_get", "name", name, "err", err)
			continue
		}
		if !found {
			continue // deleted between the List and this read
		}
		if r := inlineResult(name, entry); r != nil {
			results = append(results, r)
		}
	}
	return s.answer(ctx, b, query.ID, results)
}

func (s *state) answer(ctx context.Context, b *bot.Bot, queryID string, results []models.InlineQueryResult) error {
	_, err := b.AnswerInlineQuery(ctx, &bot.AnswerInlineQueryParams{
		InlineQueryID: queryID,
		Results:       results,
		CacheTime:     inlineCacheSeconds,
	})
	return err
}

// inlineResult maps one alias to the inline result type that carries it, or nil
// when the kind has no cached inline form.
//
// Telegram defines no InlineQueryResultCachedVideoNote, so a video-note alias
// simply does not appear in the picker — it stays reachable through /insert and
// its own /name. Returning nil rather than substituting another type is
// deliberate: sending a round video note as a plain video would change what the
// user saved.
func inlineResult(name string, a Alias) models.InlineQueryResult {
	switch a.Kind {
	case kindSticker:
		// No Title field on this type — Telegram shows the sticker itself.
		return &models.InlineQueryResultCachedSticker{ID: name, StickerFileID: a.FileID}
	case kindPhoto:
		return &models.InlineQueryResultCachedPhoto{
			ID: name, PhotoFileID: a.FileID, Title: name, Caption: a.Text,
		}
	case kindAnimation:
		return &models.InlineQueryResultCachedGif{
			ID: name, GifFileID: a.FileID, Title: name, Caption: a.Text,
		}
	case kindVideo:
		return &models.InlineQueryResultCachedVideo{
			ID: name, VideoFileID: a.FileID, Title: name, Caption: a.Text,
		}
	case kindAudio:
		return &models.InlineQueryResultCachedAudio{
			ID: name, AudioFileID: a.FileID, Caption: a.Text,
		}
	case kindVoice:
		return &models.InlineQueryResultCachedVoice{
			ID: name, VoiceFileID: a.FileID, Title: name, Caption: a.Text,
		}
	case kindDocument:
		return &models.InlineQueryResultCachedDocument{
			ID: name, DocumentFileID: a.FileID, Title: name, Caption: a.Text,
		}
	case kindText:
		return &models.InlineQueryResultArticle{
			ID:                  name,
			Title:               name,
			Description:         a.Text,
			InputMessageContent: &models.InputTextMessageContent{MessageText: a.Text},
		}
	}
	return nil
}
