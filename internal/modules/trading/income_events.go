package trading

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/log"
	"github.com/tiennm99/miti99bot/internal/modules/util/chathelper"
)

const (
	fireAntIncomeEventsDefaultURL = "https://restv2.fireant.vn"
	incomeEventsHTTPTimeout       = 10 * time.Second
	incomeEventsLookback          = 30 * 24 * time.Hour
)

type IncomeEvent struct {
	Symbol     string
	Title      string
	Subtitle   string
	DeployDate time.Time
	Link       string
}

type IncomeEventClient struct {
	HTTP  *http.Client
	URL   string
	Token string

	defaultOnce   sync.Once
	defaultClient *http.Client
}

func NewIncomeEventClientFromEnv() *IncomeEventClient {
	url := strings.TrimSpace(os.Getenv("TRADING_INCOME_EVENTS_API_URL"))
	if url == "" {
		url = fireAntIncomeEventsDefaultURL
	}
	return &IncomeEventClient{
		URL:   url,
		Token: strings.TrimSpace(os.Getenv("TRADING_INCOME_EVENTS_API_TOKEN")),
	}
}

func (c *IncomeEventClient) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	c.defaultOnce.Do(func() {
		c.defaultClient = &http.Client{Timeout: incomeEventsHTTPTimeout}
	})
	return c.defaultClient
}

type fireAntTimescaleMark struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Date  string `json:"date"`
	Title string `json:"title"`
	Color string `json:"color"`
}

var (
	ErrNoIncomeEvents                 = errors.New("trading: no income events")
	ErrIncomeEventClientNotConfigured = errors.New("trading: income events API not configured")
	ErrIncomeEventAuthRequired        = errors.New("trading: income events API authentication required")
)

func (c *IncomeEventClient) FetchRecent(ctx context.Context, ticker string, since, until time.Time) ([]IncomeEvent, error) {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	if !tickerRe.MatchString(ticker) {
		return nil, ErrUnknownTicker
	}
	if strings.TrimSpace(c.URL) == "" {
		return nil, ErrIncomeEventClientNotConfigured
	}

	fullURL, err := fireAntMarksURL(c.URL, ticker, since, until)
	if err != nil {
		return nil, err
	}
	req, err := fireAntRequest(ctx, fullURL, c.Token)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("trading: FireAnt request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	marks, err := decodeFireAntMarks(resp)
	if err != nil {
		return nil, err
	}
	events := incomeEventsFromMarks(ticker, marks, since, until)
	if len(events) == 0 {
		return nil, ErrNoIncomeEvents
	}
	return events, nil
}

func fireAntMarksURL(baseURL, ticker string, since, until time.Time) (string, error) {
	endpoint, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("trading: parse FireAnt URL: %w", err)
	}
	if !isSafeFireAntEndpoint(endpoint) {
		return "", fmt.Errorf("trading: income events API URL must be https")
	}
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/symbols/" + url.PathEscape(ticker) + "/timescale-marks"
	q := endpoint.Query()
	q.Set("startDate", since.Format(time.RFC3339))
	q.Set("endDate", until.Format(time.RFC3339))
	endpoint.RawQuery = q.Encode()
	return endpoint.String(), nil
}

func isSafeFireAntEndpoint(endpoint *url.URL) bool {
	if endpoint.Scheme == "https" {
		return true
	}
	if endpoint.Scheme != "http" {
		return false
	}
	return strings.HasPrefix(endpoint.Host, "127.0.0.1:") || strings.HasPrefix(endpoint.Host, "localhost:")
}

func fireAntRequest(ctx context.Context, fullURL, token string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("trading: build FireAnt request: %w", err)
	}
	req.Header.Set("User-Agent", "miti99bot")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req, nil
}

func decodeFireAntMarks(resp *http.Response) ([]fireAntTimescaleMark, error) {
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, ErrIncomeEventAuthRequired
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNoIncomeEvents
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("trading: FireAnt status %d", resp.StatusCode)
	}

	var marks []fireAntTimescaleMark
	if err := json.NewDecoder(resp.Body).Decode(&marks); err != nil {
		return nil, fmt.Errorf("trading: FireAnt decode: %w", err)
	}
	return marks, nil
}

func incomeEventsFromMarks(ticker string, marks []fireAntTimescaleMark, since, until time.Time) []IncomeEvent {
	var out []IncomeEvent
	for _, mark := range marks {
		date, ok := parseFireAntDate(mark.Date)
		if !ok || date.Before(since) || date.After(until) || !isIncomeEventMark(mark) {
			continue
		}
		out = append(out, incomeEventFromMark(ticker, mark, date))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].DeployDate.After(out[j].DeployDate)
	})
	return out
}

func incomeEventFromMark(ticker string, mark fireAntTimescaleMark, date time.Time) IncomeEvent {
	title := cleanIncomeEventText(mark.Title)
	label := cleanIncomeEventText(mark.Label)
	if title == "" {
		title = label
	}
	subtitle := ""
	if label != "" && label != title {
		subtitle = label
	}
	return IncomeEvent{Symbol: ticker, Title: title, Subtitle: subtitle, DeployDate: date}
}

