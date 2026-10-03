package crawler

import (
	"strings"
	"testing"
)

// chapterFixture mirrors the real page shape: words split between markup and
// CSS :before rules, HTML entities, &nbsp; spacer paragraphs and an ad script
// inside the content container.
const chapterFixture = `<!DOCTYPE html><html><head>
<style>
.t-aaa:before { content: "v\1ecb "; }
.j-bbb:before { content: "tr\1ed3 "; }
.z-ccc::before{content:"n\E0ng";}
.unused-ddd:before { content: "khong-dung"; }
</style></head><body>
<h1 class="card-title">TEN TRUYEN - 1</h1>
<div class="content-container" id="chapter-content-render">
<p>1</p>
<p>&nbsp;</p>
<p>Nghe <span class="t-aaa"></span> tr&#432;&#7903;ng t&#7917; noi.</p>
<p>&nbsp;</p>
<p>M&#7897;t <span class="j-bbb"></span> v&#224; <span class="z-ccc"></span> di.</p>
<script>ads();</script>
</div></body></html>`

func TestParseWordClassesDecodesEscapes(t *testing.T) {
	words := ParseWordClasses([]byte(chapterFixture))

	for class, want := range map[string]string{
		"t-aaa": "vị",
		"j-bbb": "trồ",
		"z-ccc": "nàng",
	} {
		if got := words[class]; got != want {
			t.Errorf("words[%q] = %q, want %q", class, got, want)
		}
	}
	if len(words) != 4 {
		t.Errorf("got %d rules, want 4", len(words))
	}
}

func TestParseChapterRestoresCSSWords(t *testing.T) {
	ch, err := ParseChapter([]byte(chapterFixture), ChapterRef{Label: "1", URL: "http://x/1.html"})
	if err != nil {
		t.Fatalf("ParseChapter: %v", err)
	}

	want := []string{
		"Nghe vị trưởng tử noi.",
		"Một trồ và nàng di.",
	}
	if len(ch.Paragraphs) != len(want) {
		t.Fatalf("got %d paragraphs %q, want %d", len(ch.Paragraphs), ch.Paragraphs, len(want))
	}
	for i, w := range want {
		if ch.Paragraphs[i] != w {
			t.Errorf("paragraph %d = %q, want %q", i, ch.Paragraphs[i], w)
		}
	}
}

// The CSS-injected words are the difference between real text and text with
// silent holes, so guard against a regression that drops them.
func TestParseChapterWithoutCSSWouldLoseWords(t *testing.T) {
	withoutCSS := strings.Replace(chapterFixture, `.t-aaa:before { content: "v\1ecb "; }`, "", 1)

	ch, err := ParseChapter([]byte(withoutCSS), ChapterRef{Label: "1", URL: "http://x/1.html"})
	if err != nil {
		t.Fatalf("ParseChapter: %v", err)
	}
	if strings.Contains(ch.Paragraphs[0], "vị") {
		t.Fatal("word appeared without its CSS rule; fixture no longer proves anything")
	}
	if want := "Nghe trưởng tử noi."; ch.Paragraphs[0] != want {
		t.Errorf("paragraph 0 = %q, want %q", ch.Paragraphs[0], want)
	}
}

func TestParseChapterDropsScriptsAndSpacers(t *testing.T) {
	ch, err := ParseChapter([]byte(chapterFixture), ChapterRef{Label: "1", URL: "http://x/1.html"})
	if err != nil {
		t.Fatalf("ParseChapter: %v", err)
	}
	for _, p := range ch.Paragraphs {
		if strings.Contains(p, "ads()") {
			t.Errorf("script text leaked into paragraph %q", p)
		}
		if strings.TrimSpace(p) == "" {
			t.Error("empty spacer paragraph was kept")
		}
		if strings.Contains(p, " ") {
			t.Errorf("non-breaking space survived in %q", p)
		}
	}
}

// The leading "<p>1</p>" repeats the chapter label and would print twice.
func TestParseChapterDropsRepeatedTitle(t *testing.T) {
	ch, err := ParseChapter([]byte(chapterFixture), ChapterRef{Label: "1", URL: "http://x/1.html"})
	if err != nil {
		t.Fatalf("ParseChapter: %v", err)
	}
	if ch.Paragraphs[0] == "1" {
		t.Error("repeated chapter label was kept as a paragraph")
	}
}

