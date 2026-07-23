package stock

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestSSIDividendProviderPaginatesDeduplicatesFiltersAndNormalizes(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	requests := make([]url.Values, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != ssiDividendPath {
			t.Fatalf("path = %q, want %q", r.URL.Path, ssiDividendPath)
		}
		mu.Lock()
		requests = append(requests, r.URL.Query())
		mu.Unlock()
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		w.Header().Set("Content-Type", "application/json")
		switch page {
		case 1:
			_, _ = fmt.Fprint(w, `{"code":"SUCCESS","status":"ok","paging":{"totalPage":2,"page":1},"data":[
				{"CorId":"cash","symbol":"TCB","eventListCode":"DIV","eventTitle":"Cash dividend","publicDate":"11/06/2026 00:00:00","exrightDate":"15/06/2026","recordDate":"16/06/2026","issueDate":"30/06/2026","value":"1500.0","ratio":"0"},
				{"CorId":"not-dividend","symbol":"TCB","eventListCode":"ISS","eventTitle":"Rights offering","publicDate":"11/06/2026 00:00:00","ratio":"0.1"},
				{"CorId":"wrong-symbol","symbol":"ACB","eventListCode":"DIV","eventTitle":"Cash dividend","publicDate":"11/06/2026 00:00:00","value":"500"}
			]}`)
		case 2:
			_, _ = fmt.Fprint(w, `{"code":"SUCCESS","status":"ok","paging":{"totalPage":2,"page":2},"data":[
				{"CorId":"cash","symbol":"TCB","eventListCode":"DIV","eventTitle":"duplicate","publicDate":"11/06/2026 00:00:00","value":"999"},
				{"CorId":"shares","symbol":"TCB","eventListCode":"ISS","eventName":"Trả cổ tức bằng cổ phiếu","eventTitle":"Share dividend","publicDate":"12/06/2026 12:00:00","exrightDate":"20/06/2026","ratio":0.125},
				{"CorId":"invalid","symbol":"TCB","eventListCode":"DIV","eventTitle":"bad value","publicDate":"11/06/2026 00:00:00","value":"1.5"}
			]}`)
		default:
			t.Fatalf("unexpected page %d", page)
		}
	}))
	defer server.Close()

	provider := &SSIDividendProvider{HTTP: server.Client(), BaseURL: server.URL}
	after := saigonTime(t, "10/06/2026 12:00:00")
	through := saigonTime(t, "12/06/2026 12:00:00")
	events, err := provider.FetchDividendEvents(context.Background(), " tcb ", after, through)
	if err != nil {
		t.Fatalf("FetchDividendEvents: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %+v, want cash and shares", events)
	}
	if cash := events[0]; cash.ProviderID != "cash" || cash.Kind != DividendKindCash || cash.VNDPerShare != 1500 || cash.Symbol != "TCB" {
		t.Fatalf("cash = %+v", cash)
	}
	if shares := events[1]; shares.ProviderID != "shares" || shares.Kind != DividendKindShares || shares.OwnedShares != 8 || shares.NewShares != 1 {
		t.Fatalf("shares = %+v", shares)
	}
	if events[1].PublishedAt != through {
		t.Fatalf("through boundary event time = %v, want %v", events[1].PublishedAt, through)
	}
	if events[0].SourceURL == "" || events[0].Title != "Cash dividend" {
		t.Fatalf("display metadata missing: %+v", events[0])
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(requests))
	}
	for i, query := range requests {
		if query.Get("symbol") != "TCB" || query.Get("fromDate") != "09/06/2026" || query.Get("toDate") != "12/06/2026" || query.Get("pageSize") != "50" || query.Get("page") != strconv.Itoa(i+1) {
			t.Errorf("request %d query = %v", i+1, query)
		}
	}
}

