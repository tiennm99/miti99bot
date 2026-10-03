package export

import "testing"

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		req     Request
		wantErr bool
	}{
		{
			name: "url and font size are valid",
			req:  Request{NovelURL: "https://monkeydd.com/example.html", FontSize: DefaultFontSize},
		},
		{
			name:    "missing url",
			req:     Request{FontSize: DefaultFontSize},
			wantErr: true,
		},
		{
			name:    "non-http scheme",
			req:     Request{NovelURL: "ftp://monkeydd.com/example.html", FontSize: DefaultFontSize},
			wantErr: true,
		},
		{
			name:    "scheme-less url",
			req:     Request{NovelURL: "monkeydd.com/example.html", FontSize: DefaultFontSize},
			wantErr: true,
		},
		{
			name:    "negative font size",
			req:     Request{NovelURL: "https://monkeydd.com/example.html", FontSize: -1},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.validate()
			if tt.wantErr && err == nil {
				t.Error("validate() = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validate() = %v, want nil", err)
			}
		})
	}
}

// An invalid request must fail before the crawler makes any request.
func TestExportRejectsInvalidRequestWithoutFetching(t *testing.T) {
	if _, err := Export(t.Context(), Request{NovelURL: "ftp://monkeydd.com/x.html", OutDir: t.TempDir()}); err == nil {
		t.Fatal("Export = nil error, want an invalid url error")
	}
}

func TestSafeFileName(t *testing.T) {
	tests := []struct {
		title    string
		fallback string
		want     string
	}{
		{"Trở Lại Năm Tháng Cũ", "slug", "Trở-Lại-Năm-Tháng-Cũ"},
		{"Chapter: One / Two", "slug", "Chapter-One-Two"},
		{"  ---  ", "slug", "slug"},
		{"", "slug", "slug"},
	}
	for _, tt := range tests {
		if got := SafeFileName(tt.title, tt.fallback); got != tt.want {
			t.Errorf("SafeFileName(%q, %q) = %q, want %q", tt.title, tt.fallback, got, tt.want)
		}
	}
}

func TestResultSummary(t *testing.T) {
	r := &Result{Title: "Example", Chapters: 12, Words: 3400}
	want := "Example — 12 chapters, 3400 words"
	if got := r.Summary(); got != want {
		t.Errorf("Summary() = %q, want %q", got, want)
	}
}
