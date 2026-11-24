package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func typeKeys(s *selectList, text string) {
	for _, r := range text {
		s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

func TestSelectListStartsOnCurrent(t *testing.T) {
	s := newSelectList("x", intSelectItems([]int{1, 2, 5, 10}), "5")
	res, val := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, selectChosen, res)
	assert.Equal(t, "5", val)
}

func TestSelectListExactMatchFirst(t *testing.T) {
	s := newSelectList("x", intSelectItems([]int{1, 100, 10, 1000}), "1")
	typeKeys(s, "10")
	res, val := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, selectChosen, res)
	assert.Equal(t, "10", val)
}

func TestSelectListCustomValue(t *testing.T) {
	s := newSelectList("x", intSelectItems([]int{1, 2}), "1").
		withCustom("", parsePositiveInt(1, 100))
	typeKeys(s, "7")
	res, val := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, selectChosen, res)
	assert.Equal(t, "7", val)
}

func TestSelectListInvalidCustomValue(t *testing.T) {
	s := newSelectList("x", intSelectItems([]int{1, 2}), "1").
		withCustom("", parsePositiveInt(1, 100))
	typeKeys(s, "500")
	res, _ := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, selectPending, res)
	assert.NotEmpty(t, s.err)
}

func TestSelectListNoCustomWithoutMatch(t *testing.T) {
	s := newSelectList("x", []selectItem{{Value: "lemma"}, {Value: "tag"}}, "tag")
	typeKeys(s, "foo")
	res, _ := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, selectPending, res)
	res, _ = s.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, selectCancelled, res)
}
