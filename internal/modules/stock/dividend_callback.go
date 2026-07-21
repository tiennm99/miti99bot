package stock

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/log"
	"github.com/tiennm99/miti99bot/internal/modules/util/chathelper"
	"github.com/tiennm99/miti99bot/internal/storage"
)

func answerDividendCallback(ctx context.Context, b *bot.Bot, queryID, text string, alert bool) error {
	_, err := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: queryID,
		Text:            text,
		ShowAlert:       alert,
	})
	return err
}

func removeDividendButton(ctx context.Context, b *bot.Bot, chatID int64, messageID int) error {
	_, err := b.EditMessageReplyMarkup(ctx, &bot.EditMessageReplyMarkupParams{
		ChatID:      chatID,
		MessageID:   messageID,
		ReplyMarkup: &models.InlineKeyboardMarkup{},
	})
	return err
}

func (s *state) handleDividendCallback(ctx context.Context, b *bot.Bot, update *models.Update) error {
	if update == nil || update.CallbackQuery == nil {
		return nil
	}
	query := update.CallbackQuery
	action, msg, actionKey, handled, err := s.resolveDividendCallback(ctx, b, query)
	if err != nil || handled {
		return err
	}
	defer s.locks.Acquire(strconv.FormatInt(action.OwnerUserID, 10))()
	return s.consumeDividendAction(ctx, b, query, msg, actionKey, action)
}

func (s *state) resolveDividendCallback(ctx context.Context, b *bot.Bot, query *models.CallbackQuery) (PendingDividendAction, *models.Message, string, bool, error) {
	token, ok := callbackToken(query.Data)
	if !ok || s.pending == nil {
		err := answerDividendCallback(ctx, b, query.ID, "This dividend suggestion is invalid.", true)
		return PendingDividendAction{}, nil, "", true, err
	}
	actionKey := pendingDividendKey(token)
	action, _, err := s.pending.Get(ctx, actionKey)
	if errors.Is(err, storage.ErrNotFound) {
		err = answerDividendCallback(ctx, b, query.ID, "This dividend suggestion expired or was already used.", true)
		return PendingDividendAction{}, nil, "", true, err
	}
	if err != nil {
		log.Error("stock_load_dividend_action", "err", err)
		err = answerDividendCallback(ctx, b, query.ID, "Could not load this dividend suggestion. Try again.", true)
		return PendingDividendAction{}, nil, "", true, err
	}
	if query.From.ID == 0 || query.From.ID != action.OwnerUserID {
		err = answerDividendCallback(ctx, b, query.ID, "Only the user who requested this portfolio can apply this dividend.", true)
		return PendingDividendAction{}, nil, "", true, err
	}
	msg := query.Message.Message
	if msg == nil || msg.Chat.ID != action.ChatID || msg.ID != action.MessageID || action.MessageID == 0 {
		err = answerDividendCallback(ctx, b, query.ID, "This dividend suggestion is no longer valid here.", true)
		return PendingDividendAction{}, nil, "", true, err
	}
	now := s.now().UnixMilli()
	if action.ExpiresAt <= now {
		_ = s.pending.Delete(ctx, actionKey)
		_ = removeDividendButton(ctx, b, action.ChatID, action.MessageID)
		err = answerDividendCallback(ctx, b, query.ID, "This suggestion expired. Run /stock_portfolio again.", true)
		return PendingDividendAction{}, nil, "", true, err
	}
	return action, msg, actionKey, false, nil
}

