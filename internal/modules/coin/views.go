package coin

import (
	"context"
	"sort"
	"strconv"

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
	var positions [][]string
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
		held := p.Assets[symbol].Quantity
		basis := p.Assets[symbol].Base
		average := basis / held
		if coin, err := ResolveCoinSymbol(symbol); err == nil {
			if price, err := s.prices.FetchUSD(fetchCtx, coin); err == nil && isPositiveFinite(price.USD) {
				value := held * price.USD
				if !isPositiveFinite(value) || !isPositiveFinite(average) {
					missingPrice = true
					positions = append(positions, []string{symbol, FormatCoinQty(held), "N/A", "N/A", "N/A", "N/A"})
					continue
				}
				totalValue += value
				totalBasis += basis
				positions = append(positions, []string{symbol, FormatCoinQty(held), FormatUSD(average), FormatUSD(price.USD), FormatUSD(value), FormatPnLUSD(value, basis)})
			} else {
				log.Error("coin_fetch_price", "symbol", symbol, "err", err)
				missingPrice = true
				positions = append(positions, []string{symbol, FormatCoinQty(held), FormatUSD(average), "N/A", "N/A", "N/A"})
			}
		} else {
			missingPrice = true
			positions = append(positions, []string{symbol, FormatCoinQty(held), FormatUSD(average), "N/A", "N/A", "N/A"})
		}
	}
	var summary [][]string
	if missingPrice {
		summary = [][]string{
			{"USD", FormatUSD(p.USD)},
			{"Priced value (partial)", FormatUSD(totalValue)},
			{"Unrealized P&L (priced)", FormatPnLUSD(totalValue-p.USD, totalBasis)},
			{"Invested", FormatUSD(p.Meta.Invested)},
			{"Account P&L", "Unavailable"},
		}
	} else {
		summary = [][]string{
			{"USD", FormatUSD(p.USD)},
			{"Total value", FormatUSD(totalValue)},
			{"Invested", FormatUSD(p.Meta.Invested)},
			{"Unrealized P&L", FormatPnLUSD(totalValue-p.USD, totalBasis)},
			{"Account P&L", FormatPnLUSD(totalValue, p.Meta.Invested)},
		}
	}
	return chathelper.ReplyHTML(ctx, b, update.Message, portfolioTableReply("Coin Portfolio", positions, summary))
}

func sortedAssetSymbols(assets map[string]AssetPosition) []string {
	symbols := make([]string, 0, len(assets))
	for symbol, position := range assets {
		if position.Quantity > 0 {
			symbols = append(symbols, symbol)
		}
	}
	sort.Strings(symbols)
	return symbols
}

const portfolioReplyLimit = 4000

func portfolioTableReply(title string, positions, summary [][]string) string {
	omitted := 0
	for {
		rows := append([][]string{}, positions...)
		if omitted > 0 {
			rows = append(rows, []string{"… " + strconv.Itoa(omitted) + " omitted"})
		}
		reply := "<b>" + title + "</b>\n" +
			chathelper.MonospaceTable([]string{"Ticker", "Qty", "Avg", "Now", "Value", "Unrealized P&L"}, rows) + "\n" +
			chathelper.MonospaceTable([]string{"Metric", "Value"}, summary)
		if len(reply) <= portfolioReplyLimit || len(positions) == 0 {
			return reply
		}
		positions = positions[:len(positions)-1]
		omitted++
	}
}
