package monkeyd

import (
	"strings"
	"testing"
)

func TestParseCrawlArgs(t *testing.T) {
	tests := []struct {
		name         string
		arg          string
		wantURL      string
		wantFontSize float64
		wantErr      bool
	}{
		{
			name:         "url only leaves the font size unset",
			arg:          testNovelURL,
			wantURL:      testNovelURL,
			wantFontSize: 0,
		},
		{
			name:         "url with font size",
			arg:          testNovelURL + " 14",
			wantURL:      testNovelURL,
			wantFontSize: 14,
		},
		{
			name:         "half point font size",
			arg:          testNovelURL + " 10.5",
			wantURL:      testNovelURL,
			wantFontSize: 10.5,
		},
		{
			name:         "extra whitespace between arguments",
			arg:          "  " + testNovelURL + "    12  ",
			wantURL:      testNovelURL,
			wantFontSize: 12,
		},
		{
			name:         "bare host gets a scheme",
			arg:          "monkeydd.com/novel.html 9",
			wantURL:      "https://monkeydd.com/novel.html",
			wantFontSize: 9,
		},
		{
			name:         "font size at the lower bound",
			arg:          testNovelURL + " 6",
			wantURL:      testNovelURL,
			wantFontSize: minFontSize,
		},
		{
			name:         "font size at the upper bound",
			arg:          testNovelURL + " 24",
			wantURL:      testNovelURL,
			wantFontSize: maxFontSize,
		},
		{
			name:    "empty argument",
			arg:     "",
			wantErr: true,
		},
		{
			name:    "font size below the lower bound",
			arg:     testNovelURL + " 5",
			wantErr: true,
		},
		{
			name:    "font size above the upper bound",
			arg:     testNovelURL + " 25",
			wantErr: true,
		},
		{
			name:    "zero font size",
			arg:     testNovelURL + " 0",
			wantErr: true,
		},
		{
			name:    "negative font size",
			arg:     testNovelURL + " -12",
			wantErr: true,
		},
		{
			name:    "non-numeric font size",
			arg:     testNovelURL + " big",
			wantErr: true,
		},
		{
			name:    "NaN font size",
			arg:     testNovelURL + " NaN",
			wantErr: true,
		},
		{
			name:    "infinite font size",
			arg:     testNovelURL + " Inf",
			wantErr: true,
		},
		{
			name:    "too many arguments",
			arg:     testNovelURL + " 12 a5",
			wantErr: true,
		},
		{
			name:    "disallowed host",
			arg:     "https://example.com/novel.html 12",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCrawlArgs(tt.arg)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseCrawlArgs(%q) = %+v, want error", tt.arg, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseCrawlArgs(%q) returned error: %v", tt.arg, err)
			}
			if got.NovelURL != tt.wantURL {
				t.Errorf("NovelURL = %q, want %q", got.NovelURL, tt.wantURL)
			}
			if got.FontSize != tt.wantFontSize {
				t.Errorf("FontSize = %v, want %v", got.FontSize, tt.wantFontSize)
			}
		})
	}
}

// The out-of-range message must state the bounds; "invalid font size" alone
// leaves the user guessing what to type instead.
func TestParseFontSizeErrorNamesTheBounds(t *testing.T) {
	_, err := parseFontSize("99")
	if err == nil {
		t.Fatal("expected an error for 99")
	}
	for _, want := range []string{"6", "24"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention the bound %q", err, want)
		}
	}
}