func (s *state) consumeDividendAction(ctx context.Context, b *bot.Bot, query *models.CallbackQuery, msg *models.Message, actionKey string, action PendingDividendAction) error {
	now := s.now().UnixMilli()
	// Re-read under the user lock so two callback workers cannot consume the
	// same action using stale state if handler execution becomes asynchronous.
	action, _, err := s.pending.Get(ctx, actionKey)
	if errors.Is(err, storage.ErrNotFound) {
		return answerDividendCallback(ctx, b, query.ID, "This dividend was already handled.", true)
	}
	if err != nil {
		return fmt.Errorf("reload pending dividend action: %w", err)
	}
	p, err := LoadPortfolio(ctx, s.store, action.OwnerUserID, now)
	if err != nil {
		log.Error("stock_load_portfolio", "user", action.OwnerUserID, "err", err)
		return answerDividendCallback(ctx, b, query.ID, "Could not load your portfolio. Try again.", true)
	}
	eventKey := dividendLedgerKey(action.ProviderEventID)
	if _, applied := p.AppliedDividendEvents[eventKey]; applied {
		_ = s.pending.Delete(ctx, actionKey)
		_ = removeDividendButton(ctx, b, action.ChatID, action.MessageID)
		return answerDividendCallback(ctx, b, query.ID, "This dividend was already applied.", true)
	}
	position, held := p.Assets[action.Symbol]
	if !held || position.Quantity <= 0 {
		_ = s.pending.Delete(ctx, actionKey)
		_ = removeDividendButton(ctx, b, action.ChatID, action.MessageID)
		return answerDividendCallback(ctx, b, query.ID, "You no longer hold "+action.Symbol+". Dividend not applied.", true)
	}
	if position.OpenedAt != action.PositionOpenedAt {
		_ = s.pending.Delete(ctx, actionKey)
		_ = removeDividendButton(ctx, b, action.ChatID, action.MessageID)
		return answerDividendCallback(ctx, b, query.ID, "This "+action.Symbol+" position was closed after the suggestion. Run /stock_portfolio again.", true)
	}

	result, err := applySuggestedDividend(&p, action, position.Quantity, now)
	if err != nil {
		log.Error("stock_apply_suggested_dividend", "user", action.OwnerUserID, "ticker", action.Symbol, "event", action.ProviderEventID, "err", err)
		return answerDividendCallback(ctx, b, query.ID, "Could not safely apply this dividend.", true)
	}
	if p.AppliedDividendEvents == nil {
		p.AppliedDividendEvents = map[string]int64{}
	}
	p.AppliedDividendEvents[eventKey] = now
	if err := SavePortfolio(ctx, s.store, action.OwnerUserID, p); err != nil {
		log.Error("stock_save_portfolio", "user", action.OwnerUserID, "err", err)
		return answerDividendCallback(ctx, b, query.ID, "Could not save your portfolio. Try again.", true)
	}

	// The portfolio ledger is the source of truth for idempotency. Cleanup and
	// Telegram UI updates are best-effort after the atomic portfolio save.
	if err := s.pending.Delete(ctx, actionKey); err != nil {
		log.Error("stock_delete_dividend_action", "err", err)
	}
	if err := removeDividendButton(ctx, b, action.ChatID, action.MessageID); err != nil {
		log.Error("stock_remove_dividend_button", "user", action.OwnerUserID, "ticker", action.Symbol, "err", err)
	}
	if err := answerDividendCallback(ctx, b, query.ID, "Dividend applied.", false); err != nil {
		log.Error("stock_answer_dividend_callback", "user", action.OwnerUserID, "ticker", action.Symbol, "err", err)
	}
	return chathelper.Reply(ctx, b, msg, result)
}

func applySuggestedDividend(p *Portfolio, action PendingDividendAction, held, now int64) (string, error) {
	preservedCursor := max(p.Assets[action.Symbol].DividendCheckedAt, action.CheckThrough)
	switch action.Kind {
	case DividendKindCash:
		total, err := cashDividendTotal(held, action.VNDPerShare)
		if err != nil {
			return "", err
		}
		balance, err := checkedVNDBalance(p.VND, total)
		if err != nil {
			return "", err
		}
		if err := p.ApplyDividend(action.Symbol, held, balance, now); err != nil {
			return "", err
		}
		preserveDividendCursor(p, action.Symbol, preservedCursor)
		return "Applied cash dividend for " + action.Symbol + ": " + FormatVND(float64(action.VNDPerShare)) +
			" × " + formatShareQuantity(held) + " = " + FormatVND(float64(total)) +
			"\nBalance: " + FormatVND(balance), nil

	case DividendKindShares:
		ratio := shareRatio{owned: action.OwnedShares, new: action.NewShares,
			raw: strconv.FormatInt(action.OwnedShares, 10) + ":" + strconv.FormatInt(action.NewShares, 10)}
		newShares, err := shareDividendEntitlement(held, ratio)
		if err != nil {
			return "", err
		}
		if newShares == 0 {
			return "", errors.New("share dividend rounds down to zero")
		}
		finalHolding, err := checkedHoldingAfterDividend(held, newShares)
		if err != nil {
			return "", err
		}
		if err := p.ApplyDividend(action.Symbol, finalHolding, p.VND, now); err != nil {
			return "", err
		}
		preserveDividendCursor(p, action.Symbol, preservedCursor)
		return "Applied share dividend for " + action.Symbol + " (" + ratio.raw + "): +" +
			formatShareQuantity(newShares) + "\nHolding: " + formatShareQuantity(held) + " → " +
			formatShareQuantity(finalHolding), nil
	default:
		return "", errors.New("unsupported dividend kind")
	}
}

func preserveDividendCursor(p *Portfolio, symbol string, cursor int64) {
	position := p.Assets[symbol]
	position.DividendCheckedAt = cursor
	p.Assets[symbol] = position
}
