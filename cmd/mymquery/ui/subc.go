package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"mquery/cmd/mymquery/api"
	"mquery/cmd/mymquery/storage"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// subcState is the subcorpus selected for the session (zero value = none)
type subcState struct {
	ID string
	// Alias marks a subcorpus configured in MQuery (via text types)
	Alias bool
	// Size and CorpusSize are in tokens (0 = unknown, e.g. for aliases)
	Size       int64
	CorpusSize int64
}

// summary describes the subcorpus for the query screen
func (s subcState) summary() string {
	if s.ID == "" {
		return "whole corpus"
	}
	switch {
	case s.Alias:
		return s.ID + " (configured)"
	case s.CorpusSize > 0:
		return fmt.Sprintf("%s (%s tokens, %.1f %% of the corpus)",
			s.ID, formatInt(int(s.Size)), pctOf(s.Size, s.CorpusSize))
	default:
		return fmt.Sprintf("%s (%s tokens)", s.ID, formatInt(int(s.Size)))
	}
}

// subcSelectedMsg is emitted by the picker when a (verified) subcorpus is chosen
type subcSelectedMsg struct {
	subc subcState
	// notice is an optional (non-fatal) problem to be shown
	notice string
}

// subcCheckedMsg delivers a result of a subcorpus check (see api.Client.SubcorpusInfo)
type subcCheckedMsg struct {
	reqID int
	info  api.SubcorpusInfo
	err   error
}

// parseSubcID validates a typed subcorpus ID
func parseSubcID(s string) (string, error) {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return "", errors.New("a subcorpus ID must have at least 2 characters")
	}
	if strings.ContainsFunc(s, func(r rune) bool { return unicode.IsSpace(r) || r == '/' || r == '\\' }) {
		return "", errors.New("a subcorpus ID must not contain spaces or slashes")
	}
	return s, nil
}

// subcPicker is a modal for choosing a subcorpus of a corpus: the whole corpus,
// subcorpora configured in MQuery (aliases), saved subcorpora used before
// (stored in the local DB, optionally with a label), or a typed ID of a saved
// subcorpus. Saved subcorpora are verified by the server before they are accepted.
type subcPicker struct {
	env     *Env
	corpus  api.CorpusItem
	current string
	saved   []storage.Subcorpus
	list    *selectList

	// checking is the ID of a subcorpus being verified ("" = none)
	checking string
	reqID    int

	// labelInput is non-nil while a label of labelID is being edited
	labelInput *textinput.Model
	labelID    string
	// confirmDelete is the ID of a subcorpus to be forgotten (waiting for `y`)
	confirmDelete string
	err           string
}

func newSubcPicker(env *Env) *subcPicker {
	return &subcPicker{env: env}
}

// Active reports whether the picker is shown
func (p *subcPicker) Active() bool {
	return p.list != nil
}

// Open shows the picker for a corpus with the `current` subcorpus marked
func (p *subcPicker) Open(corpus api.CorpusItem, current string) {
	p.corpus, p.current = corpus, current
	p.checking, p.labelInput, p.confirmDelete, p.err = "", nil, "", ""
	p.reload(current)
}

func (p *subcPicker) close() {
	p.list = nil
	p.checking = ""
	p.reqID++ // a pending check is ignored
}

// reload (re)creates the list with the cursor on `focus`
func (p *subcPicker) reload(focus string) {
	p.saved = nil
	if p.env.DB != nil {
		saved, err := p.env.DB.Subcorpora(p.corpus.ID)
		if err != nil {
			p.err = "failed to load stored subcorpora: " + err.Error()
		}
		p.saved = saved
	}
	p.list = newSelectList("Subcorpus of "+p.corpus.ID, p.items(), p.current).
		withCustom("filter or type an ID", parseSubcID)
	p.list.focusValue(focus)
}

func (p *subcPicker) isAlias(id string) bool {
	for _, s := range p.corpus.Subcorpora {
		if s.ID == id {
			return true
		}
	}
	return false
}

func (p *subcPicker) savedItem(id string) (storage.Subcorpus, bool) {
	for _, s := range p.saved {
		if s.ID == id {
			return s, true
		}
	}
	return storage.Subcorpus{}, false
}

func (p *subcPicker) items() []selectItem {
	items := []selectItem{{Value: "", Label: "(whole corpus)"}}
	for _, s := range p.corpus.Subcorpora {
		hint := "configured"
		if s.Description != "" {
			hint += " · " + s.Description
		}
		items = append(items, selectItem{Value: s.ID, Hint: hint})
	}
	for _, s := range p.saved {
		if p.isAlias(s.ID) {
			continue
		}
		label := s.ID
		if s.Label != "" {
			label = s.Label + " (" + s.ID + ")"
		}
		hint := "used " + s.LastUsed.Format("2006-01-02")
		if s.Size > 0 {
			hint = formatInt(int(s.Size)) + " tokens · " + hint
		}
		items = append(items, selectItem{Value: s.ID, Label: label, Hint: hint})
	}
	return items
}

func (p *subcPicker) check(id string) tea.Cmd {
	p.reqID++
	p.checking = id
	reqID, client, corpusID, lang := p.reqID, p.env.Client, p.corpus.ID, p.env.Lang
	return func() tea.Msg {
		info, err := client.SubcorpusInfo(context.Background(), corpusID, id, lang)
		return subcCheckedMsg{reqID: reqID, info: info, err: err}
	}
}

