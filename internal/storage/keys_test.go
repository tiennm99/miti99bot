package storage

import "testing"

func TestPrefixSuccessor(t *testing.T) {
	cases := map[string]string{
		"abc":   "abd",
		"a":     "b",
		"":      "",
		"\xff":  "\xff", // all-0xFF: degenerates
		"a\xff": "b",    // strip trailing 0xFF, increment
	}
	for in, want := range cases {
		if got := prefixSuccessor(in); got != want {
			t.Errorf("prefixSuccessor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateKey(t *testing.T) {
	valid := []string{"user:1", "a", "config:daily", "u1_b-2", "vnappmob:api_key"}
	for _, k := range valid {
		if err := validateKey(k); err != nil {
			t.Errorf("validateKey(%q) = %v, want nil", k, err)
		}
	}
	invalid := []string{"", "path/sep", ".", "..", "__reserved__"}
	for _, k := range invalid {
		if err := validateKey(k); err == nil {
			t.Errorf("validateKey(%q) = nil, want error", k)
		}
	}
	// validatePrefix permits empty (whole-collection scan).
	if err := validatePrefix(""); err != nil {
		t.Errorf("validatePrefix(\"\") = %v, want nil", err)
	}
	if err := validatePrefix("bad/prefix"); err == nil {
		t.Error("validatePrefix(\"bad/prefix\") = nil, want error")
	}
}
