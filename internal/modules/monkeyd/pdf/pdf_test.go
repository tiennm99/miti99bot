package pdf

import (
	"os"
	"path/filepath"
	"testing"
)

func testOptions(t *testing.T) Options {
	t.Helper()
	return Options{
		Page:        PhonePage,
		Margin:      6,
		Font:        BundledFont(),
		FontSize:    12,
		LineSpacing: 1.55,
		Title:       "TRỞ LẠI NĂM THÁNG CŨ",
		SourceURL:   "https://example.test/n.html",
	}
}

func TestWriteProducesReadablePDF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.pdf")

	chapters := []Chapter{
		{Heading: "Chương 1", Paragraphs: []string{
			"Nghe vị trưởng tử nói chuyện với nàng.",
			"Một đoạn văn khác để kiểm tra ngắt dòng tự động trên trang nhỏ.",
		}},
		{Heading: "Chương 2", Paragraphs: []string{"Đoạn cuối."}},
	}

	if err := Write(path, testOptions(t), chapters); err != nil {
		t.Fatalf("Write: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat output: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("wrote an empty PDF")
	}

	header := make([]byte, 5)
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.Read(header); err != nil {
		t.Fatal(err)
	}
	if string(header) != "%PDF-" {
		t.Errorf("output does not start with a PDF header, got %q", header)
	}
}

// A novel with many chapters must not overflow a page; auto page break plus the
// footer reserve handles that, so a long chapter should span several pages.
func TestWriteHandlesLongChapters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "long.pdf")

	paragraphs := make([]string, 200)
	for i := range paragraphs {
		paragraphs[i] = "Một đoạn văn dài để buộc trình kết xuất phải sang trang mới nhiều lần."
	}

	if err := Write(path, testOptions(t), []Chapter{{Heading: "Chương 1", Paragraphs: paragraphs}}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() < 2000 {
		t.Errorf("output suspiciously small (%d bytes) for 200 paragraphs", info.Size())
	}
}
