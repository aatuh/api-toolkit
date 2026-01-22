package markdown

import (
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/net/html"
)

// RenderPlain converts Markdown input into a plain text body.
func RenderPlain(input string) (string, error) {
	htmlBody, err := Render(input)
	if err != nil {
		return "", err
	}
	return HTMLToText(htmlBody)
}

// HTMLToText strips HTML markup into plain text with basic list formatting.
func HTMLToText(input string) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "", nil
	}
	root, err := html.Parse(strings.NewReader(input))
	if err != nil {
		return "", err
	}
	renderer := &plainRenderer{}
	renderer.render(root)
	return strings.TrimSpace(renderer.b.String()), nil
}

type listState struct {
	ordered bool
	index   int
}

type linkState struct {
	href     string
	startLen int
}

type plainRenderer struct {
	b            strings.Builder
	listStack    []listState
	linkStack    []linkState
	lastSpace    bool
	newlineCount int
}

func (r *plainRenderer) render(node *html.Node) {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		r.renderNode(child)
	}
}

func (r *plainRenderer) renderNode(node *html.Node) {
	switch node.Type {
	case html.TextNode:
		r.writeText(node.Data)
		return
	case html.ElementNode:
		tag := strings.ToLower(strings.TrimSpace(node.Data))
		if tag == "script" || tag == "style" {
			return
		}
		if tag == "br" {
			r.newline()
			return
		}
		if tag == "ul" {
			r.blankLine()
			r.pushList(false)
		} else if tag == "ol" {
			r.blankLine()
			r.pushList(true)
		} else if tag == "li" {
			r.startListItem()
		} else if isBlockElement(tag) {
			r.blankLine()
		}
		if tag == "a" {
			r.pushLink(node)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			r.renderNode(child)
		}
		if tag == "a" {
			r.popLink()
		}
		if tag == "li" {
			r.newline()
		} else if tag == "ul" || tag == "ol" {
			r.popList()
			r.blankLine()
		} else if isBlockElement(tag) {
			r.blankLine()
		}
		return
	default:
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			r.renderNode(child)
		}
	}
}

func (r *plainRenderer) writeText(text string) {
	for _, ch := range text {
		if unicode.IsSpace(ch) {
			if r.b.Len() == 0 || r.newlineCount > 0 || r.lastSpace {
				continue
			}
			r.b.WriteByte(' ')
			r.lastSpace = true
			continue
		}
		r.b.WriteRune(ch)
		r.lastSpace = false
		r.newlineCount = 0
	}
}

func (r *plainRenderer) writeRaw(text string) {
	if text == "" {
		return
	}
	r.b.WriteString(text)
	r.lastSpace = strings.HasSuffix(text, " ")
	switch {
	case strings.HasSuffix(text, "\n\n"):
		r.newlineCount = 2
		r.lastSpace = false
	case strings.HasSuffix(text, "\n"):
		r.newlineCount = 1
		r.lastSpace = false
	default:
		r.newlineCount = 0
	}
}

func (r *plainRenderer) newline() {
	if r.b.Len() == 0 || r.newlineCount >= 1 {
		return
	}
	r.b.WriteByte('\n')
	r.newlineCount = 1
	r.lastSpace = false
}

func (r *plainRenderer) blankLine() {
	if r.b.Len() == 0 {
		return
	}
	for r.newlineCount < 2 {
		r.b.WriteByte('\n')
		r.newlineCount++
	}
	r.lastSpace = false
}

func (r *plainRenderer) pushList(ordered bool) {
	r.listStack = append(r.listStack, listState{ordered: ordered, index: 1})
}

func (r *plainRenderer) popList() {
	if len(r.listStack) == 0 {
		return
	}
	r.listStack = r.listStack[:len(r.listStack)-1]
}

func (r *plainRenderer) startListItem() {
	r.newline()
	prefix := "- "
	if len(r.listStack) > 0 {
		last := len(r.listStack) - 1
		if r.listStack[last].ordered {
			idx := r.listStack[last].index
			prefix = strconv.Itoa(idx) + ". "
			r.listStack[last] = listState{ordered: true, index: idx + 1}
		}
	}
	r.writeRaw(prefix)
}

func (r *plainRenderer) pushLink(node *html.Node) {
	href := ""
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, "href") {
			href = strings.TrimSpace(attr.Val)
			break
		}
	}
	if href == "" {
		return
	}
	r.linkStack = append(r.linkStack, linkState{href: href, startLen: r.b.Len()})
}

func (r *plainRenderer) popLink() {
	if len(r.linkStack) == 0 {
		return
	}
	idx := len(r.linkStack) - 1
	link := r.linkStack[idx]
	r.linkStack = r.linkStack[:idx]
	if link.href == "" {
		return
	}
	text := r.b.String()
	if link.startLen > len(text) {
		return
	}
	linkText := strings.TrimSpace(text[link.startLen:])
	if linkText == "" || strings.EqualFold(linkText, link.href) {
		if linkText == "" {
			r.writeRaw(link.href)
		}
		return
	}
	r.writeRaw(" (" + link.href + ")")
}

func isBlockElement(tag string) bool {
	switch tag {
	case "p", "div", "section", "article", "header", "footer", "aside", "main",
		"table", "thead", "tbody", "tfoot", "tr", "th", "td",
		"blockquote", "pre", "hr",
		"h1", "h2", "h3", "h4", "h5", "h6":
		return true
	default:
		return false
	}
}
