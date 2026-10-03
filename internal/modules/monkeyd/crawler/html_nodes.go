package crawler

import (
	"strings"

	"golang.org/x/net/html"
)

// attr returns the value of the named attribute, or "" when absent.
func attr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}

// hasClass reports whether the node carries the given class token.
func hasClass(n *html.Node, class string) bool {
	for _, tok := range strings.Fields(attr(n, "class")) {
		if tok == class {
			return true
		}
	}
	return false
}

// findNode returns the first node in document order satisfying match.
func findNode(root *html.Node, match func(*html.Node) bool) *html.Node {
	if match(root) {
		return root
	}
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		if found := findNode(c, match); found != nil {
			return found
		}
	}
	return nil
}

// findAllNodes returns every node satisfying match, in document order.
func findAllNodes(root *html.Node, match func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if match(n) {
			out = append(out, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return out
}

// elementByID finds an element by its id attribute.
func elementByID(root *html.Node, id string) *html.Node {
	return findNode(root, func(n *html.Node) bool {
		return n.Type == html.ElementNode && attr(n, "id") == id
	})
}

// elementByTag finds the first element with the given tag name.
func elementByTag(root *html.Node, tag string) *html.Node {
	return findNode(root, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == tag
	})
}

// nodeText collects the descendant text of a node with whitespace collapsed.
func nodeText(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return collapseSpaces(b.String())
}

// collapseSpaces trims the string and reduces every whitespace run, including
// the non-breaking spaces the site emits as &nbsp;, to a single space.
func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
