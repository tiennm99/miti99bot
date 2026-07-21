package stock

import (
	"context"
	"fmt"
	"html"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/log"
	"github.com/tiennm99/miti99bot/internal/modules/util/chathelper"
)

const (
	dividendFetchTimeout = 12 * time.Second
	dividendFetchWorkers = 4
)

type dividendCheckResult struct {
	symbol   string
	openedAt int64
	events   []DividendEvent
	err      error
}

func (s *state) notifyDividendEvents(ctx context.Context, b *bot.Bot, msg *models.Message, userID int64, p Portfolio, checkedThrough time.Time) error {
	if s.dividends == nil || s.pending == nil || len(p.Assets) == 0 {
		return nil
	}
	s.cleanupExpiredDividends(ctx, checkedThrough.UnixMilli())

	type holding struct {
		symbol string
		qty    int64
		after  time.Time
	}
	holdings := make([]holding, 0, len(p.Assets))
	for symbol, position := range p.Assets {
		if position.Quantity > 0 {
			holdings = append(holdings, holding{
				symbol: symbol,
				qty:    position.Quantity,
				after:  time.UnixMilli(position.DividendCheckedAt),
			})
		}
	}
	if len(holdings) == 0 {
		return nil
	}
	sort.Slice(holdings, func(i, j int) bool { return holdings[i].symbol < holdings[j].symbol })

	fetchCtx, cancel := context.WithTimeout(ctx, dividendFetchTimeout)
	defer cancel()
	results := make([]dividendCheckResult, len(holdings))
	sem := make(chan struct{}, dividendFetchWorkers)
	var wg sync.WaitGroup
	for index, h := range holdings {
		index, h := index, h
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			events, err := s.dividends.FetchDividendEvents(fetchCtx, h.symbol, h.after, checkedThrough)
			results[index] = dividendCheckResult{symbol: h.symbol, openedAt: p.Assets[h.symbol].OpenedAt, events: events, err: err}
		}()
	}
	wg.Wait()

	successful := make([]string, 0, len(results))
	failed := make([]string, 0)
	for _, result := range results {
		if result.err != nil {
			log.Error("stock_fetch_dividend_events", "ticker", result.symbol, "err", result.err)
			failed = append(failed, result.symbol)
			continue
		}
		sort.Slice(result.events, func(i, j int) bool {
			if result.events[i].PublishedAt.Equal(result.events[j].PublishedAt) {
				return result.events[i].ProviderID < result.events[j].ProviderID
			}
			return result.events[i].PublishedAt.Before(result.events[j].PublishedAt)
		})
		delivered := true
		for _, event := range result.events {
			if _, applied := p.AppliedDividendEvents[dividendLedgerKey(event.ProviderID)]; applied {
				continue
			}
			if err := s.sendDividendSuggestion(ctx, b, msg, userID, result.openedAt, checkedThrough, event); err != nil {
				log.Error("stock_send_dividend_suggestion", "user", userID, "ticker", result.symbol, "event", event.ProviderID, "err", err)
				delivered = false
				break
			}
		}
		if delivered {
			successful = append(successful, result.symbol)
		} else {
			failed = append(failed, result.symbol)
		}
	}

	if len(successful) > 0 {
		if err := s.advanceDividendCursors(ctx, userID, successful, checkedThrough.UnixMilli()); err != nil {
			log.Error("stock_save_dividend_cursors", "user", userID, "err", err)
			if replyErr := chathelper.Reply(ctx, b, msg, "Dividend events were checked, but the check time could not be saved. You may see the same suggestions again."); replyErr != nil {
				return replyErr
			}
		}
	}
	if len(failed) > 0 {
		sort.Strings(failed)
		failed = uniqueStrings(failed)
		return chathelper.Reply(ctx, b, msg, "Dividend events could not be checked for "+strings.Join(failed, ", ")+". Try again later.")
	}
	return nil
}