func selectedSubc(s subcState, notice string) tea.Cmd {
	return func() tea.Msg { return subcSelectedMsg{subc: s, notice: notice} }
}

// Update handles keys (while Active) and subcCheckedMsg
func (p *subcPicker) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case subcCheckedMsg:
		if msg.reqID != p.reqID || p.list == nil {
			return nil
		}
		id := p.checking
		p.checking = ""
		if msg.err != nil {
			var apiErr *api.Error
			if errors.As(msg.err, &apiErr) {
				p.list.err = apiErr.Message
			} else {
				p.list.err = msg.err.Error()
			}
			return nil
		}
		ans := subcState{ID: id, CorpusSize: msg.info.CorpusSize}
		var notice string
		if msg.info.Type == api.SubcTypeAlias {
			ans.Alias = true

		} else {
			ans.Size = msg.info.Size
			if p.env.DB != nil {
				// the subcorpus is selected even if this fails
				if err := p.env.DB.UseSubcorpus(p.corpus.ID, id, msg.info.Size); err != nil {
					notice = "failed to store the subcorpus: " + err.Error()
				}
			}
		}
		p.close()
		return selectedSubc(ans, notice)

	case tea.KeyMsg:
		return p.updateKey(msg)
	}
	return nil
}

func (p *subcPicker) updateKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()
	if p.checking != "" {
		if key == "esc" { // cancel the check
			p.checking = ""
			p.reqID++
		}
		return nil
	}
	if p.labelInput != nil {
		return p.updateLabel(msg)
	}
	if p.confirmDelete != "" {
		if key == "y" {
			if err := p.env.DB.DeleteSubcorpus(p.corpus.ID, p.confirmDelete); err != nil {
				p.err = "failed to remove the subcorpus: " + err.Error()
			}
			p.reload(p.current)
		}
		p.confirmDelete = ""
		return nil
	}
	p.err = ""
	switch key {
	case "ctrl+r", "ctrl+x":
		it, ok := p.list.selected()
		if _, isSaved := p.savedItem(it.Value); !ok || !isSaved || p.env.DB == nil {
			p.list.err = "only previously used saved subcorpora can be labelled or removed"
			return nil
		}
		if key == "ctrl+x" {
			p.confirmDelete = it.Value
			return nil
		}
		s, _ := p.savedItem(it.Value)
		ti := textinput.New()
		ti.Prompt = "› "
		ti.PromptStyle = accentStyle
		ti.Placeholder = "label (empty = none)"
		ti.CharLimit = 100
		ti.Cursor.SetMode(cursor.CursorStatic)
		ti.SetValue(s.Label)
		ti.CursorEnd()
		ti.Focus()
		p.labelInput, p.labelID = &ti, it.Value
		return nil
	}
	res, value := p.list.Update(msg)
	switch res {
	case selectCancelled:
		p.close()
	case selectChosen:
		switch {
		case value == "":
			p.close()
			return selectedSubc(subcState{}, "")
		case p.isAlias(value):
			p.close()
			return selectedSubc(subcState{ID: value, Alias: true}, "")
		default:
			return p.check(value)
		}
	}
	return nil
}

func (p *subcPicker) updateLabel(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		p.labelInput = nil
		return nil
	case "enter":
		label := strings.TrimSpace(p.labelInput.Value())
		if err := p.env.DB.SetSubcorpusLabel(p.corpus.ID, p.labelID, label); err != nil {
			p.err = "failed to save the label: " + err.Error()
		}
		p.labelInput = nil
		p.reload(p.labelID)
		return nil
	}
	ti, _ := p.labelInput.Update(msg)
	p.labelInput = &ti
	return nil
}

// View renders the picker box; maxWidth is the available screen width
func (p *subcPicker) View(maxWidth int) string {
	if p.labelInput != nil {
		w := max(selectMinWidth, min(maxWidth-6, 60))
		p.labelInput.Width = w - 3
		body := titleStyle.Render("Label of "+p.labelID) + "\n" + p.labelInput.View()
		return lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colAccent).
			Padding(0, 1).
			Width(w + 2).
			Render(body)
	}
	box := p.list.View(maxWidth)
	var status string
	switch {
	case p.checking != "":
		status = accentStyle.Render("checking " + p.checking + " …")
	case p.confirmDelete != "":
		status = errorStyle.Render("forget " + p.confirmDelete + "? (y = yes, any other key = no)")
	case p.err != "":
		status = errorStyle.Render(p.err)
	}
	if status == "" {
		return box
	}
	status = ansi.Truncate(status, max(10, lipgloss.Width(box)), "…")
	return lipgloss.JoinVertical(lipgloss.Left, box, status)
}

func (p *subcPicker) Help() string {
	switch {
	case p.checking != "":
		return "esc cancel"
	case p.labelInput != nil:
		return "enter save label • esc cancel"
	case p.confirmDelete != "":
		return "y forget • any other key cancel"
	}
	return "type to filter or enter an ID • ↑/↓ move • enter select • ctrl+r label • ctrl+x forget • esc cancel"
}
