package cql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEscapeValue(t *testing.T) {
	assert.Equal(t, `\.`, EscapeValue("."))
	assert.Equal(t, `a\+b\(c\)`, EscapeValue("a+b(c)"))
	assert.Equal(t, `\"x\"`, EscapeValue(`"x"`))
	assert.Equal(t, `£2,000`, EscapeValue("£2,000"))
}

func TestAttrCond(t *testing.T) {
	assert.Equal(t, `lemma="house"`, AttrCond("lemma", "house", false))
	assert.Equal(t, `word="(?i)\?"`, AttrCond("word", "?", true))
}

func TestSplitPositions(t *testing.T) {
	tests := []struct {
		q    string
		want []string
		ok   bool
	}{
		{`[lemma="house"]`, []string{`lemma="house"`}, true},
		{`  [lemma="house"]  [tag="V.*"] `, []string{`lemma="house"`, `tag="V.*"`}, true},
		{`[lemma="house"][]`, []string{`lemma="house"`, ``}, true},
		{`[word="\]" & tag="[NV].*"]`, []string{`word="\]" & tag="[NV].*"`}, true},
		{`[word="a"]{2}`, nil, false},
		{`[word="a"] within <s/>`, nil, false},
		{`[word="a"] | [word="b"]`, nil, false},
		{`"house"`, nil, false},
		{`[word="a"`, nil, false},
		{``, nil, false},
	}
	for _, tt := range tests {
		got, ok := SplitPositions(tt.q)
		assert.Equal(t, tt.ok, ok, tt.q)
		assert.Equal(t, tt.want, got, tt.q)
	}
}

func TestRestrictByValues(t *testing.T) {
	q, merged := RestrictByValues(`[lemma="house"] [tag="V.*"]`, "word", []string{"houses", "are"}, true)
	assert.True(t, merged)
	assert.Equal(t, `[(lemma="house") & word="(?i)houses"] [(tag="V.*") & word="(?i)are"]`, q)

	q, merged = RestrictByValues(`[lemma="house"] []`, "tag", []string{"NN1", "VBZ"}, false)
	assert.True(t, merged)
	assert.Equal(t, `[(lemma="house") & tag="NN1"] [tag="VBZ"]`, q)

	q, merged = RestrictByValues(`[lemma="house"]{2}`, "word", []string{"house", "house"}, false)
	assert.False(t, merged)
	assert.Equal(t, `[word="house"] [word="house"]`, q)

	q, merged = RestrictByValues(`[lemma="house"]`, "word", []string{"away", "from"}, false)
	assert.False(t, merged)
	assert.Equal(t, `[word="away"] [word="from"]`, q)
}

func TestWithinStructAttr(t *testing.T) {
	q, err := WithinStructAttr(`[lemma="house"] | [lemma="home"]`, "bncdoc.alltyp", "Spoken demographic")
	assert.NoError(t, err)
	assert.Equal(t, `([lemma="house"] | [lemma="home"]) within <bncdoc alltyp="Spoken demographic"/>`, q)

	q, err = WithinStructAttr(`[word="a"]`, "doc.title", `Who? "Me" (1.)`)
	assert.NoError(t, err)
	assert.Equal(t, `([word="a"]) within <doc title="Who\? \"Me\" \(1\.\)"/>`, q)

	q, err = WithinStructAttr(`[word="a"]`, "doc.author", "")
	assert.NoError(t, err)
	assert.Equal(t, `([word="a"]) within <doc author=""/>`, q)

	_, err = WithinStructAttr(`[word="a"]`, "author", "x")
	assert.Error(t, err)
}
