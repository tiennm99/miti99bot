package stock

import (
	"testing"
	"time"
)

func TestUpsertDividendEventPreservesLocalStateAndProviderCorrections(t *testing.T) {
	p := NewPortfolio(1)
	p.Dividends["TCB"] = map[string]DividendRecord{
		"2612974": {
			Kind: DividendKindCash, PublishedAt: 1, VNDPerShare: 1000,
			Processed: true,
		},
	}
	event := DividendEvent{
		ProviderID: "2612974", Symbol: "TCB", Kind: DividendKindCash,
		PublishedAt: time.UnixMilli(2), RecordDate: time.UnixMilli(3), VNDPerShare: 1500,
	}
	if !p.upsertDividendEvent(event) {
		t.Fatal("provider correction was not recorded")
	}
	got := p.Dividends["TCB"]["2612974"]
	if got.VNDPerShare != 1500 || got.RecordDate != 3 || !got.Processed {
		t.Fatalf("merged event = %+v", got)
	}
}

func TestPruneDividendHistoryUsesRecordDateThenPublishedDate(t *testing.T) {
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, saigonLocation)
	p := NewPortfolio(1)
	p.Dividends["TCB"] = map[string]DividendRecord{
		"record-expired": {
			Kind: DividendKindCash, PublishedAt: now.AddDate(0, 0, -120).UnixMilli(),
			RecordDate: now.AddDate(0, 0, -90).UnixMilli(), VNDPerShare: 1000,
		},
		"missing-expired": {
			Kind: DividendKindCash, PublishedAt: now.AddDate(0, 0, -90).UnixMilli(), VNDPerShare: 1000,
		},
		"keep": {
			Kind: DividendKindCash, PublishedAt: now.AddDate(0, 0, -100).UnixMilli(),
			RecordDate: now.AddDate(0, 0, -89).UnixMilli(), VNDPerShare: 1000,
		},
	}
	if !p.pruneDividendHistory(now) {
		t.Fatal("prune reported no change")
	}
	if len(p.Dividends["TCB"]) != 1 || p.Dividends["TCB"]["keep"].RecordDate == 0 {
		t.Fatalf("retained history = %+v", p.Dividends)
	}
}

func TestPositionOpenedByRecordDateUsesSaigonCalendarDay(t *testing.T) {
	recordDate := time.Date(2026, 6, 25, 0, 0, 0, 0, saigonLocation)
	event := DividendRecord{RecordDate: recordDate.UnixMilli()}
	if !positionOpenedByRecordDate(AssetPosition{}, event) {
		t.Fatal("legacy position without openedAt should remain eligible")
	}
	if !positionOpenedByRecordDate(AssetPosition{OpenedAt: recordDate.Add(12 * time.Hour).UnixMilli()}, event) {
		t.Fatal("position opened on record date should be eligible")
	}
	if positionOpenedByRecordDate(AssetPosition{OpenedAt: recordDate.AddDate(0, 0, 1).UnixMilli()}, event) {
		t.Fatal("position opened after record date should be ineligible")
	}
}