func TestSSIStockEventProviderIncludesAllTypesPaginatesDeduplicatesFiltersAndOrders(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		w.Header().Set("Content-Type", "application/json")
		switch page {
		case 1:
			_, _ = fmt.Fprint(w, `{"code":"SUCCESS","status":"ok","paging":{"totalPage":2,"page":1},"data":[
				{"CorId":"meeting","symbol":"TCB","eventListCode":"AGM","eventName":"Annual meeting","eventTitle":"Meeting title","eventDescription":"Meeting description","publicDate":"11/06/2026 12:00:00","recordDate":"malformed-raw-date","value":"123.45","ratio":"0.25"},
				{"CorId":"tie-b","symbol":"TCB","eventListCode":"OTHER","eventTitle":"Second by ID","publicDate":"12/06/2026 00:00:00"},
				{"CorId":"at-after","symbol":"TCB","eventListCode":"OTHER","publicDate":"10/06/2026 12:00:00"},
				{"CorId":"overlap-only","symbol":"TCB","eventListCode":"OTHER","publicDate":"10/06/2026 00:00:00"},
				{"CorId":"too-old","symbol":"TCB","eventListCode":"OTHER","publicDate":"08/06/2026 23:59:59"},
				{"CorId":"wrong-symbol","symbol":"ACB","eventListCode":"AGM","publicDate":"11/06/2026"}
			]}`)
		case 2:
			_, _ = fmt.Fprint(w, `{"code":"SUCCESS","status":"ok","paging":{"totalPage":2,"page":2},"data":[
				{"CorId":"meeting","symbol":"TCB","eventListCode":"AGM","eventTitle":"duplicate","publicDate":"11/06/2026 12:00:00"},
				{"CorId":"tie-a","symbol":"TCB","eventListCode":"ISS","eventName":"Rights issue","eventTitle":"First by ID","publicDate":"12/06/2026 00:00:00","exrightDate":"20/06/2026","issueDate":"30/06/2026"},
				{"CorId":"after-through","symbol":"TCB","eventListCode":"OTHER","publicDate":"12/06/2026 00:00:01"},
				{"CorId":"bad-date","symbol":"TCB","eventListCode":"OTHER","publicDate":"not-a-date","exrightDate":"11/06/2026"},
				{"CorId":"fallback-date","symbol":"TCB","eventListCode":"FALLBACK","eventDescription":"raw fallback event","exrightDate":"bad-optional","recordDate":"11/06/2026","issueDate":"also-bad"}
			]}`)
		default:
			t.Fatalf("unexpected page %d", page)
		}
	}))
	defer server.Close()

	provider := &SSIDividendProvider{HTTP: server.Client(), BaseURL: server.URL}
	events, err := provider.FetchStockEvents(context.Background(), " tcb ", saigonTime(t, "10/06/2026 12:00:00"), saigonTime(t, "12/06/2026 00:00:00"))
	if err != nil {
		t.Fatalf("FetchStockEvents: %v", err)
	}
	if len(events) != 4 {
		t.Fatalf("events = %+v, want fallback, meeting, and two tie events", events)
	}
	if events[0].CorID != "fallback-date" || events[0].PublicDate != "" || events[0].ExrightDate != "bad-optional" || events[0].RecordDate != "11/06/2026" {
		t.Fatalf("fallback cursor/raw fields = %+v", events[0])
	}
	if events[1].CorID != "meeting" || events[1].EventListCode != "AGM" || events[1].EventName != "Annual meeting" || events[1].EventTitle != "Meeting title" || events[1].EventDescription != "Meeting description" || events[1].RecordDate != "malformed-raw-date" || events[1].Value != "123.45" || events[1].Ratio != "0.25" {
		t.Fatalf("generic non-dividend raw event = %+v", events[1])
	}
	if events[2].CorID != "tie-a" || events[3].CorID != "tie-b" {
		t.Fatalf("tie order = %q, %q", events[2].CorID, events[3].CorID)
	}
	if events[2].ExrightDate != "20/06/2026" || events[2].IssueDate != "30/06/2026" || events[2].SourceURL == "" {
		t.Fatalf("raw event dates/source missing: %+v", events[2])
	}
	for _, event := range events {
		if event.CorID == "bad-date" {
			t.Fatalf("malformed publicDate was accepted via optional fallback: %+v", event)
		}
	}

	dividends, err := provider.FetchDividendEvents(context.Background(), "TCB", saigonTime(t, "10/06/2026 12:00:00"), saigonTime(t, "12/06/2026 00:00:00"))
	if err != nil {
		t.Fatalf("FetchDividendEvents: %v", err)
	}
	if len(dividends) != 0 {
		t.Fatalf("generic events leaked into dividend path: %+v", dividends)
	}
}

func TestSSIStockEventProviderRequiresEveryPageAndValidRange(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			http.Error(w, "upstream unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = fmt.Fprint(w, `{"code":"SUCCESS","status":"ok","paging":{"totalPage":2,"page":1},"data":[]}`)
	}))
	defer server.Close()
	provider := &SSIDividendProvider{HTTP: server.Client(), BaseURL: server.URL}
	now := saigonTime(t, "12/06/2026")
	if events, err := provider.FetchStockEvents(context.Background(), "TCB", now.Add(-24*time.Hour), now); err == nil || events != nil {
		t.Fatalf("partial events, err = %+v, %v", events, err)
	}
	if _, err := provider.FetchStockEvents(context.Background(), "TCB", now, now); err == nil {
		t.Fatal("equal range accepted")
	}
}

