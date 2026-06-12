package coin

import (
	"context"
	"errors"
	"strconv"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/log"
	"github.com/tiennm99/miti99bot/internal/modules/util/chathelper"
)

var (
	errInsufficientUSD  = errors.New("coin: insufficient USD")
	errInsufficientCoin = errors.New("coin: insufficient coin")
)

func (s *state) handlePrice(ctx context.Context, b *bot.Bot, update *models.Update) error {
	args := argsAfterCommand(update.Message.Text)
	if len(args) != 1 {
		return chathelper.Reply(ctx, b, update.Message, "Usage: /coin_price <COIN>\nExample: /coin_price BTC")
	}
	coin, err := ResolveCoinSymbol(args[0])
	if err != nil {
		return s.replyPriceError(ctx, b, update, err)
	}
	price, err := s.prices.FetchUSD(ctx, coin)
	if err != nil {
		return s.replyPriceError(ctx, b, update, err)
	}
	return chathelper.Reply(ctx, b, update.Message,
		coin.Symbol+" price: "+FormatUSD(price.USD)+" ("+price.Source+")")
}

func (s *state) handleTopup(ctx context.Context, b *bot.Bot, update *models.Update) error {
	userID, ok := senderInfo(update)
	if !ok {
		return chathelper.Reply(ctx, b, update.Message,
			"Cannot identify user - coin only works in private/group chats with a sender.")
	}
	args := argsAfterCommand(update.Message.Text)
	if len(args) != 1 {
		return chathelper.Reply(ctx, b, update.Message, "Usage: /coin_topup <usd_amount>\nExample: /coin_topup 1000")
	}
	amount, ok := parsePositiveFinite(args[0])
	if !ok || !isSafeUSD(amount) {
		return chathelper.Reply(ctx, b, update.Message, "Amount must be a positive finite USD number within the supported range.")
	}
	defer s.locks.Acquire(strconv.FormatInt(userID, 10))()
	p, err := UpdatePortfolio(ctx, s.kv, userID, s.now().UnixMilli(), func(p *Portfolio) error {
		p.AddUSD(amount)
		p.Meta.Invested += amount
		return nil
	})
	if err != nil {
		log.Error("coin_save_portfolio", "user", userID, "err", err)
		return chathelper.Reply(ctx, b, update.Message, "Could not save coin portfolio. Try again later.")
	}
	return chathelper.Reply(ctx, b, update.Message,
		"Topped up "+FormatUSD(amount)+".\nBalance: "+FormatUSD(p.USD))
}

func (s *state) handleBuy(ctx context.Context, b *bot.Bot, update *models.Update) error {
	userID, ok := senderInfo(update)
	if !ok {
		return chathelper.Reply(ctx, b, update.Message,
			"Cannot identify user - coin only works in private/group chats with a sender.")
	}
	args := argsAfterCommand(update.Message.Text)
	if len(args) != 2 {
		return chathelper.Reply(ctx, b, update.Message, "Usage: /coin_buy <COIN> <usd_amount>\nExample: /coin_buy BTC 10")
	}
	parsed, err := parseCoinValueArgs(args, isSafeUSD, errInvalidUSDAmount)
	if errors.Is(err, errInvalidUSDAmount) {
		return chathelper.Reply(ctx, b, update.Message, "USD amount must be a positive finite number within the supported range.")
	}
	if err != nil {
		return s.replyPriceError(ctx, b, update, err)
	}
	coin := parsed.coin
	amount := parsed.value
	price, err := s.prices.FetchUSD(ctx, coin)
	if err != nil {
		return s.replyPriceError(ctx, b, update, err)
	}
	qty := amount / price.USD
	if !isPositiveFinite(qty) {
		return chathelper.Reply(ctx, b, update.Message, "Trade value is invalid.")
	}
	defer s.locks.Acquire(strconv.FormatInt(userID, 10))()
	var insufficientBalance *float64
	p, err := UpdatePortfolio(ctx, s.kv, userID, s.now().UnixMilli(), func(p *Portfolio) error {
		ok, balance := p.DeductUSD(amount)
		if !ok {
			insufficientBalance = &balance
			return errInsufficientUSD
		}
		p.AddAsset(coin.Symbol, qty)
		return nil
	})
	if errors.Is(err, errInsufficientUSD) && insufficientBalance != nil {
		return chathelper.Reply(ctx, b, update.Message,
			"Insufficient USD. Need "+FormatUSD(amount)+", have "+FormatUSD(*insufficientBalance)+".")
	}
	if err != nil {
		log.Error("coin_save_portfolio", "user", userID, "err", err)
		return chathelper.Reply(ctx, b, update.Message, "Could not save coin portfolio. Try again later.")
	}
	return chathelper.Reply(ctx, b, update.Message,
		"Bought "+FormatCoinQty(qty)+" "+coin.Symbol+" @ "+FormatUSD(price.USD)+" ("+price.Source+")"+
			"\nCost: "+FormatUSD(amount)+"\nRemaining: "+FormatUSD(p.USD))
}

func (s *state) handleSell(ctx context.Context, b *bot.Bot, update *models.Update) error {
	userID, ok := senderInfo(update)
	if !ok {
		return chathelper.Reply(ctx, b, update.Message,
			"Cannot identify user - coin only works in private/group chats with a sender.")
	}
	args := argsAfterCommand(update.Message.Text)
	if len(args) != 2 {
		return chathelper.Reply(ctx, b, update.Message, "Usage: /coin_sell <COIN> <qty>\nExample: /coin_sell BTC 0.01")
	}
	parsed, err := parseCoinValueArgs(args, isPositiveFinite, errInvalidQuantity)
	if errors.Is(err, errInvalidQuantity) {
		return chathelper.Reply(ctx, b, update.Message, "Quantity must be a positive finite number.")
	}
	if err != nil {
		return s.replyPriceError(ctx, b, update, err)
	}
	coin := parsed.coin
	qty := parsed.value
	price, err := s.prices.FetchUSD(ctx, coin)
	if err != nil {
		return s.replyPriceError(ctx, b, update, err)
	}
	revenue := qty * price.USD
	if !isSafeUSD(revenue) {
		return chathelper.Reply(ctx, b, update.Message, "Trade value is too large.")
	}
	defer s.locks.Acquire(strconv.FormatInt(userID, 10))()
	var insufficientHeld *float64
	p, err := UpdatePortfolio(ctx, s.kv, userID, s.now().UnixMilli(), func(p *Portfolio) error {
		ok, held := p.DeductAsset(coin.Symbol, qty)
		if !ok {
			insufficientHeld = &held
			return errInsufficientCoin
		}
		p.AddUSD(revenue)
		return nil
	})
	if errors.Is(err, errInsufficientCoin) && insufficientHeld != nil {
		return chathelper.Reply(ctx, b, update.Message,
			"Insufficient "+coin.Symbol+". You have: "+FormatCoinQty(*insufficientHeld))
	}
	if err != nil {
		log.Error("coin_save_portfolio", "user", userID, "err", err)
		return chathelper.Reply(ctx, b, update.Message, "Could not save coin portfolio. Try again later.")
	}
	return chathelper.Reply(ctx, b, update.Message,
		"Sold "+FormatCoinQty(qty)+" "+coin.Symbol+" @ "+FormatUSD(price.USD)+" ("+price.Source+")"+
			"\nRevenue: "+FormatUSD(revenue)+"\nRemaining: "+FormatUSD(p.USD))
}
