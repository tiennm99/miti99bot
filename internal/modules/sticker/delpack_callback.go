package sticker

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/log"
	"github.com/tiennm99/miti99bot/internal/storage"
)

// handleDelPack asks for confirmation before destroying the caller's pack.
//
// /delpack is also the *only* way to change a pack's URL, since Telegram has no
// rename-short-name method. That makes this prompt the last point at which a
// user who came here wanting a new link learns that the stickers do not survive
// the change — so it states the title, the count being lost, the exact link
// being surrendered, and that both are permanent.
func (s *state) handleDelPack(ctx context.Context, b *bot.Bot, update *models.Update) error {
	ctx, cancel := handlerContext(ctx)
	defer cancel()

	msg := update.Message
	ownerID, err := senderID(msg)
	if err != nil {
		return reply(ctx, b, msg, senderRefusal)
	}

	pack, found, err := getPack(ctx, s.store, ownerID)
	if err != nil {
		log.Error("sticker_delpack_load", "err", err)
		return reply(ctx, b, msg, genericFailure)
	}
	if !found {
		return reply(ctx, b, msg, noPackYet)
	}

	// A pending record is bookkeeping, not proof that this bot created a set
	// under that name on this user's behalf — /newpack writes it before
	// Telegram is called, and anyone can make one naming any set. Deleting by
	// set name is authorised by Telegram for every set this bot created, so
	// confirming a delete from a pending record would let one user destroy
	// another's pack. Clear the local record only, and touch nothing upstream.
	if pack.Pending {
		defer s.lockUser(ownerID)()
		s.dropPackRecord(ctx, ownerID)
		return reply(ctx, b, msg, fmt.Sprintf(
			"Cleared an unfinished attempt at %s and freed the name. Nothing was deleted at Telegram; if that attempt did create a pack, it is no longer reachable through this bot.",
			pack.Slug))
	}

	id, err := newActionID()
	if err != nil {
		log.Error("sticker_delpack_id", "err", err)
		return reply(ctx, b, msg, genericFailure)
	}

	now := s.now()
	action := PendingDelete{
		ID:        id,
		OwnerID:   ownerID,
		Slug:      pack.Slug,
		SetName:   pack.Name,
		ChatID:    msg.Chat.ID,
		CreatedAt: now.UnixMilli(),
		ExpiresAt: now.Add(pendingDeleteTTL).UnixMilli(),
	}

	sent, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          msg.Chat.ID,
		ReplyParameters: &models.ReplyParameters{MessageID: msg.ID},
		Text: fmt.Sprintf(
			"Delete %s (%s)?\n\nThis destroys %d sticker(s) and gives up %s permanently. Neither can be recovered, and the link may not be reusable.",
			pack.Title, pack.Slug, pack.Count, shareLink(pack.Name)),
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{{
				{Text: "Delete permanently", CallbackData: deleteCallbackData(id)},
			}},
		},
	})
	if err != nil {
		log.Error("sticker_delpack_prompt", "err", err)
		return err
	}

	// Bind the action to the message carrying the button, so a press from a
	// forwarded or replayed copy resolves to nothing.
	action.MessageID = sent.ID
	commitCtx, cancelCommit := commitContext(ctx)
	defer cancelCommit()
	if err := s.pending.Put(commitCtx, pendingDeleteKey(ownerID), action); err != nil {
		log.Error("sticker_delpack_store", "err", err)
		return reply(ctx, b, msg, genericFailure)
	}
	return nil
}