// gatedChapterFixture mirrors how most chapters are served: a visible sponsor
// block (div.actcl) stands in for the body, while the real text sits in a
// sibling div.actac hidden with display:none and revealed by the site's
// JavaScript. A source watermark and prev/next navigation bracket the prose.
const gatedChapterFixture = `<!DOCTYPE html><html><head>
<style>.t-aaa:before { content: "v\1ecb"; }</style></head><body>
<div class="content-container" id="chapter-content-render">
  <div class="actcl">
    <h4 class="text-center text-primary">Moi Quy doc gia CLICK vao lien ket</h4>
    <p class="text-center">mo ung dung Shopee, sau do quay tro lai de doc!</p>
    <a class="btn btn-primary px-3" href="https://s.shopee.vn/xxxx">
      <img src="x.jpg"><span class="text-uppercase text-danger">CLICK</span></a>
    <h4 class="text-center text-primary">MonkeyD va doi ngu Editor xin chan thanh cam on!</h4>
  </div>
  <div class="actac" style=" display:none; ">
    <p>Doan van dau tien co <span class="t-aaa"></span> tri.</p>
    <p>&nbsp;</p>
    <p class="signature">[Truyen duoc dang tai duy nhat tai MonkeyDD.com - https://monkeydd.com/n/10.html.]</p>
    <p>Doan van cuoi cung.</p>
  </div>
  <div class="my-4"><div class="d-flex justify-content-center">
    <a class="btn btn-primary px-3 me-2" href="/n/9.html"><i class="bx bx-chevron-left"></i>Chương trước</a>
    <a class="btn btn-primary px-3" href="/n/11.html">Chương sau<i class="bx bx-chevron-right"></i></a>
  </div></div>
</div></body></html>`

// The gated body is the one thing that must survive: it is hidden with
// display:none, so any rule that drops hidden or "act*" containers silently
// discards the entire chapter.
func TestParseChapterKeepsGatedBody(t *testing.T) {
	ch, err := ParseChapter([]byte(gatedChapterFixture), ChapterRef{Label: "10", URL: "http://x/10.html"})
	if err != nil {
		t.Fatalf("ParseChapter: %v", err)
	}

	want := []string{
		"Doan van dau tien co vị tri.",
		"Doan van cuoi cung.",
	}
	if len(ch.Paragraphs) != len(want) {
		t.Fatalf("got %d paragraphs %q, want %d", len(ch.Paragraphs), ch.Paragraphs, len(want))
	}
	for i, w := range want {
		if ch.Paragraphs[i] != w {
			t.Errorf("paragraph %d = %q, want %q", i, ch.Paragraphs[i], w)
		}
	}
}

// Everything the site injects around the prose must be gone.
func TestParseChapterDropsInjectedBlocks(t *testing.T) {
	ch, err := ParseChapter([]byte(gatedChapterFixture), ChapterRef{Label: "10", URL: "http://x/10.html"})
	if err != nil {
		t.Fatalf("ParseChapter: %v", err)
	}
	body := strings.Join(ch.Paragraphs, "\n")

	for _, junk := range []string{
		"Shopee",       // sponsor copy
		"CLICK",        // sponsor call to action
		"cam on",       // sponsor sign-off
		"MonkeyDD.com", // source watermark
		"Chương trước", // navigation
		"Chương sau",   // navigation
	} {
		if strings.Contains(body, junk) {
			t.Errorf("injected text %q survived extraction in %q", junk, body)
		}
	}
}

func TestParseChapterMissingContainer(t *testing.T) {
	if _, err := ParseChapter([]byte(`<html><body><p>hi</p></body></html>`),
		ChapterRef{URL: "http://x/1.html"}); err == nil {
		t.Fatal("want an error when the content container is absent")
	}
}

func TestChapterHeading(t *testing.T) {
	for _, tc := range []struct{ label, want string }{
		{"14", "Chương 14"},
		{"Chương 12", "Chương 12"},
		{"", "Chương"},
	} {
		ch := &Chapter{Label: tc.label}
		if got := ch.Heading(); got != tc.want {
			t.Errorf("Heading(%q) = %q, want %q", tc.label, got, tc.want)
		}
	}
}
