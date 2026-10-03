package crawler

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// contentElementID is the container holding a chapter's body text.
const contentElementID = "chapter-content-render"

// Chapter is one fetched chapter, reduced to plain paragraphs.
type Chapter struct {
	Label      string
	URL        string
	Paragraphs []string
}

// Heading is the chapter title to print. Labels are often a bare number, which
// reads poorly as a heading, so those get the Vietnamese word for "chapter".
func (c *Chapter) Heading() string {
	label := strings.TrimSpace(c.Label)
	if label == "" {
		return "Chương"
	}
	if _, err := strconv.Atoi(label); err == nil {
		return "Chương " + label
	}
	return label
}

// WordCount is a rough word count, used to sanity-check extraction.
func (c *Chapter) WordCount() int {
	n := 0
	for _, p := range c.Paragraphs {
		n += len(strings.Fields(p))
	}
	return n
}

// blockTags end the current paragraph when opened or closed.
var blockTags = map[string]bool{
	"p": true, "div": true, "br": true, "hr": true, "blockquote": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"li": true, "ul": true, "ol": true, "tr": true,
}

// skipTags never contribute prose.
//
// Anchors are included because inside a chapter body they are always site
// chrome: the prev/next chapter navigation and the sponsor call-to-action are
// both links, while novel prose never needs one. Inline emphasis tags (b, i,
// em) are deliberately absent so italics in the prose survive; the site's icons
// use <i> but carry no text.
var skipTags = map[string]bool{
	"script": true, "style": true, "noscript": true, "iframe": true,
	"ins": true, "form": true, "select": true, "button": true, "textarea": true,
	"a": true, "img": true, "svg": true,
}

// junkClasses marks containers the site injects into the chapter body. Their
// whole subtree is dropped.
//
// These are matched on class rather than position because the blocks move: the
// sponsor block opens the body on most chapters but is absent on others, and
// the watermark is planted at a different paragraph in every chapter. Only
// site-specific class names are listed; generic Bootstrap utilities such as
// "my-4" or "text-center" are not, since prose could legitimately carry them.
//
// Note the sibling class "actac" is NOT junk: it wraps the real chapter text and
// carries style="display:none", because the site gates the body behind a click
// on the sponsor link and reveals it with JavaScript. Skipping hidden elements,
// or skipping "act*" as a family, would therefore discard the whole chapter.
var junkClasses = map[string]bool{
	"actcl":     true, // sponsor block shown in place of the gated chapter body
	"signature": true, // "[Truyện được đăng tải duy nhất tại ...]" source watermark
}

// hasJunkClass reports whether a node is an injected non-prose container.
func hasJunkClass(n *html.Node) bool {
	for _, tok := range strings.Fields(attr(n, "class")) {
		if junkClasses[tok] {
			return true
		}
	}
	return false
}

// ParseChapter extracts a chapter's paragraphs, restoring the words the site
// serves through CSS :before rules instead of markup.
func ParseChapter(page []byte, ref ChapterRef) (*Chapter, error) {
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return nil, fmt.Errorf("parse chapter %s: %w", ref.URL, err)
	}
	content := elementByID(doc, contentElementID)
	if content == nil {
		return nil, fmt.Errorf("chapter %s: no #%s container (page layout may have changed)",
			ref.URL, contentElementID)
	}

	ch := &Chapter{
		Label:      ref.Label,
		URL:        ref.URL,
		Paragraphs: extractParagraphs(content, ParseWordClasses(page)),
	}
	if len(ch.Paragraphs) == 0 {
		return nil, fmt.Errorf("chapter %s: extracted no text", ref.URL)
	}
	ch.Paragraphs = dropRepeatedTitle(ch.Paragraphs, ref.Label)
	return ch, nil
}

// extractParagraphs walks the content subtree into plain paragraphs, replacing
// each word-carrying element with the word its CSS rule injects.
func extractParagraphs(content *html.Node, words map[string]string) []string {
	var b strings.Builder

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		switch n.Type {
		case html.TextNode:
			// The HTML parser has already decoded entities such as &#432;.
			b.WriteString(n.Data)
			return
		case html.ElementNode:
			if skipTags[n.Data] || hasJunkClass(n) {
				return
			}
			// These elements are empty in the markup; the CSS word replaces them.
			if word, ok := injectedWord(n, words); ok {
				b.WriteString(word)
				return
			}
			if blockTags[n.Data] {
				b.WriteByte('\n')
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}

		if n.Type == html.ElementNode && blockTags[n.Data] {
			b.WriteByte('\n')
		}
	}
	walk(content)

	var paragraphs []string
	for _, line := range strings.Split(b.String(), "\n") {
		if p := collapseSpaces(line); p != "" {
			paragraphs = append(paragraphs, p)
		}
	}
	return paragraphs
}

// injectedWord returns the word a node's class supplies via CSS, if any.
func injectedWord(n *html.Node, words map[string]string) (string, bool) {
	class := attr(n, "class")
	if class == "" {
		return "", false
	}
	for _, tok := range strings.Fields(class) {
		if word, ok := words[tok]; ok {
			return word, true
		}
	}
	return "", false
}

// dropRepeatedTitle removes a leading paragraph that only repeats the chapter
// label, since the export prints its own heading.
func dropRepeatedTitle(paragraphs []string, label string) []string {
	if len(paragraphs) < 2 {
		return paragraphs
	}
	first := strings.TrimSpace(paragraphs[0])
	if strings.EqualFold(first, strings.TrimSpace(label)) {
		return paragraphs[1:]
	}
	return paragraphs
}