func TestSSIDividendProviderUsesDayOverlapAndDateFallback(t *testing.T) {
	t.Parallel()
	server := dividendTestServer(t, http.StatusOK, `{"code":"SUCCESS","status":"ok","paging":{"totalPage":1,"page":1},"data":[
		{"CorId":"too-old","symbol":"TCB","eventListCode":"DIV","publicDate":"08/06/2026 00:00:00","value":"100"},
		{"CorId":"same-day-midnight","symbol":"TCB","eventListCode":"DIV","publicDate":"10/06/2026 00:00:00","value":"100"},
		{"CorId":"at-after","symbol":"TCB","eventListCode":"DIV","publicDate":"10/06/2026 12:00:00","value":"100"},
		{"CorId":"fallback","symbol":"TCB","eventListCode":"ISS","eventTitle":"Dividend in shares","exrightDate":"11/06/2026","ratio":"0.13"},
		{"CorId":"after-through","symbol":"TCB","eventListCode":"DIV","publicDate":"13/06/2026 00:00:00","value":"100"},
		{"CorId":"bad-date","symbol":"TCB","eventListCode":"DIV","publicDate":"not-a-date","value":"100"}
	]}`)
	defer server.Close()
	provider := &SSIDividendProvider{HTTP: server.Client(), BaseURL: server.URL}
	events, err := provider.FetchDividendEvents(context.Background(), "TCB", saigonTime(t, "10/06/2026 12:00:00"), saigonTime(t, "12/06/2026 00:00:00"))
	if err != nil {
		t.Fatalf("FetchDividendEvents: %v", err)
	}
	if len(events) != 3 || events[0].ProviderID != "same-day-midnight" || events[1].ProviderID != "at-after" || events[2].ProviderID != "fallback" || events[2].PublishedAt != saigonTime(t, "11/06/2026 00:00:00") {
		t.Fatalf("events = %+v, want same-day overlap, cursor boundary, and fallback events", events)
	}
	if events[2].OwnedShares != 100 || events[2].NewShares != 13 {
		t.Fatalf("ratio = %d:%d, want 100:13", events[2].OwnedShares, events[2].NewShares)
	}
}

func TestSSIDividendProviderRequiresEveryPage(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			http.Error(w, "upstream unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = fmt.Fprint(w, `{"code":"SUCCESS","status":"ok","paging":{"totalPage":2,"page":1},"data":[{"CorId":"cash","symbol":"TCB","eventListCode":"DIV","publicDate":"11/06/2026","value":"100"}]}`)
	}))
	defer server.Close()
	provider := &SSIDividendProvider{HTTP: server.Client(), BaseURL: server.URL}
	events, err := provider.FetchDividendEvents(context.Background(), "TCB", saigonTime(t, "10/06/2026"), saigonTime(t, "12/06/2026"))
	if err == nil || events != nil {
		t.Fatalf("events, err = %+v, %v; want nil and page error", events, err)
	}
}

func TestSSIDividendProviderRejectsInvalidRangeAndExcessivePages(t *testing.T) {
	t.Parallel()
	provider := &SSIDividendProvider{}
	now := time.Now()
	if _, err := provider.FetchDividendEvents(context.Background(), "TCB", now, now); err == nil {
		t.Fatal("equal cursor range accepted")
	}

	server := dividendTestServer(t, http.StatusOK, `{"code":"SUCCESS","status":"ok","paging":{"totalPage":21,"page":1},"data":[]}`)
	defer server.Close()
	provider = &SSIDividendProvider{HTTP: server.Client(), BaseURL: server.URL}
	if _, err := provider.FetchDividendEvents(context.Background(), "TCB", saigonTime(t, "10/06/2026"), saigonTime(t, "12/06/2026")); err == nil {
		t.Fatal("response above page cap accepted")
	}
}

func dividendTestServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = fmt.Fprint(w, body)
	}))
}

func saigonTime(t *testing.T, value string) time.Time {
	t.Helper()
	for _, layout := range []string{"02/01/2006 15:04:05", "02/01/2006"} {
		parsed, err := time.ParseInLocation(layout, value, saigonLocation)
		if err == nil {
			return parsed
		}
	}
	t.Fatalf("parse test time %q", value)
	return time.Time{}
}