// handleDelPackCallback consumes a confirm press.
func (s *state) handleDelPackCallback(ctx context.Context, b *bot.Bot, update *models.Update) error {
	ctx, cancel := handlerContext(ctx)
	defer cancel()

	if update == nil || update.CallbackQuery == nil {
		return nil
	}
	query := update.CallbackQuery

	id, ok := parseDeleteCallback(query.Data)
	if !ok {
		return answerCallback(ctx, b, query.ID, "This confirmation is invalid.")
	}

	// The lookup is keyed by the presser, never by the payload: the payload is
	// client-controlled, so using it to choose *whose* action to load would let
	// anyone address someone else's confirmation.
	if query.From.ID == 0 {
		return answerCallback(ctx, b, query.ID, "This confirmation is invalid.")
	}
	key := pendingDeleteKey(query.From.ID)
	action, _, err := s.pending.Get(ctx, key)
	if errors.Is(err, storage.ErrNotFound) {
		return answerCallback(ctx, b, query.ID, "This confirmation expired or was already used.")
	}
	if err != nil {
		log.Error("sticker_delpack_action_load", "err", err)
		return answerCallback(ctx, b, query.ID, "Could not load this confirmation. Try /delpack again.")
	}

	// Unreachable by construction — the action was loaded under this presser's
	// own key, so a foreign press already returned above. Kept as defence in
	// depth against a future change to how actions are addressed.
	if query.From.ID != action.OwnerID {
		return answerCallback(ctx, b, query.ID, "Only the user who ran /delpack can confirm it.")
	}

	// CallbackQuery.Message is a MaybeInaccessibleMessage: nil for messages
	// Telegram considers inaccessible. The panic barrier is the backstop, not a
	// reason to skip the guard — and this must come before any use of msg.
	msg := query.Message.Message
	if msg == nil {
		return answerCallback(ctx, b, query.ID, "This confirmation is no longer valid here.")
	}

	// Binding first, and with no side effect. This press may be on somebody
	// else's prompt: anyone in a group can tap anyone's button, so touching the
	// message before proving it is the one this action was written for let a
	// bystander strip the button off a live confirmation they had no part in.
	//
	// It also subsumes the stale-prompt case — an older prompt is a different
	// message id, so it fails here.
	if msg.Chat.ID != action.ChatID || msg.ID != action.MessageID || action.MessageID == 0 {
		return answerCallback(ctx, b, query.ID, "This confirmation is no longer valid here.")
	}

	// Defence in depth: the binding above already implies this, since a newer
	// /delpack writes a new message id. Clearing is safe here only because the
	// binding proved this is the caller's own bound message.
	if action.ID != id {
		clearButton(ctx, b, msg.Chat.ID, msg.ID)
		return answerCallback(ctx, b, query.ID, "This confirmation was replaced by a newer /delpack.")
	}

	if action.ExpiresAt <= s.now().UnixMilli() {
		s.dropPendingDelete(ctx, key)
		clearButton(ctx, b, action.ChatID, action.MessageID)
		return answerCallback(ctx, b, query.ID, "This confirmation expired. Run /delpack again.")
	}

	defer s.lockUser(action.OwnerID)()

	// Re-establish, under the lock, that the caller still holds this exact set.
	//
	// The authority to delete comes from the record, not from the prompt, and a
	// prompt outlives the record: /delpack can sit unpressed for ten minutes
	// while the pack disappears from Telegram's side, a self-heal frees the
	// name, and somebody else claims it. DeleteStickerSet is keyed by set name
	// and Telegram authorises it for every set this bot created, so a press
	// then lands on whoever holds the name at that moment.
	//
	// Stated as an allowlist deliberately. The first version of this guard
	// listed the states it would refuse — a pending record still naming this
	// set — and fell through on the two that mattered: no record at all, and a
	// record that had moved on to a different pack. Proving authority is the
	// only formulation that fails closed against a state nobody thought of.
	current, found, err := getPack(ctx, s.store, action.OwnerID)
	if err != nil {
		log.Error("sticker_delpack_recheck", "err", err)
		return answerCallback(ctx, b, query.ID, "Could not confirm right now. Try /delpack again.")
	}
	if !found || current.Pending || !ownsSet(current, action.SetName) {
		s.dropPendingDelete(ctx, key)
		clearButton(ctx, b, action.ChatID, action.MessageID)
		return answerCallback(ctx, b, query.ID,
			"This confirmation is out of date — that pack is no longer yours to delete. Run /delpack again if you still want to.")
	}

	// Consume the action *before* the destructive call, so a double press
	// cannot delete twice or race a second confirmation.
	if err := s.pending.Delete(ctx, key); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return answerCallback(ctx, b, query.ID, "This confirmation was already used.")
		}
		log.Error("sticker_delpack_consume", "err", err)
		return answerCallback(ctx, b, query.ID, "Could not confirm right now. Try /delpack again.")
	}

	_, err = b.DeleteStickerSet(ctx, &bot.DeleteStickerSetParams{Name: action.SetName})
	switch {
	case err == nil, isStickerSetMissing(err):
		// Missing counts as success: the set is gone either way, and clearing
		// the record is what unblocks /newpack.
		//
		// But clear it only if it still names *this* set. dropPackRecord deletes
		// by owner, and the user's record may have moved on to a different pack
		// since this confirmation was written — that is exactly what the
		// documented /delpack-then-/newpack URL-change route does. Deleting
		// blindly by owner would then erase a live pack's record.
		s.dropPackRecordIfSet(ctx, action.OwnerID, action.SetName)
		clearButton(ctx, b, action.ChatID, action.MessageID)
		// Reply to the prompt rather than sending bare to the chat: in a forum
		// supergroup a bare ChatID send lands in General instead of the topic
		// the button lives in, leaking the pack name across topics and losing
		// the confirmation. chathelper.Reply carries MessageThreadID.
		_ = reply(ctx, b, msg, fmt.Sprintf("Deleted %s. You can create a new pack with /newpack.", action.Slug))
		return answerCallback(ctx, b, query.ID, "Pack deleted.")

	default:
		// The record stays: we have no positive signal that the set is gone.
		log.Error("sticker_delpack_delete", "err", err)
		clearButton(ctx, b, action.ChatID, action.MessageID)
		return answerCallback(ctx, b, query.ID, "Telegram refused the delete. Your pack is unchanged.")
	}
}

func (s *state) dropPendingDelete(ctx context.Context, key string) {
	commitCtx, cancel := commitContext(ctx)
	defer cancel()
	if err := s.pending.Delete(commitCtx, key); err != nil && !errors.Is(err, storage.ErrNotFound) {
		log.Error("sticker_drop_pending_delete", "err", err)
	}
}

func answerCallback(ctx context.Context, b *bot.Bot, queryID, text string) error {
	_, err := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: queryID,
		Text:            text,
		ShowAlert:       true,
	})
	return err
}

// clearButton removes the inline keyboard so a spent prompt cannot be pressed
// again. Best effort: the action is already consumed either way.
func clearButton(ctx context.Context, b *bot.Bot, chatID int64, messageID int) {
	_, err := b.EditMessageReplyMarkup(ctx, &bot.EditMessageReplyMarkupParams{
		ChatID:      chatID,
		MessageID:   messageID,
		ReplyMarkup: &models.InlineKeyboardMarkup{},
	})
	if err != nil {
		log.Error("sticker_clear_button", "err", err)
	}
}
