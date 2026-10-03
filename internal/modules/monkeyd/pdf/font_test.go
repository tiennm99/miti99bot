package pdf

import (
	"testing"

	"golang.org/x/image/font/sfnt"
)

func TestBundledFontIsParseable(t *testing.T) {
	font := BundledFont()
	if font.Name != BundledFontName {
		t.Errorf("Name = %q, want %q", font.Name, BundledFontName)
	}
	if _, err := sfnt.Parse(font.Data); err != nil {
		t.Fatalf("bundled font does not parse as a TrueType font: %v", err)
	}
}

// The bundled font exists to render Vietnamese. A replacement that lacks these
// glyphs would silently emit blanks, so check before trusting it.
func TestBundledFontCoversVietnamese(t *testing.T) {
	parsed, err := sfnt.Parse(BundledFont().Data)
	if err != nil {
		t.Fatalf("parse bundled font: %v", err)
	}
	var buf sfnt.Buffer
	for _, r := range []rune{'ư', 'ơ', 'đ', 'ạ', 'ế', 'ộ', 'ữ', 'ằ', 'ỷ', 'ỹ', 'Ọ', 'Ế', 'Ư', 'Đ'} {
		index, err := parsed.GlyphIndex(&buf, r)
		if err != nil {
			t.Errorf("GlyphIndex(%q): %v", r, err)
			continue
		}
		// Glyph 0 is .notdef — the character is absent from the font.
		if index == 0 {
			t.Errorf("bundled font has no glyph for %q (U+%04X)", r, r)
		}
	}
}
