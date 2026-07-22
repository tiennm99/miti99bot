package stock

import "time"

const (
	dividendDiscoveryWindow = 30 * 24 * time.Hour
	dividendRetentionDays   = 90
)

func (r DividendRecord) valid() bool {
	if r.PublishedAt <= 0 || r.ExDate < 0 || r.RecordDate < 0 || r.PaymentDate < 0 {
		return false
	}
	switch r.Kind {
	case DividendKindCash:
		return r.VNDPerShare > 0 && r.OwnedShares == 0 && r.NewShares == 0
	case DividendKindShares:
		return r.VNDPerShare == 0 && r.OwnedShares > 0 && r.NewShares > 0
	default:
		return false
	}
}

func dividendRecordFromEvent(event DividendEvent) DividendRecord {
	return DividendRecord{
		Kind:        event.Kind,
		PublishedAt: timeMillis(event.PublishedAt),
		ExDate:      timeMillis(event.ExDate),
		RecordDate:  timeMillis(event.RecordDate),
		PaymentDate: timeMillis(event.PaymentDate),
		VNDPerShare: event.VNDPerShare,
		OwnedShares: event.OwnedShares,
		NewShares:   event.NewShares,
		Title:       event.Title,
		SourceURL:   event.SourceURL,
	}
}

func (r DividendRecord) event(symbol, providerID string) DividendEvent {
	return DividendEvent{
		ProviderID:  providerID,
		Symbol:      symbol,
		Kind:        r.Kind,
		PublishedAt: millisTime(r.PublishedAt),
		ExDate:      millisTime(r.ExDate),
		RecordDate:  millisTime(r.RecordDate),
		PaymentDate: millisTime(r.PaymentDate),
		VNDPerShare: r.VNDPerShare,
		OwnedShares: r.OwnedShares,
		NewShares:   r.NewShares,
		Title:       r.Title,
		SourceURL:   r.SourceURL,
	}
}

func timeMillis(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.UnixMilli()
}

func millisTime(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(value).In(saigonLocation)
}

func (r DividendRecord) retentionAnchor() int64 {
	if r.RecordDate != 0 {
		return r.RecordDate
	}
	return r.PublishedAt
}

func (p Portfolio) dividendRecord(symbol, eventID string) (DividendRecord, bool) {
	events := p.Dividends[symbol]
	if events == nil {
		return DividendRecord{}, false
	}
	record, exists := events[eventID]
	return record, exists
}

func (p *Portfolio) setDividendRecord(symbol, eventID string, record DividendRecord) {
	if p.Dividends == nil {
		p.Dividends = map[string]map[string]DividendRecord{}
	}
	events := p.Dividends[symbol]
	if events == nil {
		events = map[string]DividendRecord{}
		p.Dividends[symbol] = events
	}
	events[eventID] = record
}

func (p *Portfolio) upsertDividendEvent(event DividendEvent) bool {
	old, exists := p.dividendRecord(event.Symbol, event.ProviderID)
	updated := dividendRecordFromEvent(event)
	updated.Processed = old.Processed
	if exists && old == updated {
		return false
	}
	p.setDividendRecord(event.Symbol, event.ProviderID, updated)
	return true
}

func (p *Portfolio) pruneDividendHistory(now time.Time) bool {
	changed := false
	for symbol, events := range p.Dividends {
		for eventID, event := range events {
			if dividendRecordExpired(event, now) {
				delete(events, eventID)
				changed = true
			}
		}
		if len(events) == 0 {
			delete(p.Dividends, symbol)
		}
	}
	return changed
}

func dividendRecordExpired(event DividendRecord, now time.Time) bool {
	anchor := event.retentionAnchor()
	return anchor > 0 && !startOfSaigonDay(now).Before(startOfSaigonDay(millisTime(anchor)).AddDate(0, 0, dividendRetentionDays))
}

func dividendRecordDue(event DividendRecord, now time.Time) bool {
	return event.RecordDate > 0 && !startOfSaigonDay(now).Before(startOfSaigonDay(millisTime(event.RecordDate)))
}

func positionOpenedByRecordDate(position AssetPosition, event DividendRecord) bool {
	if event.RecordDate <= 0 || position.OpenedAt < 0 {
		return false
	}
	// OpenedAt was added after portfolios already existed. Zero means the
	// lifecycle predates tracking, so preserve eligibility for legacy holdings.
	if position.OpenedAt == 0 {
		return true
	}
	openedDay := startOfSaigonDay(millisTime(position.OpenedAt))
	recordDay := startOfSaigonDay(millisTime(event.RecordDate))
	return !openedDay.After(recordDay)
}
