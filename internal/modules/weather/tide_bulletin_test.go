package weather

import (
	"os"
	"testing"
	"time"
)

func loadBulletinFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/HCMC_TVHN_20261002.pdf")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestParseTideBulletin_RealPDF(t *testing.T) {
	issued := time.Date(2026, 10, 2, 0, 0, 0, 0, ictZone)
	b, err := parseTideBulletin(loadBulletinFixture(t), issued)
	if err != nil {
		t.Fatalf("parseTideBulletin: %v", err)
	}
	if len(b.Stations) != len(bulletinStations) {
		t.Fatalf("stations = %d, want %d", len(b.Stations), len(bulletinStations))
	}
	phuAn, nhaBe := b.Stations[0], b.Stations[1]
	if got := phuAn[0].Date; !got.Equal(issued) {
		t.Errorf("first date = %v, want %v", got, issued)
	}
	want := []tidePeak{{1.33, "07:00"}, {1.29, "21:00"}}
	if len(phuAn[0].Peaks) != 2 || phuAn[0].Peaks[0] != want[0] || phuAn[0].Peaks[1] != want[1] {
		t.Errorf("Phú An 02/10 = %+v, want %+v", phuAn[0].Peaks, want)
	}
	// 05/10 has no second high tide ("ct").
	if got := phuAn[3].Peaks; len(got) != 1 || got[0] != (tidePeak{0.84, "10:00"}) {
		t.Errorf("Phú An 05/10 = %+v, want one peak 0.84 m at 10:00", got)
	}
	// The middle row's date tokens come out swapped ("10 04/").
	if got := nhaBe[2]; !got.Date.Equal(issued.AddDate(0, 0, 2)) || got.Peaks[0] != (tidePeak{1.02, "08:00"}) {
		t.Errorf("Nhà Bè 04/10 = %+v", got)
	}
	if p, _ := b.Stations[2][0].maxPeak(); p.Height != 1.60 {
		t.Errorf("Thủ Dầu Một 02/10 max = %v, want 1.60", p.Height)
	}
}

// In the 10/09 bulletin the Thủ Dầu Một label is a text row of its own, one
// point from the block's middle row, and the week is at alarm level.
func TestParseTideBulletin_SeparateLabelRow(t *testing.T) {
	data, err := os.ReadFile("testdata/HCMC_TVHN_20260910.pdf")
	if err != nil {
		t.Fatal(err)
	}
	b, err := parseTideBulletin(data, time.Date(2026, 9, 10, 0, 0, 0, 0, ictZone))
	if err != nil {
		t.Fatalf("parseTideBulletin: %v", err)
	}
	if p, _ := b.Stations[1][2].maxPeak(); p != (tidePeak{1.46, "16:00"}) {
		t.Errorf("Nhà Bè 12/09 max = %+v, want 1.46 m at 16:00", p)
	}
	if p, _ := b.Stations[2][2].maxPeak(); p != (tidePeak{1.55, "17:30"}) {
		t.Errorf("Thủ Dầu Một 12/09 max = %+v, want 1.55 m at 17:30", p)
	}
}

func TestParseTideBulletin_RejectsNonBulletin(t *testing.T) {
	if _, err := parseTideBulletin([]byte("not a pdf"), time.Now()); err == nil || err.Error() != "not a PDF" {
		t.Error("want an error for non-PDF input")
	}
}

func TestParseBulletinRow(t *testing.T) {
	issued := time.Date(2026, 12, 30, 0, 0, 0, 0, ictZone)
	row, ok := parseBulletinRow([]string{"P", "n", "A", "ú", "h", "1", "02/", "1.01", "09.00", "ct", "ct", "1.78", "-"}, issued)
	if !ok {
		t.Fatal("row not parsed")
	}
	if want := time.Date(2027, 1, 2, 0, 0, 0, 0, ictZone); !row.day.Date.Equal(want) {
		t.Errorf("date = %v, want %v (year rollover)", row.day.Date, want)
	}
	if !sameLetters(row.label, "Phú An") {
		t.Errorf("label %q should match Phú An", row.label)
	}
	if len(row.day.Peaks) != 1 || row.day.Peaks[0] != (tidePeak{1.01, "09:00"}) {
		t.Errorf("peaks = %+v, want one at 09:00", row.day.Peaks)
	}
	// A one-digit hour ("1.30") is padded.
	if row, ok := parseBulletinRow([]string{"06/", "10", "0.68", "11.00", "1.13", "1.30"}, issued); !ok || row.day.Peaks[1].Time != "01:30" {
		t.Errorf("one-digit hour: %+v ok=%v, want 01:30", row.day.Peaks, ok)
	}
	for _, tokens := range [][]string{
		{"đ", "ế", "n", "7h", "02/", "10"},              // header, no values
		{"02/", "10", "61.56", "1538", "776", "256.0"},  // reservoir row, no times
		{"MỰC", "NGÀY", "01/", "10/", "2026"},           // date without a month token
		{"02/", "10", "1.33", "25.00", "1.29", "21.00"}, // hour out of range
	} {
		if _, ok := parseBulletinRow(tokens, issued); ok {
			t.Errorf("parseBulletinRow(%q) should be rejected", tokens)
		}
	}
}

func TestNewestArticleAndBulletinLink(t *testing.T) {
	home := []byte(`<a href="/index.php/100-thong-tin-kttv/thuy-van/29180-ba-n-tin-da-ba-o-tha-y-v-n-tphcm-ra-nga-y-01-10-2026">
<a href="/index.php/100-thong-tin-kttv/thuy-van/29194-ba-n-tin-da-ba-o-tha-y-v-n-tphcm-ra-nga-y-02-10-2026" itemprop="url">
<a href="/index.php/100-thong-tin-kttv/thuy-van/29196-ba-n-tin-da-ba-o-tha-y-v-n-pha-c-va-quy-tra-nh-ra-nga-y-02-10-2026">`)
	got, err := newestArticle(home)
	if err != nil {
		t.Fatal(err)
	}
	if want := kttvnbURL + "/index.php/100-thong-tin-kttv/thuy-van/29194-ba-n-tin-da-ba-o-tha-y-v-n-tphcm-ra-nga-y-02-10-2026"; got != want {
		t.Errorf("newestArticle = %q, want %q", got, want)
	}
	if _, err := newestArticle([]byte("<html></html>")); err == nil {
		t.Error("want an error when no article is linked")
	}

	article := []byte(`<a class="at_icon" href="https://kttvnb.vn/attachments/article/29194/HCMC_TVHN_20261002.pdf" title="x">`)
	pdfURL, issued, err := bulletinLink(article)
	if err != nil {
		t.Fatal(err)
	}
	if pdfURL != "https://kttvnb.vn/attachments/article/29194/HCMC_TVHN_20261002.pdf" {
		t.Errorf("pdfURL = %q", pdfURL)
	}
	if !issued.Equal(time.Date(2026, 10, 2, 0, 0, 0, 0, ictZone)) {
		t.Errorf("issued = %v", issued)
	}
}
