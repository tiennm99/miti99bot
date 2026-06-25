package coin

import (
	"context"
	"sort"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"golang.org/x/sync/errgroup"

	"github.com/tiennm99/miti99bot/internal/log"
	"github.com/tiennm99/miti99bot/internal/modules/util/chathelper"
)

func (s *state) handleStats(ctx context.Context, b *bot.Bot, update *models.Update) error {
	userID, ok := senderInfo(update)
	if !ok {
		return chathelper.Reply(ctx, b, update.Message, "Cannot identify user - /coin_stats needs a sender.")
	}
	p, err := LoadPortfolio(ctx, s.kv, userID, s.now().UnixMilli())
	if err != nil {
		log.Error("coin_load_portfolio", "user", userID, "err", err)
		return chathelper.Reply(ctx, b, update.Message, "Could not load coin portfolio. Try again later.")
	}
	lines := []string{"Coin Account Summary\n", "USD: " + FormatUSD(p.USD)}
	totalValue := p.USD

	// Fetch every held coin's price concurrently (bounded), under a
	// reply-reserved sub-context so a slow provider cannot drain the budget the
	// final Reply needs. Per-coin errors degrade to "(price unavailable)";
	// results are written by index (no shared-write race) and rendered in order.
	symbols := sortedAssetSymbols(p.Assets)
	fetchCtx, cancel := chathelper.FetchContext(ctx)
	defer cancel()
	type coinResult struct {
		line  string
		value float64
	}
	results := make([]coinResult, len(symbols))
	var g errgroup.Group
	g.SetLimit(8)
	for i, symbol := range symbols {
		i, symbol := i, symbol
		g.Go(func() error {
			held := p.Assets[symbol]
			line := symbol + ": " + FormatCoinQty(held)
			if coin, err := ResolveCoinSymbol(symbol); err == nil {
				if price, err := s.prices.FetchUSD(fetchCtx, coin); err == nil {
					value := held * price.USD
					results[i] = coinResult{
						line:  line + " = " + FormatUSD(value) + " @ " + FormatUSD(price.USD) + " (" + price.Source + ")",
						value: value,
					}
					return nil
				}
				line += " (price unavailable)"
			}
			results[i] = coinResult{line: line}
			return nil
		})
	}
	_ = g.Wait() // closures never return an error; partial results are intended
	for _, r := range results {
		lines = append(lines, r.line)
		totalValue += r.value
	}
	lines = append(lines, "Total value: "+FormatUSD(totalValue))
	lines = append(lines, "Invested: "+FormatUSD(p.Meta.Invested))
	lines = append(lines, "P&L: "+FormatPnLUSD(totalValue, p.Meta.Invested))
	return chathelper.Reply(ctx, b, update.Message, strings.Join(lines, "\n"))
}

func sortedAssetSymbols(assets map[string]float64) []string {
	symbols := make([]string, 0, len(assets))
	for symbol, amount := range assets {
		if amount > 0 {
			symbols = append(symbols, symbol)
		}
	}
	sort.Strings(symbols)
	return symbols
}
