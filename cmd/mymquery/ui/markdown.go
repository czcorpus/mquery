package ui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/glamour"
	gansi "github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// mdCacheMaxItems bounds the number of cached rendered texts
// (the cache is just cleared when it is full)
const mdCacheMaxItems = 500

// result references like [r1] or [r2, r3] in the model's answers
var resultRefRegexp = regexp.MustCompile(`\[(r\d+(?:\s*,\s*r\d+)*)\]`)

// Result references are replaced by these (private use) runes before
// rendering, so they survive as plain text and can be highlighted afterwards.
// Spaces within a reference are replaced too, so a reference is never wrapped
// (each `rN` must stay on one line to be selectable, see chatLink).
const (
	refOpen  = '\ue000'
	refClose = '\ue001'
	refSpace = '\ue002'
)

var renderedRefRegexp = regexp.MustCompile("([^]*)")

// mdRenderer renders markdown (the AI chat answers) for the terminal.
// Renderers are created per width and rendered texts are cached.
type mdRenderer struct {
	renderers map[int]*glamour.TermRenderer
	cache     map[mdCacheKey]string
}

type mdCacheKey struct {
	text  string
	width int
}

func newMDRenderer() *mdRenderer {
	return &mdRenderer{
		renderers: make(map[int]*glamour.TermRenderer),
		cache:     make(map[mdCacheKey]string),
	}
}

// mdStyle is glamour's dark or light style (according to the terminal
// background) adjusted to fit into the chat: no margins and empty lines
// around the document, the terminal's default text color, accent headings.
func mdStyle() gansi.StyleConfig {
	st := styles.LightStyleConfig
	if lipgloss.HasDarkBackground() {
		st = styles.DarkStyleConfig
	}
	zero := uint(0)
	st.Document.Margin = &zero
	st.Document.BlockPrefix = ""
	st.Document.BlockSuffix = ""
	st.Document.Color = nil
	accent := colAccent.Light
	if lipgloss.HasDarkBackground() {
		accent = colAccent.Dark
	}
	st.Heading.Color = &accent
	// H1 has a background box by default which does not fit into a chat
	st.H1.BackgroundColor = nil
	st.H1.Color = &accent
	return st
}

func (r *mdRenderer) renderer(width int) (*glamour.TermRenderer, error) {
	if tr, ok := r.renderers[width]; ok {
		return tr, nil
	}
	tr, err := glamour.NewTermRenderer(
		glamour.WithStyles(mdStyle()),
		glamour.WithWordWrap(width),
		glamour.WithColorProfile(lipgloss.ColorProfile()),
	)
	if err != nil {
		return nil, err
	}
	r.renderers[width] = tr
	return tr, nil
}

// Render renders markdown text wrapped to `width` with result references
// highlighted. With `cache`, the result is cached (texts which are still
// changing, e.g. streamed ones, should not be). If the rendering fails,
// the text is shown as plain text.
func (r *mdRenderer) Render(text string, width int, cache bool) string {
	key := mdCacheKey{text: text, width: width}
	if ans, ok := r.cache[key]; ok {
		return ans
	}
	ans := r.render(text, width)
	if cache {
		if len(r.cache) >= mdCacheMaxItems {
			clear(r.cache)
		}
		r.cache[key] = ans
	}
	return ans
}

func (r *mdRenderer) render(text string, width int) string {
	segments := splitMDTables(text)
	parts := make([]string, 0, len(segments))
	for _, seg := range segments {
		var out string
		if seg.isTable {
			out = renderMDTable(seg.text, width)

		} else if strings.TrimSpace(seg.text) != "" {
			out = r.renderMarkdown(seg.text, width)
		}
		if out != "" {
			parts = append(parts, out)
		}
	}
	return strings.Join(parts, "\n\n")
}

// renderMarkdown renders markdown (without tables) via glamour
func (r *mdRenderer) renderMarkdown(text string, width int) string {
	tr, err := r.renderer(width)
	if err != nil {
		return renderPlainAnswer(text, width)
	}
	marked := resultRefRegexp.ReplaceAllStringFunc(text, markRef)
	out, err := tr.Render(marked)
	if err != nil {
		return renderPlainAnswer(text, width)
	}
	out = strings.Trim(out, "\n")
	// glamour pads lines to the full width; the padding is not needed
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		lines[i] = trimRightSpaceANSI(line)
	}
	out = strings.Join(lines, "\n")
	return renderedRefRegexp.ReplaceAllStringFunc(out, func(s string) string {
		inner := strings.TrimSuffix(strings.TrimPrefix(s, string(refOpen)), string(refClose))
		inner = strings.ReplaceAll(inner, string(refSpace), " ")
		return kwicStyle.Render("[" + inner + "]")
	})
}

// markRef replaces a reference like `[r1,  r3]` with its marked form
// (normalized to `r1, r3`, with the space replaced by refSpace)
func markRef(ref string) string {
	ids := strings.Split(strings.Trim(ref, "[]"), ",")
	for i, id := range ids {
		ids[i] = strings.TrimSpace(id)
	}
	return string(refOpen) + strings.Join(ids, ","+string(refSpace)) + string(refClose)
}

// trimRightSpaceANSI removes trailing spaces of a line which may contain
// ANSI sequences (the sequences are kept)
func trimRightSpaceANSI(line string) string {
	plain := ansi.Strip(line)
	trimmed := strings.TrimRight(plain, " ")
	if len(trimmed) == len(plain) {
		return line
	}
	return ansi.Truncate(line, ansi.StringWidth(trimmed), "") + "\x1b[0m"
}

// renderPlainAnswer wraps an answer as plain text (with highlighted references)
func renderPlainAnswer(text string, w int) string {
	return resultRefRegexp.ReplaceAllStringFunc(ansi.Wordwrap(text, w, " "), func(s string) string {
		return kwicStyle.Render(s)
	})
}
