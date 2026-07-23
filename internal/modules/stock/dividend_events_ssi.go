package stock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	ssiDividendDefaultURL = "https://iboard-api.ssi.com.vn"
	ssiDividendPath       = "/statistics/company/corporate-actions"
	ssiDividendPageSize   = 50
	ssiDividendMaxPages   = 20
	ssiDividendTimeout    = 3 * time.Second
)

var saigonLocation = time.FixedZone("Asia/Saigon", 7*60*60)
var ssiProviderIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// SSIDividendProvider reads corporate actions from SSI iBoard's undocumented
// public endpoint. It is isolated behind DividendEventProvider because SSI does
// not publish a compatibility or availability guarantee for this endpoint.
type SSIDividendProvider struct {
	HTTP    *http.Client
	BaseURL string

	defaultOnce   sync.Once
	defaultClient *http.Client
}

type ssiDividendResponse struct {
	Code   string             `json:"code"`
	Status string             `json:"status"`
	Data   []ssiDividendEvent `json:"data"`
	Paging struct {
		TotalPage int `json:"totalPage"`
		Page      int `json:"page"`
	} `json:"paging"`
}

type ssiDividendEvent struct {
	CorID            string     `json:"CorId"`
	Symbol           string     `json:"symbol"`
	EventListCode    string     `json:"eventListCode"`
	EventName        string     `json:"eventName"`
	EventTitle       string     `json:"eventTitle"`
	EventDescription string     `json:"eventDescription"`
	ExrightDate      string     `json:"exrightDate"`
	RecordDate       string     `json:"recordDate"`
	IssueDate        string     `json:"issueDate"`
	PublicDate       string     `json:"publicDate"`
	Value            ssiDecimal `json:"value"`
	Ratio            ssiDecimal `json:"ratio"`
}

// ssiDecimal retains the exact JSON spelling whether SSI encodes a decimal as
// a JSON string (current behavior) or changes it to a JSON number.
type ssiDecimal string

func (d *ssiDecimal) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*d = ""
		return nil
	}
	if len(data) > 0 && data[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*d = ssiDecimal(value)
		return nil
	}
	value := string(data)
	if _, ok := new(big.Rat).SetString(value); !ok {
		return fmt.Errorf("invalid decimal %q", value)
	}
	*d = ssiDecimal(value)
	return nil
}

func (p *SSIDividendProvider) httpClient() *http.Client {
	if p.HTTP != nil {
		return p.HTTP
	}
	p.defaultOnce.Do(func() {
		p.defaultClient = &http.Client{Timeout: ssiDividendTimeout}
	})
	return p.defaultClient
}

func (p *SSIDividendProvider) endpoint() string {
	baseURL := strings.TrimRight(strings.TrimSpace(p.BaseURL), "/")
	if baseURL == "" {
		baseURL = ssiDividendDefaultURL
	}
	return baseURL + ssiDividendPath
}

// FetchDividendEvents returns only validated cash dividends and explicitly
// described share dividends. SSI is queried with a one-calendar-day overlap in
// Asia/Saigon, then results are filtered by publication time to (after, through].
func (p *SSIDividendProvider) FetchDividendEvents(ctx context.Context, symbol string, after, through time.Time) ([]DividendEvent, error) {
	symbol, from, to, err := prepareSSIEventFetch(
		symbol,
		after,
		through,
		"stock: dividend symbol is empty",
		"stock: invalid dividend event range",
	)
	if err != nil {
		return nil, err
	}

	rawEvents, err := p.fetchUniqueCorporateActions(ctx, symbol, from, to, "dividend")
	if err != nil {
		return nil, err
	}
	lowerBound := startOfSaigonDay(after).AddDate(0, 0, -1)
	events := make([]DividendEvent, 0)
	for _, raw := range rawEvents {
		event, ok := p.normalizeEvent(raw, symbol)
		if !ok || event.PublishedAt.Before(lowerBound) || event.PublishedAt.After(through) {
			continue
		}
		events = append(events, event)
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].PublishedAt.Equal(events[j].PublishedAt) {
			return events[i].ProviderID < events[j].ProviderID
		}
		return events[i].PublishedAt.Before(events[j].PublishedAt)
	})
	return events, nil
}

// FetchStockEvents returns all displayable SSI corporate actions. It is
// intentionally separate from FetchDividendEvents so read-only event listing
// cannot relax portfolio dividend validation.
func (p *SSIDividendProvider) FetchStockEvents(ctx context.Context, symbol string, after, through time.Time) ([]SSIStockEvent, error) {
	symbol, from, to, err := prepareSSIEventFetch(
		symbol,
		after,
		through,
		"stock: event symbol is empty",
		"stock: invalid stock event range",
	)
	if err != nil {
		return nil, err
	}

	rawEvents, err := p.fetchUniqueCorporateActions(ctx, symbol, from, to, "event")
	if err != nil {
		return nil, err
	}
	events := make([]SSIStockEvent, 0)
	for _, raw := range rawEvents {
		event, ok := p.copyStockEvent(raw, symbol)
		if !ok || !event.cursorAt.After(after) || event.cursorAt.After(through) {
			continue
		}
		events = append(events, event)
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].cursorAt.Equal(events[j].cursorAt) {
			return events[i].CorID < events[j].CorID
		}
		return events[i].cursorAt.Before(events[j].cursorAt)
	})
	return events, nil
}

