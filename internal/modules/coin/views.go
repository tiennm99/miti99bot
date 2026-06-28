package coin

import (
	"context"
	"sort"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/log"
	"github.com/tiennm99/miti99bot/internal/modules/util/chathelper"
)

func (s *state) handleStats(ctx context.Context, b *bot.Bot, update *models.Update) error {
	userID, ok := senderInfo(update)
	if !ok {
		return chathelper.Reply(ctx, b, update.Message, "Cannot identify user - /coin_stats needs a sender.")
	}
	p, err := LoadPortfolio(ctx, s.store, userID, s.now().UnixMilli())
	if err != nil {
		log.Error("coin_load_portfolio", "user", userID, "err", err)
		return chathelper.Reply(ctx, b, update.Message, "Could not load coin portfolio. Try again later.")
	}
	lines := []string{"Coin Account Summary\n", "USD: " + FormatUSD(p.USD)}
	totalValue := p.USD

	// Fetch sequentially (not concurrently) so the price client's keep-alive
	// connection pool is reused across coins rather than opening N simultaneous
	// TLS handshakes. The reply-reserved sub-context bounds the whole loop so the final
	// Reply keeps its budget; a slow/failed provider degrades to "(price
	// unavailable)" instead of failing the summary.
	fetchCtx, cancel := chathelper.FetchContext(ctx)
	defer cancel()
	for _, symbol := range sortedAssetSymbols(p.Assets) {
		held := p.Assets[symbol]
		line := symbol + ": " + FormatCoinQty(held)
		if coin, err := ResolveCoinSymbol(symbol); err == nil {
			if price, err := s.prices.FetchUSD(fetchCtx, coin); err == nil {
				value := held * price.USD
				totalValue += value
				line += " = " + FormatUSD(value) + " @ " + FormatUSD(price.USD) + " (" + price.Source + ")"
			} else {
				log.Error("coin_fetch_price", "symbol", symbol, "err", err)
				line += " (price unavailable)"
			}
		}
		lines = append(lines, line)
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
