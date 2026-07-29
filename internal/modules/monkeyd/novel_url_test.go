package monkeyd

import "testing"

func TestNormalizeNovelURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{
			name: "https url passes through",
			raw:  "https://monkeydd.com/tro-lai-nam-thang-cu.html",
			want: "https://monkeydd.com/tro-lai-nam-thang-cu.html",
		},
		{
			name: "www host is allowed",
			raw:  "https://www.monkeydd.com/tro-lai-nam-thang-cu.html",
			want: "https://www.monkeydd.com/tro-lai-nam-thang-cu.html",
		},
		{
			name: "missing scheme gets https",
			raw:  "monkeydd.com/tro-lai-nam-thang-cu.html",
			want: "https://monkeydd.com/tro-lai-nam-thang-cu.html",
		},
		{
			name: "http is kept",
			raw:  "http://monkeydd.com/tro-lai-nam-thang-cu.html",
			want: "http://monkeydd.com/tro-lai-nam-thang-cu.html",
		},
		{
			name: "surrounding whitespace is trimmed",
			raw:  "  https://monkeydd.com/tro-lai-nam-thang-cu.html  ",
			want: "https://monkeydd.com/tro-lai-nam-thang-cu.html",
		},
		{
			name: "uppercase host is canonicalised",
			raw:  "https://MonkeyDD.com/tro-lai-nam-thang-cu.html",
			want: "https://monkeydd.com/tro-lai-nam-thang-cu.html",
		},
		{
			name: "fragment is dropped",
			raw:  "https://monkeydd.com/tro-lai-nam-thang-cu.html#chuong-1",
			want: "https://monkeydd.com/tro-lai-nam-thang-cu.html",
		},
		{
			name: "query is preserved",
			raw:  "https://monkeydd.com/novel.html?page=2",
			want: "https://monkeydd.com/novel.html?page=2",
		},
		{
			name:    "empty input",
			raw:     "",
			wantErr: true,
		},
		{
			name:    "other host",
			raw:     "https://example.com/novel.html",
			wantErr: true,
		},
		{
			name:    "host that merely ends with the allowed name",
			raw:     "https://evilmonkeydd.com/novel.html",
			wantErr: true,
		},
		{
			name:    "host that merely contains the allowed name",
			raw:     "https://monkeydd.com.evil.example/novel.html",
			wantErr: true,
		},
		{
			name:    "allowed host as a subdomain of another host",
			raw:     "https://monkeydd.com.attacker.test/novel.html",
			wantErr: true,
		},
		{
			name:    "non-http scheme",
			raw:     "file:///etc/passwd",
			wantErr: true,
		},
		{
			name:    "credentials in url",
			raw:     "https://user:pass@monkeydd.com/novel.html",
			wantErr: true,
		},
		{
			name:    "landing page with no novel path",
			raw:     "https://monkeydd.com",
			wantErr: true,
		},
		{
			name:    "root path only",
			raw:     "https://monkeydd.com/",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeNovelURL(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("normalizeNovelURL(%q) = %q, want error", tt.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeNovelURL(%q) returned error: %v", tt.raw, err)
			}
			if got != tt.want {
				t.Errorf("normalizeNovelURL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

// A port on an allowed host must not defeat the allowlist, and must survive
// canonicalisation so the crawl reaches the same place the user asked for.
func TestNormalizeNovelURLKeepsPort(t *testing.T) {
	got, err := normalizeNovelURL("http://monkeydd.com:8080/novel.html")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "http://monkeydd.com:8080/novel.html"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
