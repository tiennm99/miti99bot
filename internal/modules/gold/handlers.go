package gold

import (
	"context"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/log"
	"github.com/tiennm99/miti99bot/internal/modules/util/chathelper"
)

func (s *state) handlePrice(ctx context.Context, b *bot.Bot, update *models.Update) error {
	args := argsAfterCommand(update.Message.Text)
	if len(args) != 0 {
		return chathelper.Reply(ctx, b, update.Message, "Usage: /gold_price")
	}
	p, err := s.prices.FetchPrice(ctx)
	if err != nil {
		return s.replyPriceError(ctx, b, update, err)
	}
	lines := []string{
		"Gold Spot Price",
		"XAU: " + FormatUSD(p.XAUUSD) + " USD/oz",
		"Rate: " + FormatVND(p.USDVND) + "/USD",
		"VND: " + FormatVND(p.VNDPerLuong) + "/luong",
	}
	return chathelper.Reply(ctx, b, update.Message, strings.Join(lines, "\n"))
}

func (s *state) handleTopup(ctx context.Context, b *bot.Bot, update *models.Update) error {
	userID, ok := senderInfo(update)
	if !ok {
		return chathelper.Reply(ctx, b, update.Message,
			"Cannot identify user - gold only works in private/group chats with a sender.")
	}
	args := argsAfterCommand(update.Message.Text)
	if len(args) != 1 {
		return chathelper.Reply(ctx, b, update.Message, "Usage: /gold_topup <amount>\nExample: /gold_topup 5000000")
	}
	amount, ok := parsePositiveFinite(args[0])
	if !ok || !isSafeVND(amount) {
		return chathelper.Reply(ctx, b, update.Message, "Amount must be a positive finite number within the supported range.")
	}

	defer s.locks.Acquire(strconv.FormatInt(userID, 10))()
	p, err := LoadPortfolio(ctx, s.kv, userID, s.now().UnixMilli())
	if err != nil {
		log.Error("gold_load_portfolio", "user", userID, "err", err)
		return chathelper.Reply(ctx, b, update.Message, "Could not load gold portfolio. Try again later.")
	}
	p.AddVND(amount)
	p.Meta.Invested += amount
	if err := SavePortfolio(ctx, s.kv, userID, p); err != nil {
		log.Error("gold_save_portfolio", "user", userID, "err", err)
		return chathelper.Reply(ctx, b, update.Message, "Could not save gold portfolio. Try again later.")
	}
	return chathelper.Reply(ctx, b, update.Message,
		"Topped up "+FormatVND(amount)+".\nBalance: "+FormatVND(p.VND))
}

func (s *state) handleBuy(ctx context.Context, b *bot.Bot, update *models.Update) error {
	userID, ok := senderInfo(update)
	if !ok {
		return chathelper.Reply(ctx, b, update.Message,
			"Cannot identify user - gold only works in private/group chats with a sender.")
	}
	args := argsAfterCommand(update.Message.Text)
	if len(args) != 1 {
		return chathelper.Reply(ctx, b, update.Message, "Usage: /gold_buy <luong>\nExample: /gold_buy 1")
	}
	qty, ok := parsePositiveFinite(args[0])
	if !ok {
		return chathelper.Reply(ctx, b, update.Message, "Luong must be a positive finite number.")
	}
	price, err := s.prices.FetchLuongPrice(ctx)
	if err != nil {
		return s.replyPriceError(ctx, b, update, err)
	}
	cost := qty * price
	if !isSafeVND(cost) {
		return chathelper.Reply(ctx, b, update.Message, "Trade value is too large.")
	}

	defer s.locks.Acquire(strconv.FormatInt(userID, 10))()
	p, err := LoadPortfolio(ctx, s.kv, userID, s.now().UnixMilli())
	if err != nil {
		log.Error("gold_load_portfolio", "user", userID, "err", err)
		return chathelper.Reply(ctx, b, update.Message, "Could not load gold portfolio. Try again later.")
	}
	ok, balance := p.DeductVND(cost)
	if !ok {
		return chathelper.Reply(ctx, b, update.Message,
			"Insufficient VND. Need "+FormatVND(cost)+", have "+FormatVND(balance)+".")
	}
	p.AddLuong(qty)
	if err := SavePortfolio(ctx, s.kv, userID, p); err != nil {
		log.Error("gold_save_portfolio", "user", userID, "err", err)
		return chathelper.Reply(ctx, b, update.Message, "Could not save gold portfolio. Try again later.")
	}
	return chathelper.Reply(ctx, b, update.Message,
		"Bought "+FormatLuong(qty)+" luong gold @ "+FormatVND(price)+"/luong\nCost: "+FormatVND(cost)+
			"\nRemaining: "+FormatVND(p.VND))
}

