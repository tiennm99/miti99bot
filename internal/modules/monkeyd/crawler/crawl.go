package crawler

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
)

// Crawler fetches a novel and its chapters.
type Crawler struct {
	Client *Client

	// CacheDir, when set, stores raw pages on disk and serves later runs from
	// them. Re-exporting with different font or page settings then costs no
	// requests.
	CacheDir string

	// Workers bounds concurrent fetches. The client's delay still caps the
	// overall request rate.
	Workers int

	// Log receives progress messages. Optional.
	Log func(format string, args ...any)
}

func (c *Crawler) logf(format string, args ...any) {
	if c.Log != nil {
		c.Log(format, args...)
	}
}

// Novel fetches a novel landing page and resolves its chapter list.
//
// The chapter list is taken from the landing page and cross-checked against the
// dropdown embedded in the first chapter page, so a truncated list cannot
// silently shorten the export.
func (c *Crawler) Novel(ctx context.Context, novelURL string) (*Novel, error) {
	page, err := c.page(ctx, novelURL)
	if err != nil {
		return nil, err
	}
	novel, err := ParseNovelPage(page, novelURL)
	if err != nil {
		return nil, err
	}
	c.logf("novel: %s (%d chapters listed)", novel.Title, len(novel.Chapters))

	base, err := url.Parse(novelURL)
	if err != nil {
		return nil, err
	}
	firstPage, err := c.page(ctx, novel.Chapters[0].URL)
	if err != nil {
		return nil, err
	}
	fromSelect, err := ChapterRefsFromSelect(firstPage, base)
	if err != nil {
		return nil, err
	}

	final, extra := ReconcileChapterRefs(novel.Chapters, fromSelect)
	if len(extra) > 0 {
		c.logf("warning: %d chapter(s) appear only in the chapter dropdown and were not "+
			"in the landing page list; verify the export is complete", len(extra))
	}
	if len(final) != len(novel.Chapters) {
		c.logf("chapter list reconciled to %d chapters using the in-chapter dropdown", len(final))
	}
	novel.Chapters = final
	return novel, nil
}

// NovelInfo fetches only the landing page and returns what it carries: title,
// slug, tags, and the chapter list as that page shows it.
//
// Unlike Novel it does not also fetch a chapter page to cross-check the chapter
// list, so it costs a single request. Callers that only want metadata should
// prefer it; callers about to export every chapter want Novel, whose
// reconciliation guards against a silently truncated list.
func (c *Crawler) NovelInfo(ctx context.Context, novelURL string) (*Novel, error) {
	page, err := c.page(ctx, novelURL)
	if err != nil {
		return nil, err
	}
	return ParseNovelPage(page, novelURL)
}

// Chapters fetches every chapter concurrently and returns them in reading
// order. Any chapter that cannot be fetched or parsed fails the whole run
// rather than yielding a book with a hole in it.
func (c *Crawler) Chapters(ctx context.Context, novel *Novel) ([]*Chapter, error) {
	chapters := make([]*Chapter, len(novel.Chapters))

	workers := c.Workers
	if workers < 1 {
		workers = 1
	}

	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(workers)

	var mu sync.Mutex
	done := 0

	for i, ref := range novel.Chapters {
		i, ref := i, ref
		group.Go(func() error {
			page, err := c.page(groupCtx, ref.URL)
			if err != nil {
				return err
			}
			chapter, err := ParseChapter(page, ref)
			if err != nil {
				return err
			}
			chapters[i] = chapter

			mu.Lock()
			done++
			c.logf("fetched %d/%d: %s (%d words)", done, len(novel.Chapters),
				chapter.Heading(), chapter.WordCount())
			mu.Unlock()
			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return nil, err
	}
	return chapters, nil
}

// page returns a page from the cache when available, otherwise fetches and
// caches it.
func (c *Crawler) page(ctx context.Context, pageURL string) ([]byte, error) {
	path := c.cachePath(pageURL)
	if path != "" {
		if body, err := os.ReadFile(path); err == nil && len(body) > 0 { //nolint:gosec // G304: cachePath strips every path separator from the URL
			return body, nil
		}
	}

	body, err := c.Client.Get(ctx, pageURL)
	if err != nil {
		return nil, err
	}

	if path != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err == nil {
			// A failed cache write must not fail the crawl.
			_ = os.WriteFile(path, body, 0o600)
		}
	}
	return body, nil
}

// unsafeFileChars matches everything not allowed in a cache file name.
var unsafeFileChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// cachePath maps a page URL to a cache file, or "" when caching is disabled.
func (c *Crawler) cachePath(pageURL string) string {
	if c.CacheDir == "" {
		return ""
	}
	u, err := url.Parse(pageURL)
	if err != nil {
		return ""
	}
	name := unsafeFileChars.ReplaceAllString(strings.Trim(u.Path, "/"), "_")
	if name == "" {
		return ""
	}
	if !strings.HasSuffix(name, ".html") {
		name += ".html"
	}
	return filepath.Join(c.CacheDir, name)
}

// TotalWords sums the word count across chapters.
func TotalWords(chapters []*Chapter) int {
	n := 0
	for _, ch := range chapters {
		n += ch.WordCount()
	}
	return n
}

// Describe renders a one-line summary of a crawl result.
func Describe(novel *Novel, chapters []*Chapter) string {
	return fmt.Sprintf("%s — %d chapters, %d words", novel.Title, len(chapters), TotalWords(chapters))
}