func prepareSSIEventFetch(symbol string, after, through time.Time, emptyErr, rangeErr string) (normalized string, from, to time.Time, err error) {
	normalized = strings.ToUpper(strings.TrimSpace(symbol))
	if normalized == "" {
		return "", time.Time{}, time.Time{}, errors.New(emptyErr)
	}
	if after.IsZero() || through.IsZero() || !through.After(after) {
		return "", time.Time{}, time.Time{}, errors.New(rangeErr)
	}
	return normalized, after.In(saigonLocation).AddDate(0, 0, -1), through.In(saigonLocation), nil
}

func (p *SSIDividendProvider) fetchUniqueCorporateActions(ctx context.Context, symbol string, from, to time.Time, kind string) ([]ssiDividendEvent, error) {
	seen := make(map[string]struct{})
	events := make([]ssiDividendEvent, 0)
	totalPages := 1
	for page := 1; page <= totalPages; page++ {
		if page > ssiDividendMaxPages {
			return nil, fmt.Errorf("stock: SSI %s response exceeds %d pages", kind, ssiDividendMaxPages)
		}
		body, err := p.fetchPage(ctx, symbol, from, to, page)
		if err != nil {
			return nil, fmt.Errorf("stock: fetch SSI %s page %d: %w", kind, page, err)
		}
		if body.Paging.TotalPage > ssiDividendMaxPages {
			return nil, fmt.Errorf("stock: SSI %s response has %d pages, maximum is %d", kind, body.Paging.TotalPage, ssiDividendMaxPages)
		}
		if body.Paging.TotalPage > totalPages {
			totalPages = body.Paging.TotalPage
		}
		for _, raw := range body.Data {
			id := strings.TrimSpace(raw.CorID)
			if !ssiProviderIDPattern.MatchString(id) {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			events = append(events, raw)
		}
	}
	return events, nil
}

func (p *SSIDividendProvider) copyStockEvent(raw ssiDividendEvent, requestedSymbol string) (SSIStockEvent, bool) {
	symbol := strings.ToUpper(strings.TrimSpace(raw.Symbol))
	if symbol != requestedSymbol {
		return SSIStockEvent{}, false
	}
	cursorAt, ok := ssiStockEventCursor(raw)
	if !ok {
		return SSIStockEvent{}, false
	}
	return SSIStockEvent{
		CorID:            strings.TrimSpace(raw.CorID),
		Symbol:           symbol,
		EventListCode:    strings.TrimSpace(raw.EventListCode),
		EventName:        strings.TrimSpace(raw.EventName),
		EventTitle:       strings.TrimSpace(raw.EventTitle),
		EventDescription: strings.TrimSpace(raw.EventDescription),
		PublicDate:       strings.TrimSpace(raw.PublicDate),
		ExrightDate:      strings.TrimSpace(raw.ExrightDate),
		RecordDate:       strings.TrimSpace(raw.RecordDate),
		IssueDate:        strings.TrimSpace(raw.IssueDate),
		Value:            strings.TrimSpace(string(raw.Value)),
		Ratio:            strings.TrimSpace(string(raw.Ratio)),
		SourceURL:        p.eventSourceURL(symbol, raw.CorID),
		cursorAt:         cursorAt,
	}, true
}

func ssiStockEventCursor(raw ssiDividendEvent) (time.Time, bool) {
	if strings.TrimSpace(raw.PublicDate) != "" {
		return parseSSIDate(raw.PublicDate)
	}
	for _, candidate := range []string{raw.ExrightDate, raw.RecordDate, raw.IssueDate} {
		if parsed, ok := parseSSIDate(candidate); ok {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func startOfSaigonDay(value time.Time) time.Time {
	local := value.In(saigonLocation)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, saigonLocation)
}

func (p *SSIDividendProvider) fetchPage(ctx context.Context, symbol string, from, to time.Time, page int) (ssiDividendResponse, error) {
	query := url.Values{}
	query.Set("symbol", symbol)
	query.Set("fromDate", from.Format("02/01/2006"))
	query.Set("toDate", to.Format("02/01/2006"))
	query.Set("page", strconv.Itoa(page))
	query.Set("pageSize", strconv.Itoa(ssiDividendPageSize))
	req, err := newSSIRequest(ctx, http.MethodGet, p.endpoint()+"?"+query.Encode(), nil)
	if err != nil {
		return ssiDividendResponse{}, err
	}
	resp, err := p.httpClient().Do(req)
	if err != nil {
		return ssiDividendResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, ssiErrorBodyLimit))
		return ssiDividendResponse{}, fmt.Errorf("SSI status %d body %q", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	var body ssiDividendResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return ssiDividendResponse{}, fmt.Errorf("decode SSI response: %w", err)
	}
	if !strings.EqualFold(body.Code, "SUCCESS") || !strings.EqualFold(body.Status, "ok") {
		return ssiDividendResponse{}, fmt.Errorf("SSI unsuccessful response code=%q status=%q", body.Code, body.Status)
	}
	if body.Paging.Page != 0 && body.Paging.Page != page {
		return ssiDividendResponse{}, fmt.Errorf("SSI returned page %d for request page %d", body.Paging.Page, page)
	}
	return body, nil
}

func (p *SSIDividendProvider) normalizeEvent(raw ssiDividendEvent, requestedSymbol string) (DividendEvent, bool) {
	symbol := strings.ToUpper(strings.TrimSpace(raw.Symbol))
	if symbol != requestedSymbol {
		return DividendEvent{}, false
	}
	publishedAt, exDate, recordDate, paymentDate, ok := parseSSIDividendDates(raw)
	if !ok {
		return DividendEvent{}, false
	}
	event := DividendEvent{
		ProviderID: strings.TrimSpace(raw.CorID), Symbol: symbol,
		PublishedAt: publishedAt, ExDate: exDate, RecordDate: recordDate, PaymentDate: paymentDate,
		Title: strings.TrimSpace(raw.EventTitle), SourceURL: p.eventSourceURL(symbol, raw.CorID),
	}
	switch strings.ToUpper(strings.TrimSpace(raw.EventListCode)) {
	case "DIV":
		value, ok := positiveWholeNumber(string(raw.Value))
		if !ok {
			return DividendEvent{}, false
		}
		event.Kind = DividendKindCash
		event.VNDPerShare = value
	case "ISS":
		wording := strings.ToLower(strings.Join([]string{raw.EventName, raw.EventTitle, raw.EventDescription}, " "))
		if !strings.Contains(wording, "cổ tức") && !strings.Contains(wording, "dividend") {
			return DividendEvent{}, false
		}
		owned, newShares, ok := exactShareRatio(string(raw.Ratio))
		if !ok {
			return DividendEvent{}, false
		}
		event.Kind = DividendKindShares
		event.OwnedShares, event.NewShares = owned, newShares
	default:
		return DividendEvent{}, false
	}
	return event, true
}

// SSI normally supplies publicDate, the correct cursor timestamp. Older rows
// can omit it, so ex-right, record, then issue/payment date are used as a
// deterministic fallback. Any supplied but malformed date invalidates the row.
func parseSSIDividendDates(raw ssiDividendEvent) (publishedAt, exDate, recordDate, paymentDate time.Time, ok bool) {
	var valid bool
	if exDate, valid = parseSSIOptionalDate(raw.ExrightDate); !valid {
		return time.Time{}, time.Time{}, time.Time{}, time.Time{}, false
	}
	if recordDate, valid = parseSSIOptionalDate(raw.RecordDate); !valid {
		return time.Time{}, time.Time{}, time.Time{}, time.Time{}, false
	}
	if paymentDate, valid = parseSSIOptionalDate(raw.IssueDate); !valid {
		return time.Time{}, time.Time{}, time.Time{}, time.Time{}, false
	}
	if strings.TrimSpace(raw.PublicDate) != "" {
		publishedAt, valid = parseSSIDate(raw.PublicDate)
		if !valid {
			return time.Time{}, time.Time{}, time.Time{}, time.Time{}, false
		}
	} else {
		for _, fallback := range []time.Time{exDate, recordDate, paymentDate} {
			if !fallback.IsZero() {
				publishedAt = fallback
				break
			}
		}
	}
	return publishedAt, exDate, recordDate, paymentDate, !publishedAt.IsZero()
}

func parseSSIOptionalDate(value string) (time.Time, bool) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, true
	}
	return parseSSIDate(value)
}