func (s *state) handleSell(ctx context.Context, b *bot.Bot, update *models.Update) error {
	userID, ok := senderInfo(update)
	if !ok {
		return chathelper.Reply(ctx, b, update.Message,
			"Cannot identify user - gold only works in private/group chats with a sender.")
	}
	args := argsAfterCommand(update.Message.Text)
	if len(args) != 1 {
		return chathelper.Reply(ctx, b, update.Message, "Usage: /gold_sell <luong>\nExample: /gold_sell 0.5")
	}
	qty, ok := parsePositiveFinite(args[0])
	if !ok {
		return chathelper.Reply(ctx, b, update.Message, "Luong must be a positive finite number.")
	}
	price, err := s.prices.FetchLuongPrice(ctx)
	if err != nil {
		return s.replyPriceError(ctx, b, update, err)
	}

	defer s.locks.Acquire(strconv.FormatInt(userID, 10))()
	p, err := LoadPortfolio(ctx, s.kv, userID, s.now().UnixMilli())
	if err != nil {
		log.Error("gold_load_portfolio", "user", userID, "err", err)
		return chathelper.Reply(ctx, b, update.Message, "Could not load gold portfolio. Try again later.")
	}
	ok, held := p.DeductLuong(qty)
	if !ok {
		return chathelper.Reply(ctx, b, update.Message,
			"Insufficient gold. You have: "+FormatLuong(held)+" luong")
	}
	revenue := qty * price
	if !isSafeVND(revenue) {
		return chathelper.Reply(ctx, b, update.Message, "Trade value is too large.")
	}
	p.AddVND(revenue)
	if err := SavePortfolio(ctx, s.kv, userID, p); err != nil {
		log.Error("gold_save_portfolio", "user", userID, "err", err)
		return chathelper.Reply(ctx, b, update.Message, "Could not save gold portfolio. Try again later.")
	}
	return chathelper.Reply(ctx, b, update.Message,
		"Sold "+FormatLuong(qty)+" luong gold @ "+FormatVND(price)+"/luong\nRevenue: "+FormatVND(revenue)+
			"\nRemaining: "+FormatVND(p.VND))
}

func (s *state) handleStats(ctx context.Context, b *bot.Bot, update *models.Update) error {
	userID, ok := senderInfo(update)
	if !ok {
		return chathelper.Reply(ctx, b, update.Message,
			"Cannot identify user - /gold_stats needs a sender.")
	}
	p, err := LoadPortfolio(ctx, s.kv, userID, s.now().UnixMilli())
	if err != nil {
		log.Error("gold_load_portfolio", "user", userID, "err", err)
		return chathelper.Reply(ctx, b, update.Message, "Could not load gold portfolio. Try again later.")
	}

	lines := []string{"Gold Account Summary\n", "VND: " + FormatVND(p.VND), "Gold: " + FormatLuong(p.Luong) + " luong"}
	totalValue := p.VND
	if price, err := s.prices.FetchLuongPrice(ctx); err == nil {
		goldValue := p.Luong * price
		totalValue += goldValue
		lines = append(lines, "Price: "+FormatVND(price)+"/luong")
		lines = append(lines, "Gold value: "+FormatVND(goldValue))
		lines = append(lines, "Total value: "+FormatVND(totalValue))
		lines = append(lines, "Invested: "+FormatVND(p.Meta.Invested))
		lines = append(lines, "P&L: "+FormatPnL(totalValue, p.Meta.Invested))
	} else {
		lines = append(lines, "Price: no price")
		lines = append(lines, "Total value: "+FormatVND(totalValue)+" + gold holdings")
	}
	return chathelper.Reply(ctx, b, update.Message, strings.Join(lines, "\n"))
}