func (s *state) advanceDividendCursors(ctx context.Context, userID int64, symbols []string, checkedThrough int64) error {
	defer s.locks.Acquire(strconv.FormatInt(userID, 10))()
	p, err := LoadPortfolio(ctx, s.store, userID, checkedThrough)
	if err != nil {
		return err
	}
	changed := false
	for _, symbol := range symbols {
		position, ok := p.Assets[symbol]
		if ok && position.DividendCheckedAt < checkedThrough {
			position.DividendCheckedAt = checkedThrough
			p.Assets[symbol] = position
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return SavePortfolio(ctx, s.store, userID, p)
}

func (s *state) sendDividendSuggestion(ctx context.Context, b *bot.Bot, msg *models.Message, userID, expectedOpenedAt int64, checkedThrough time.Time, event DividendEvent) error {
	defer s.locks.Acquire(strconv.FormatInt(userID, 10))()

	now := s.now()
	latest, err := LoadPortfolio(ctx, s.store, userID, now.UnixMilli())
	if err != nil {
		return err
	}
	if _, applied := latest.AppliedDividendEvents[dividendLedgerKey(event.ProviderID)]; applied {
		return nil
	}
	position, held := latest.Assets[event.Symbol]
	if !held || position.Quantity <= 0 || position.OpenedAt != expectedOpenedAt {
		return nil
	}
	observedHolding := position.Quantity
	hasPending, err := s.hasPendingDividendEvent(ctx, userID, event.ProviderID, now.UnixMilli())
	if err != nil {
		return err
	}
	if hasPending {
		return nil
	}
	action := PendingDividendAction{
		OwnerUserID:      userID,
		ChatID:           msg.Chat.ID,
		ProviderEventID:  event.ProviderID,
		Symbol:           event.Symbol,
		Kind:             event.Kind,
		VNDPerShare:      event.VNDPerShare,
		OwnedShares:      event.OwnedShares,
		NewShares:        event.NewShares,
		ObservedHolding:  observedHolding,
		PositionOpenedAt: position.OpenedAt,
		CheckThrough:     checkedThrough.UnixMilli(),
		CreatedAt:        now.UnixMilli(),
		ExpiresAt:        now.Add(pendingDividendTTL).UnixMilli(),
	}
	token, err := s.createPendingDividend(ctx, action)
	if err != nil {
		return err
	}
	key := pendingDividendKey(token)
	sent, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          msg.Chat.ID,
		MessageThreadID: msg.MessageThreadID,
		Text:            dividendEventText(event, observedHolding),
		ParseMode:       models.ParseModeHTML,
		ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{{
			Text:         "Apply dividend",
			CallbackData: dividendCallbackPrefix + token,
		}}}},
	})
	if err != nil {
		_ = s.pending.Delete(ctx, key)
		return err
	}
	action.MessageID = sent.ID
	if err := s.pending.Put(ctx, key, action); err != nil {
		_ = removeDividendButton(ctx, b, action.ChatID, action.MessageID)
		_ = s.pending.Delete(ctx, key)
		return fmt.Errorf("bind dividend action message: %w", err)
	}
	return nil
}

func (s *state) hasPendingDividendEvent(ctx context.Context, userID int64, providerEventID string, now int64) (bool, error) {
	keys, err := s.pending.List(ctx, pendingDividendPrefix)
	if err != nil {
		return false, err
	}
	for _, key := range keys {
		action, _, getErr := s.pending.Get(ctx, key)
		if getErr != nil {
			continue
		}
		if action.ExpiresAt <= now || action.MessageID == 0 {
			_ = s.pending.Delete(ctx, key)
			continue
		}
		if action.OwnerUserID == userID && action.ProviderEventID == providerEventID {
			return true, nil
		}
	}
	return false, nil
}

func dividendEventText(event DividendEvent, observedHolding int64) string {
	var value string
	switch event.Kind {
	case DividendKindCash:
		value = FormatVND(float64(event.VNDPerShare)) + "/share"
	case DividendKindShares:
		value = strconv.FormatInt(event.OwnedShares, 10) + ":" + strconv.FormatInt(event.NewShares, 10) + " shares"
	}
	lines := []string{
		"<b>Recent dividend event · " + html.EscapeString(event.Symbol) + "</b>",
		html.EscapeString(value),
	}
	if !event.ExDate.IsZero() {
		lines = append(lines, "Ex-right: "+event.ExDate.Format("02/01/2006"))
	}
	if !event.RecordDate.IsZero() {
		lines = append(lines, "Record: "+event.RecordDate.Format("02/01/2006"))
	}
	if !event.PaymentDate.IsZero() {
		lines = append(lines, "Payment/trading: "+event.PaymentDate.Format("02/01/2006"))
	}
	if title := truncateRunes(strings.TrimSpace(event.Title), 240); title != "" {
		lines = append(lines, html.EscapeString(title))
	}
	lines = append(lines,
		"SSI event: <code>"+html.EscapeString(event.ProviderID)+"</code>",
		"Observed holding: "+html.EscapeString(formatShareQuantity(observedHolding))+" shares.",
		"The dividend uses your current holding when you accept.",
		"Button expires in 24 hours.",
	)
	return strings.Join(lines, "\n")
}

func truncateRunes(value string, limit int) string {
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit-1]) + "…"
}

func uniqueStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	out := values[:1]
	for _, value := range values[1:] {
		if value != out[len(out)-1] {
			out = append(out, value)
		}
	}
	return out
}