func parseSSIDate(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"02/01/2006 15:04:05", "02/01/2006", "2006-01-02T15:04:05"} {
		parsed, err := time.ParseInLocation(layout, value, saigonLocation)
		if err == nil {
			return parsed, true
		}
	}
	parsed, err := time.Parse(time.RFC3339, value)
	return parsed, err == nil
}

func positiveWholeNumber(value string) (int64, bool) {
	ratio, ok := new(big.Rat).SetString(strings.TrimSpace(value))
	if !ok || ratio.Sign() <= 0 || !ratio.IsInt() || !ratio.Num().IsInt64() {
		return 0, false
	}
	return ratio.Num().Int64(), true
}

func exactShareRatio(value string) (owned, newShares int64, ok bool) {
	ratio, parsed := new(big.Rat).SetString(strings.TrimSpace(value))
	if !parsed || ratio.Sign() <= 0 || !ratio.Num().IsInt64() || !ratio.Denom().IsInt64() {
		return 0, 0, false
	}
	return ratio.Denom().Int64(), ratio.Num().Int64(), true
}

func (p *SSIDividendProvider) eventSourceURL(symbol, providerID string) string {
	query := url.Values{}
	query.Set("symbol", symbol)
	query.Set("corId", strings.TrimSpace(providerID))
	return p.endpoint() + "?" + query.Encode()
}
