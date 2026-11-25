package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ctxTokens is a value of Env.concCtxStruct meaning "no context structure"
// (a structure name cannot contain parentheses)
const ctxTokens = "(tokens)"

const (
	concMaxContextWidth = 50 // see MQuery's ConcordanceMaxWidth
	concMaxPageSize     = 1000
)

// concordance options form fields (in the form order)
const (
	optCtxWidth = iota
	optCtxStruct
	optOrder
	optTextProps
	optPageSize
)

const (
	orderShuffled = "shuffled"
	orderCorpus   = "corpus"
)

func (v *concView) newOptionsForm() *optionsForm {
	structs := []selectItem{
		{Value: "", Label: "corpus default"},
		{Value: ctxTokens, Label: "none (tokens only)"},
	}
	known := v.env.concCtxStruct == "" || v.env.concCtxStruct == ctxTokens
	for _, s := range v.env.structCache[v.corpus.ID] {
		structs = append(structs, selectItem{Value: s.Name, Label: "<" + s.Name + ">"})
		known = known || s.Name == v.env.concCtxStruct
	}
	if !known { // e.g. structures of the corpus are not loaded
		structs = append(structs, selectItem{
			Value: v.env.concCtxStruct, Label: "<" + v.env.concCtxStruct + ">"})
	}
	order := orderShuffled
	if v.env.concNoShuffle {
		order = orderCorpus
	}
	return newOptionsForm(
		"Concordance options",
		newInputField(
			"Context width", fmt.Sprintf("tokens around KWIC in total (0–%d)", concMaxContextWidth),
			strconv.Itoa(v.env.ContextWidth), parsePositiveInt(0, concMaxContextWidth)),
		newChoiceField(
			"Context limited to",
			"the context does not cross the boundaries of the structure containing KWIC "+
				"(by default, the corpus sentence structure, if configured)",
			structs, v.env.concCtxStruct),
		newChoiceField(
			"Line order",
			"shuffled lines are a random sample; corpus order is the order of the texts",
			[]selectItem{
				{Value: orderShuffled, Label: "shuffled"},
				{Value: orderCorpus, Label: "corpus order"},
			}, order),
		newChoiceField(
			"Text properties", "metadata attached to lines (see t in the list)",
			[]selectItem{{Value: "0", Label: "none"}, {Value: "1", Label: "basic"}, {Value: "2", Label: "all"}},
			strconv.Itoa(v.env.concTextProps)),
		newInputField(
			"Lines per page", fmt.Sprintf("1–%d", concMaxPageSize),
			strconv.Itoa(v.env.PageSize), parsePositiveInt(1, concMaxPageSize)),
	)
}

func (v *concView) updateOptions(msg tea.KeyMsg) (ResultView, tea.Cmd) {
	switch v.optsForm.Update(msg) {
	case selectCancelled:
		v.optsForm = nil
	case selectChosen:
		vals, _ := v.optsForm.values() // already validated
		v.optsForm = nil
		return v, v.applyOptions(vals)
	}
	return v, nil
}

// applyOptions stores the options (session-wide) and reloads the view
// if anything has changed
func (v *concView) applyOptions(vals []string) tea.Cmd {
	env := v.env
	width, _ := strconv.Atoi(vals[optCtxWidth])
	textProps, _ := strconv.Atoi(vals[optTextProps])
	pageSize, _ := strconv.Atoi(vals[optPageSize])
	noShuffle := vals[optOrder] == orderCorpus
	if width == env.ContextWidth && vals[optCtxStruct] == env.concCtxStruct &&
		noShuffle == env.concNoShuffle && textProps == env.concTextProps &&
		pageSize == env.PageSize {
		return nil
	}
	switch {
	case noShuffle != env.concNoShuffle:
		v.rowsOffset = 0 // a different order => different pages
	case pageSize != env.PageSize:
		v.rowsOffset = v.rowsOffset / pageSize * pageSize // the page with the first current line
	}
	env.ContextWidth = width
	env.concCtxStruct = vals[optCtxStruct]
	env.concNoShuffle = noShuffle
	env.concTextProps = textProps
	env.PageSize = pageSize
	return v.fetch()
}

// optionsSummary describes non-default options for the summary line
func (v *concView) optionsSummary() string {
	var parts []string
	switch s := v.env.concCtxStruct; s {
	case "":
	case ctxTokens:
		parts = append(parts, fmt.Sprintf("context: %d tokens", v.env.ContextWidth))
	default:
		parts = append(parts, fmt.Sprintf("context: %d within <%s>", v.env.ContextWidth, s))
	}
	if v.env.concNoShuffle {
		parts = append(parts, "corpus order")
	}
	return strings.Join(parts, ", ")
}
