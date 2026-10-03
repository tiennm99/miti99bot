package crawler

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// ChapterRef points at a single chapter listed on a novel page.
type ChapterRef struct {
	Label string // as shown on the site, e.g. "14" or "Chương 12"
	URL   string
}

// Novel is a novel's landing page: its title and its chapters in reading order.
type Novel struct {
	Title string
	Slug  string
	URL   string

	// Tags are the novel's own genres, as labelled on the site, in the order
	// they appear. Empty when the page lists none.
	Tags     []string
	Chapters []ChapterRef
}

// ParseNovelPage reads the title and chapter list from a novel landing page.
//
// Chapter URLs are always taken from the anchors on the page. Slugs are not
// uniform across novels ("/14.html" on one, "/chuong-12.html" on another) and
// numbering has gaps, so generating URLs from a chapter count would fetch 404s
// and miss real chapters.
func ParseNovelPage(page []byte, pageURL string) (*Novel, error) {
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return nil, fmt.Errorf("parse novel page: %w", err)
	}
	base, err := url.Parse(pageURL)
	if err != nil {
		return nil, fmt.Errorf("parse novel url: %w", err)
	}

	novel := &Novel{
		Title:    novelTitle(doc),
		Slug:     slugFromNovelURL(base),
		URL:      pageURL,
		Tags:     tagsFromInfo(doc),
		Chapters: chapterRefsFromList(doc, base),
	}
	if novel.Title == "" {
		novel.Title = novel.Slug
	}
	if len(novel.Chapters) == 0 {
		return nil, fmt.Errorf("no chapters found on %s (page layout may have changed)", pageURL)
	}
	return novel, nil
}

// novelTitle prefers the <h1> heading and falls back to the document title.
func novelTitle(doc *html.Node) string {
	if h1 := elementByTag(doc, "h1"); h1 != nil {
		if t := nodeText(h1); t != "" {
			return t
		}
	}
	return nodeText(elementByTag(doc, "title"))
}

// tagsFromInfo reads the novel's own genres out of the info block.
//
// The anchors are matched on the schema.org itemprop="genre" microdata rather
// than on their href. The page also carries a site-wide genre menu linking every
// category — 69 distinct ones against this novel's 6 on a sampled page — so
// matching the /the-loai/ URL shape would pull in the whole navigation.
func tagsFromInfo(doc *html.Node) []string {
	links := findAllNodes(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "a" && attr(n, "itemprop") == "genre"
	})

	var tags []string
	seen := make(map[string]bool, len(links))
	for _, link := range links {
		// The label appears both as the link text and as its title attribute;
		// prefer the text and fall back to the attribute.
		name := nodeText(link)
		if name == "" {
			name = collapseSpaces(attr(link, "title"))
		}
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		tags = append(tags, name)
	}
	return tags
}

// slugFromNovelURL turns https://host/tro-lai-nam-thang-cu.html into
// "tro-lai-nam-thang-cu".
func slugFromNovelURL(u *url.URL) string {
	seg := strings.Trim(u.Path, "/")
	if i := strings.LastIndex(seg, "/"); i >= 0 {
		seg = seg[i+1:]
	}
	return strings.TrimSuffix(seg, ".html")
}

// chapterRefsFromList reads the "list-chapters" block on the landing page.
// The site lists newest first, so the result is reversed into reading order.
func chapterRefsFromList(doc *html.Node, base *url.URL) []ChapterRef {
	list := findNode(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && hasClass(n, "list-chapters")
	})
	if list == nil {
		return nil
	}

	titles := findAllNodes(list, func(n *html.Node) bool {
		return n.Type == html.ElementNode && hasClass(n, "episode-title")
	})

	var refs []ChapterRef
	for _, title := range titles {
		link := elementByTag(title, "a")
		if link == nil {
			continue
		}
		href := strings.TrimSpace(attr(link, "href"))
		if href == "" {
			continue
		}
		abs, err := base.Parse(href)
		if err != nil {
			continue
		}
		refs = append(refs, ChapterRef{Label: nodeText(link), URL: abs.String()})
	}
	return reverseRefs(refs)
}

// ChapterRefsFromSelect reads the chapter dropdown embedded in every chapter
// page, whose options hold "novel-slug,chapter-slug" pairs. This is a second,
// independent view of the chapter list used to cross-check the landing page.
func ChapterRefsFromSelect(page []byte, base *url.URL) ([]ChapterRef, error) {
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return nil, fmt.Errorf("parse chapter page: %w", err)
	}
	sel := elementByID(doc, "selected_chapter")
	if sel == nil {
		return nil, nil
	}

	var refs []ChapterRef
	for _, opt := range findAllNodes(sel, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "option"
	}) {
		novelSlug, chapterSlug, ok := strings.Cut(attr(opt, "value"), ",")
		if !ok || novelSlug == "" || chapterSlug == "" {
			continue
		}
		abs, err := base.Parse("/" + novelSlug + "/" + chapterSlug + ".html")
		if err != nil {
			continue
		}
		refs = append(refs, ChapterRef{Label: nodeText(opt), URL: abs.String()})
	}
	return reverseRefs(refs), nil
}

// ReconcileChapterRefs picks the chapter list to crawl from the landing page
// list and the in-chapter dropdown.
//
// Both views come from the same site ordering, so when the dropdown is a
// superset it is preferred: that keeps the run correct even if the landing page
// ever truncates or paginates its list. Anything the dropdown alone knows about
// while disagreeing on order is returned as extra so the caller can warn rather
// than silently export a short book.
func ReconcileChapterRefs(fromList, fromSelect []ChapterRef) (final, extra []ChapterRef) {
	if len(fromSelect) == 0 {
		return fromList, nil
	}

	inSelect := refURLSet(fromSelect)
	if listIsSubsetOf(fromList, inSelect) {
		return fromSelect, nil
	}

	inList := refURLSet(fromList)
	for _, ref := range fromSelect {
		if !inList[ref.URL] {
			extra = append(extra, ref)
		}
	}
	return fromList, extra
}

func refURLSet(refs []ChapterRef) map[string]bool {
	set := make(map[string]bool, len(refs))
	for _, r := range refs {
		set[r.URL] = true
	}
	return set
}

func listIsSubsetOf(refs []ChapterRef, set map[string]bool) bool {
	for _, r := range refs {
		if !set[r.URL] {
			return false
		}
	}
	return true
}

func reverseRefs(refs []ChapterRef) []ChapterRef {
	for i, j := 0, len(refs)-1; i < j; i, j = i+1, j-1 {
		refs[i], refs[j] = refs[j], refs[i]
	}
	return refs
}
