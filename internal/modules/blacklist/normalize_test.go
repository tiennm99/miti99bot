package blacklist

import "testing"

func TestNormalize_ComposesVietnamese(t *testing.T) {
	// The same word as a client sending NFC and a client sending NFD. Without
	// composition these are different strings and would never match.
	nfc := "má"  // U+00E1 precomposed
	nfd := "má" // "a" + combining acute
	gotNFC, ok := Normalize(nfc)
	if !ok {
		t.Fatal("NFC form rejected")
	}
	gotNFD, ok := Normalize(nfd)
	if !ok {
		t.Fatal("NFD form rejected")
	}
	if gotNFC != gotNFD {
		t.Fatalf("NFC %q and NFD %q normalize differently", gotNFC, gotNFD)
	}
}

func TestNormalize_FoldsCaseButKeepsDiacritics(t *testing.T) {
	upper, _ := Normalize("MÁ")
	lower, _ := Normalize("má")
	if upper != lower {
		t.Fatalf("case not folded: %q vs %q", upper, lower)
	}

	bare, _ := Normalize("ma")
	if bare == lower {
		t.Fatalf("diacritic was stripped: %q == %q", bare, lower)
	}
}

func TestNormalize_FoldsCompatibilityVariants(t *testing.T) {
	full, _ := Normalize("ＡＢ") // full-width AB
	plain, _ := Normalize("ab")
	if full != plain {
		t.Fatalf("full-width %q did not fold to %q", full, plain)
	}
}

func TestNormalize_CollapsesWhitespace(t *testing.T) {
	got, ok := Normalize("  cat \n  dog  ")
	if !ok || got != "cat dog" {
		t.Fatalf("Normalize = %q, %v; want \"cat dog\", true", got, ok)
	}
}

func TestNormalize_RejectsEmpty(t *testing.T) {
	for _, in := range []string{"", "   ", "\n\t "} {
		if got, ok := Normalize(in); ok {
			t.Fatalf("Normalize(%q) = %q, true; want not ok", in, got)
		}
	}
}
