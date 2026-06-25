package stock

import (
	"errors"
	"testing"
)

func TestNormalizeStockSymbol(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "uppercases", input: "tcb", want: "TCB"},
		{name: "trims", input: "  fpt  ", want: "FPT"},
		{name: "allows digits", input: "abc123", want: "ABC123"},
		{name: "allows sixteen chars", input: "abcdefghijklmnop", want: "ABCDEFGHIJKLMNOP"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeStockSymbol(tt.input)
			if err != nil {
				t.Fatalf("normalizeStockSymbol(%q): %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("normalizeStockSymbol(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeStockSymbolRejectsInvalid(t *testing.T) {
	for _, input := range []string{
		"",
		"  ",
		"FPT.VN",
		"FPT-VN",
		"đxg",
		"abcdefghijklmnopq",
	} {
		t.Run(input, func(t *testing.T) {
			_, err := normalizeStockSymbol(input)
			if !errors.Is(err, ErrUnknownTicker) {
				t.Fatalf("normalizeStockSymbol(%q) error = %v, want ErrUnknownTicker", input, err)
			}
		})
	}
}
