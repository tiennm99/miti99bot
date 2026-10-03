package crawler

import (
	"net/url"
	"testing"
)

// novelFixture reproduces the two traits that break naive crawlers: the list is
// newest-first, and chapter numbering has a gap (no chapter 4).
const novelFixture = `<html><head><title>TEN TRUYEN</title></head><body>
<h1>TEN TRUYEN</h1>
<div class="list-chapters">
  <div class="item"><div class="episode-title"><a href="https://monkeydd.com/n/5.html">5</a></div></div>
  <div class="item"><div class="episode-title"><a href="https://monkeydd.com/n/3.html">3</a></div></div>
  <div class="item"><div class="episode-title"><a href="/n/2.html">2</a></div></div>
  <div class="item"><div class="episode-title"><a href="https://monkeydd.com/n/1.html">1</a></div></div>
</div></body></html>`

func TestParseNovelPageOrdersChaptersForReading(t *testing.T) {
	novel, err := ParseNovelPage([]byte(novelFixture), "https://monkeydd.com/n.html")
	if err != nil {
		t.Fatalf("ParseNovelPage: %v", err)
	}

	if novel.Title != "TEN TRUYEN" {
		t.Errorf("Title = %q", novel.Title)
	}
	if novel.Slug != "n" {
		t.Errorf("Slug = %q, want %q", novel.Slug, "n")
	}

	wantLabels := []string{"1", "2", "3", "5"}
	if len(novel.Chapters) != len(wantLabels) {
		t.Fatalf("got %d chapters, want %d", len(novel.Chapters), len(wantLabels))
	}
	for i, want := range wantLabels {
		if novel.Chapters[i].Label != want {
			t.Errorf("chapter %d label = %q, want %q", i, novel.Chapters[i].Label, want)
		}
	}
	// Relative hrefs must resolve against the novel URL.
	if got, want := novel.Chapters[1].URL, "https://monkeydd.com/n/2.html"; got != want {
		t.Errorf("chapter 2 URL = %q, want %q", got, want)
	}
}

// tagsFixture mirrors the real page: the novel's own genres carry
// itemprop="genre", while a site-wide menu links every category without it. A
// parser matching on the /the-loai/ href shape would swallow the whole menu.
const tagsFixture = `<html><head><title>TEN TRUYEN</title></head><body>
<h1>TEN TRUYEN</h1>
<dl class="row">
  <dt class="col-sm-3">Thể loại</dt>
  <dd class="col-sm-9">
    <a class='cate-item' itemprop='genre' title='Trọng Sinh' href='https://monkeydd.com/the-loai/trong-sinh.html'>Trọng Sinh</a>
    <a class='cate-item' itemprop='genre' title='Cổ Đại' href='https://monkeydd.com/the-loai/co-dai.html'>Cổ  Đại</a>
    <a class='cate-item' itemprop='genre' title='Gia Đình' href='https://monkeydd.com/the-loai/gia-dinh.html'></a>
    <a class='cate-item' itemprop='genre' title='Cổ Đại' href='https://monkeydd.com/the-loai/co-dai.html'>Cổ Đại</a>
  </dd>
</dl>
<nav class="site-menu">
  <a href='https://monkeydd.com/the-loai/dam-my.html'>Đam Mỹ</a>
  <a href='https://monkeydd.com/the-loai/bach-hop.html'>Bách Hợp</a>
</nav>
<div class="list-chapters">
  <div class="item"><div class="episode-title"><a href="/n/1.html">1</a></div></div>
</div></body></html>`

func TestParseNovelPageReadsTags(t *testing.T) {
	novel, err := ParseNovelPage([]byte(tagsFixture), "https://monkeydd.com/n.html")
	if err != nil {
		t.Fatalf("ParseNovelPage: %v", err)
	}

	// "Cổ  Đại" has its inner whitespace collapsed, the empty anchor falls back
	// to its title attribute, and the repeated genre appears once.
	want := []string{"Trọng Sinh", "Cổ Đại", "Gia Đình"}
	if len(novel.Tags) != len(want) {
		t.Fatalf("Tags = %q, want %q", novel.Tags, want)
	}
	for i := range want {
		if novel.Tags[i] != want[i] {
			t.Errorf("Tags[%d] = %q, want %q", i, novel.Tags[i], want[i])
		}
	}
}

func TestParseNovelPageWithoutTags(t *testing.T) {
	novel, err := ParseNovelPage([]byte(novelFixture), "https://monkeydd.com/n.html")
	if err != nil {
		t.Fatalf("ParseNovelPage: %v", err)
	}
	if len(novel.Tags) != 0 {
		t.Errorf("Tags = %q, want none", novel.Tags)
	}
}

func TestParseNovelPageNoChapters(t *testing.T) {
	if _, err := ParseNovelPage([]byte(`<html><title>x</title><body></body></html>`),
		"https://monkeydd.com/n.html"); err == nil {
		t.Fatal("want an error when no chapters are listed")
	}
}

const selectFixture = `<html><body>
<select name="selected_chapter" id="selected_chapter">
  <option value="n,5">5</option>
  <option value="n,chuong-3">Chương 3</option>
  <option value="n,1">1</option>
  <option value="14">malformed</option>
</select></body></html>`

func TestChapterRefsFromSelect(t *testing.T) {
	base, err := url.Parse("https://monkeydd.com/n.html")
	if err != nil {
		t.Fatal(err)
	}
	refs, err := ChapterRefsFromSelect([]byte(selectFixture), base)
	if err != nil {
		t.Fatalf("ChapterRefsFromSelect: %v", err)
	}

	if len(refs) != 3 {
		t.Fatalf("got %d refs %+v, want 3 (malformed option skipped)", len(refs), refs)
	}
	if got, want := refs[0].URL, "https://monkeydd.com/n/1.html"; got != want {
		t.Errorf("first ref URL = %q, want %q", got, want)
	}
	if got, want := refs[1].URL, "https://monkeydd.com/n/chuong-3.html"; got != want {
		t.Errorf("second ref URL = %q, want %q", got, want)
	}
}

func TestReconcileChapterRefs(t *testing.T) {
	ref := func(u string) ChapterRef { return ChapterRef{Label: u, URL: u} }

	t.Run("dropdown superset wins", func(t *testing.T) {
		list := []ChapterRef{ref("a"), ref("b")}
		sel := []ChapterRef{ref("a"), ref("b"), ref("c")}

		final, extra := ReconcileChapterRefs(list, sel)
		if len(final) != 3 {
			t.Errorf("got %d chapters, want the 3 from the dropdown", len(final))
		}
		if len(extra) != 0 {
			t.Errorf("got %d extra, want 0", len(extra))
		}
	})

	t.Run("disagreement is reported not silently merged", func(t *testing.T) {
		list := []ChapterRef{ref("a"), ref("z")}
		sel := []ChapterRef{ref("a"), ref("b")}

		final, extra := ReconcileChapterRefs(list, sel)
		if len(final) != 2 || final[1].URL != "z" {
			t.Errorf("final = %+v, want the landing page list", final)
		}
		if len(extra) != 1 || extra[0].URL != "b" {
			t.Errorf("extra = %+v, want [b] so the caller can warn", extra)
		}
	})

	t.Run("empty dropdown falls back to the list", func(t *testing.T) {
		list := []ChapterRef{ref("a")}
		final, extra := ReconcileChapterRefs(list, nil)
		if len(final) != 1 || len(extra) != 0 {
			t.Errorf("final = %+v, extra = %+v", final, extra)
		}
	})
}