func parseFireAntDate(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02", "02/01/2006"} {
		parsed, err := time.Parse(layout, raw)
		if err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

func isIncomeEventMark(mark fireAntTimescaleMark) bool {
	text := normalizeIncomeEventSearchText(mark.Label + " " + mark.Title)
	terms := []string{
		"co tuc",
		"dividend",
		"quyen mua",
		"phat hanh co phieu",
		"chia co phieu",
		"bonus share",
		"stock dividend",
		"cash dividend",
	}
	for _, term := range terms {
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func normalizeIncomeEventSearchText(s string) string {
	s = strings.ToLower(cleanIncomeEventText(s))
	replacer := strings.NewReplacer(
		"à", "a", "á", "a", "ạ", "a", "ả", "a", "ã", "a", "â", "a", "ầ", "a", "ấ", "a", "ậ", "a", "ẩ", "a", "ẫ", "a", "ă", "a", "ằ", "a", "ắ", "a", "ặ", "a", "ẳ", "a", "ẵ", "a",
		"è", "e", "é", "e", "ẹ", "e", "ẻ", "e", "ẽ", "e", "ê", "e", "ề", "e", "ế", "e", "ệ", "e", "ể", "e", "ễ", "e",
		"ì", "i", "í", "i", "ị", "i", "ỉ", "i", "ĩ", "i",
		"ò", "o", "ó", "o", "ọ", "o", "ỏ", "o", "õ", "o", "ô", "o", "ồ", "o", "ố", "o", "ộ", "o", "ổ", "o", "ỗ", "o", "ơ", "o", "ờ", "o", "ớ", "o", "ợ", "o", "ở", "o", "ỡ", "o",
		"ù", "u", "ú", "u", "ụ", "u", "ủ", "u", "ũ", "u", "ư", "u", "ừ", "u", "ứ", "u", "ự", "u", "ử", "u", "ữ", "u",
		"ỳ", "y", "ý", "y", "ỵ", "y", "ỷ", "y", "ỹ", "y",
		"đ", "d",
	)
	return replacer.Replace(s)
}

func cleanIncomeEventText(s string) string {
	return strings.Join(strings.Fields(html.UnescapeString(s)), " ")
}

func RenderIncomeEvents(events []IncomeEvent, since, until time.Time) string {
	if len(events) == 0 {
		return "No recent income events from FireAnt in the last 30 days."
	}
	var lines []string
	lines = append(lines, "Income events from FireAnt")
	lines = append(lines, since.Format("02/01/2006")+" - "+until.Format("02/01/2006"))

	for _, event := range events {
		line := event.Symbol + " - " + event.DeployDate.Format("02/01/2006") + ": " + event.Title
		if event.Subtitle != "" {
			line += "\n  " + event.Subtitle
		}
		if event.Link != "" {
			line += "\n  " + event.Link
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (s *state) handleIncomeEvents(ctx context.Context, b *bot.Bot, update *models.Update) error {
	userID, ok := senderInfo(update)
	if !ok {
		return chathelper.Reply(ctx, b, update.Message,
			"Cannot identify user - /trade_income_events needs a sender.")
	}

	args := argsAfterCommand(update.Message.Text)
	symbols, err := s.incomeEventSymbols(ctx, userID, args)
	if err != nil {
		if errors.Is(err, ErrUnknownTicker) {
			ticker := ""
			if len(args) > 0 {
				ticker = strings.ToUpper(args[0])
			}
			return chathelper.Reply(ctx, b, update.Message, "Unknown stock ticker \""+ticker+"\".")
		}
		log.Error("trading_income_events_symbols", "user", userID, "err", err)
		return chathelper.Reply(ctx, b, update.Message, "Could not load holdings. Try again later.")
	}
	if len(symbols) == 0 {
		return chathelper.Reply(ctx, b, update.Message,
			"You don't hold any stocks yet. Usage: /trade_income_events <TICKER>")
	}

	until := s.now().UTC()
	since := until.Add(-incomeEventsLookback)
	var all []IncomeEvent
	var failed []string
	var notConfigured bool
	for _, symbol := range symbols {
		events, err := s.incomeEvents.FetchRecent(ctx, symbol, since, until)
		if err != nil {
			if errors.Is(err, ErrIncomeEventClientNotConfigured) {
				notConfigured = true
				break
			}
			if errors.Is(err, ErrIncomeEventAuthRequired) {
				return chathelper.Reply(ctx, b, update.Message,
					"FireAnt income events API requires authentication. Set TRADING_INCOME_EVENTS_API_TOKEN or TRADING_INCOME_EVENTS_API_TOKEN_PARAMETER_NAME.")
			}
			if errors.Is(err, ErrNoIncomeEvents) {
				continue
			}
			log.Error("trading_fetch_income_events", "ticker", symbol, "err", err)
			failed = append(failed, symbol)
			continue
		}
		all = append(all, events...)
	}
	if notConfigured {
		return chathelper.Reply(ctx, b, update.Message,
			"Income events API is not configured. Set TRADING_INCOME_EVENTS_API_URL or use the FireAnt default.")
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].DeployDate.Equal(all[j].DeployDate) {
			return all[i].Symbol < all[j].Symbol
		}
		return all[i].DeployDate.After(all[j].DeployDate)
	})

	reply := RenderIncomeEvents(all, since, until)
	if len(failed) > 0 {
		reply += "\nCould not fetch: " + strings.Join(failed, ", ")
	}
	return chathelper.Reply(ctx, b, update.Message, reply)
}

func (s *state) incomeEventSymbols(ctx context.Context, userID int64, args []string) ([]string, error) {
	if len(args) > 0 {
		resolved, err := ResolveSymbol(ctx, s.kv, s.prices, args[0])
		if err != nil {
			return nil, err
		}
		return []string{resolved.Symbol}, nil
	}

	p, err := LoadPortfolio(ctx, s.kv, userID, s.now().UnixMilli())
	if err != nil {
		return nil, err
	}
	var symbols []string
	for symbol, qty := range p.Assets {
		if qty > 0 {
			symbols = append(symbols, symbol)
		}
	}
	sort.Strings(symbols)
	return symbols, nil
}
