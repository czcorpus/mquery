package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseWindow(t *testing.T) {
	for in, want := range map[string]string{"3,5": "3,5", "3 5": "3,5", "4": "4,4", "0,2": "0,2", " 1 , 0 ": "1,0"} {
		got, err := parseWindow(in)
		assert.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	for _, in := range []string{"0,0", "0", "a,b", "1,2,3", "-1,3", "51,1", ""} {
		_, err := parseWindow(in)
		assert.Error(t, err, in)
	}
}

func TestCollConcRange(t *testing.T) {
	l, r := collConcRange([2]int{5, 5})
	assert.Equal(t, [2]int{-5, 5}, [2]int{l, r})
	l, r = collConcRange([2]int{5, 0})
	assert.Equal(t, [2]int{-5, -1}, [2]int{l, r})
	l, r = collConcRange([2]int{0, 3})
	assert.Equal(t, [2]int{1, 3}, [2]int{l, r})
}

func TestWindowLabel(t *testing.T) {
	assert.Equal(t, "-5 … +5", windowLabel([2]int{5, 5}))
	assert.Equal(t, "left 3", windowLabel([2]int{3, 0}))
	assert.Equal(t, "right 1", windowLabel([2]int{0, 1}))
}
