package coin

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/log"
	"github.com/tiennm99/miti99bot/internal/modules/util/chathelper"
)

func (s *state) handleStats(ctx context.Context, b *bot.Bot, update *models.Update) error {
	userID, ok := senderInfo(update)
	if !ok {
		return chathelper.Reply(ctx, b, update.Message, "Cannot identify user - /coin_portfolio needs a sender.")
	}
	p, err := LoadPortfolio(ctx, s.store, userID, s.now().UnixMilli())
	if err != nil {
		log.Error("coin_load_portfolio", "user", userID, "err", err)
		return chathelper.Reply(ctx, b, update.Message, "Could not load coin portfolio. Try again later.")
	}
	header := []string{"Coin Account Summary", "USD: " + FormatUSD(p.USD)}
	var positions []string
	totalValue := p.USD
	totalBasis := 0.0
	missingPrice := false

	// Fetch sequentially (not concurrently) so the price client's keep-alive
	// connection pool is reused across coins rather than opening N simultaneous
	// TLS handshakes. The reply-reserved sub-context bounds the whole loop so the final
	// Reply keeps its budget; a slow/failed provider degrades to "(price
	// unavailable)" instead of failing the summary.
	fetchCtx, cancel := chathelper.FetchContext(ctx)
	defer cancel()
	for _, symbol := range sortedAssetSymbols(p.Assets) {
		held := p.Assets[symbol]
		basis := p.CostBasis[symbol]
		average := basis / held
		line := symbol + ": " + FormatCoinQty(held) + " | Avg " + FormatUSD(average)
		if coin, err := ResolveCoinSymbol(symbol); err == nil {
			if price, err := s.prices.FetchUSD(fetchCtx, coin); err == nil && isPositiveFinite(price.USD) {
				value := held * price.USD
				if !isPositiveFinite(value) || !isPositiveFinite(average) {
					missingPrice = true
					positions = append(positions, symbol+": "+FormatCoinQty(held)+" | valuation unavailable")
					continue
				}
				totalValue += value
				totalBasis += basis
				line += " | Now " + FormatUSD(price.USD) + " (" + price.Source + ")" +
					" | Value " + FormatUSD(value) + " | Unrealized P&L " + FormatPnLUSD(value, basis)
			} else {
				log.Error("coin_fetch_price", "symbol", symbol, "err", err)
				missingPrice = true
				line += " | price unavailable"
			}
		} else {
			missingPrice = true
			line += " | price unavailable"
		}
		positions = append(positions, line)
	}
	var summary []string
	if missingPrice {
		summary = []string{
			"Priced value (partial): " + FormatUSD(totalValue),
			"Unrealized P&L (priced positions): " + FormatPnLUSD(totalValue-p.USD, totalBasis),
			"Invested: " + FormatUSD(p.Meta.Invested),
			"Account P&L: unavailable until all positions have prices",
		}
	} else {
		summary = []string{
			"Total value: " + FormatUSD(totalValue),
			"Invested: " + FormatUSD(p.Meta.Invested),
			"Unrealized P&L: " + FormatPnLUSD(totalValue-p.USD, totalBasis),
			"Account P&L: " + FormatPnLUSD(totalValue, p.Meta.Invested),
		}
	}
	return chathelper.Reply(ctx, b, update.Message, boundedPortfolioReply(header, positions, summary))
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

const portfolioReplyLimit = 4000

func boundedPortfolioReply(header, positions, summary []string) string {
	omitted := 0
	for {
		lines := append(append(append([]string{}, header...), positions...), summary...)
		if omitted > 0 {
			insertAt := len(header) + len(positions)
			lines = append(lines[:insertAt], append([]string{"… " + strconv.Itoa(omitted) + " position(s) omitted"}, lines[insertAt:]...)...)
		}
		reply := strings.Join(lines, "\n")
		if len(reply) <= portfolioReplyLimit || len(positions) == 0 {
			return reply
		}
		positions = positions[:len(positions)-1]
		omitted++
	}
}
