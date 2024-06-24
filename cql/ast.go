package cql

import "encoding/json"

// Query
//
//	Sequence (_ BINAND _ GlobPart)? (_ WithinOrContaining)* EOF {
type Query struct {
	Sequence           *Sequence
	GlobPart           *GlobPart
	WithinOrContaining []*WithinOrContaining
}

// Seq (_ BINOR _ Seq)* / Seq
type Sequence struct {
	Seq []*Seq
}

// GlobPart
// gc:GlobCond gc2:(_ BINAND _ GlobCond)*
type GlobPart struct {
	GlobCond []*GlobCond
}

// WithinOrContaining
//
//	NOT? (KW_WITHIN / KW_CONTAINING) _ WithinContainingPart {
type WithinOrContaining struct {
	KwWithin             string
	KwContaining         string
	WithinContainingPart *WithinContainingPart
}

// WithinContainingPart
//
//	Sequence / WithinNumber / NOT? AlignedPart
type WithinContainingPart struct {
	variant1 struct {
		sequence *Sequence
	}

	variant2 struct {
		withinNumber *WithinNumber
	}

	variant3 struct {
		alignedPart *AlignedPart
	}
}

// GlobCond
//
// v1: NUMBER DOT AttName _ NOT? EQ _ NUMBER DOT AttName {
//
// v2: KW_FREQ LPAREN _ NUMBER DOT AttName _ RPAREN NOT? _ ( EQ / LEQ / GEQ / LSTRUCT / RSTRUCT ) _ NUMBER {

type globCondVariant1 struct {
	Number1  string
	AttName3 string
	Not4     string
	Eq5      string
	Number6  string
	AttName8 string
}

type globCondVariant2 struct {
	KwFreq1   string
	Number2   string
	AttName3  string
	Not4      bool
	Operator5 string
	Number6   string
}

type GlobCond struct {
	variant1 *globCondVariant1

	variant2 *globCondVariant2
}

func (gc *GlobCond) MarshalJSON() ([]byte, error) {
	if gc.variant1 != nil {
		return json.Marshal(
			globCondVariant1{
				Number1:  gc.variant1.Number1,
				AttName3: gc.variant1.AttName3,
				Not4:     gc.variant1.Not4,
				Eq5:      gc.variant1.Eq5,
				Number6:  gc.variant1.Number6,
				AttName8: gc.variant1.AttName8,
			},
		)

	} else if gc.variant2 != nil {
		return json.Marshal(
			globCondVariant2{
				KwFreq1:   gc.variant2.KwFreq1,
				Number2:   gc.variant2.Number2,
				AttName3:  gc.variant2.AttName3,
				Not4:      gc.variant2.Not4,
				Operator5: gc.variant2.Operator5,
				Number6:   gc.variant2.Number6,
			},
		)

	} else {
		return json.Marshal(struct{}{})
	}
}

// Structure
//
// AttName _ AttValList?
type Structure struct {
	attName    string
	attValList *AttValList
}

// AttValList
//
//	av1:AttValAnd av2:(_ BINOR _ AttValAnd)*
type AttValList struct {
	AttValAnd []*AttValAnd
}

// NumberedPosition
//
// NUMBER COLON OnePosition
type NumberedPosition struct {
	Number      string
	Colon       string
	OnePosition *OnePosition
}

type onePositionVariant1 struct {
	AttValList *AttValList
}

type onePositionVariant2 struct {
	RegExp *RegExp
}

type onePositionVariant3 struct {
	Number string
	RegExp *RegExp
}

type onePositionVariant4 struct {
}

type onePositionVariant5 struct {
	MuPart *MuPart
}

// OnePosition
// var1: LBRACKET _ AttValList? _ RBRACKET
// var2: RegExp
// var3: TEQ NUMBER? RegExp
// var4: KW_MU
// var5: MuPart
type OnePosition struct {
	variant1 *onePositionVariant1
	variant2 *onePositionVariant2
	variant3 *onePositionVariant3
	variant4 *onePositionVariant4
	variant5 *onePositionVariant5
}

func (op *OnePosition) MarshalJSON() ([]byte, error) {
	if op.variant1 != nil {
		return json.Marshal(op.variant1)

	} else if op.variant2 != nil {
		return json.Marshal(op.variant2)

	} else if op.variant3 != nil {
		return json.Marshal(op.variant3)

	} else if op.variant4 != nil {
		return json.Marshal(op.variant4)

	} else if op.variant5 != nil {
		return json.Marshal(op.variant5)

	} else {
		return json.Marshal(struct{}{})
	}
}

type positionVariant1 struct {
	OnePosition *OnePosition
}

type positionVariant2 struct {
	NumberedPosition *NumberedPosition
}

// Position
//
//	OnePosition / NumberedPosition
type Position struct {
	variant1 *positionVariant1

	variant2 *positionVariant2
}

func (p *Position) MarshalJSON() ([]byte, error) {
	if p.variant1 != nil {
		return json.Marshal(p.variant1)

	} else if p.variant2 != nil {
		return json.Marshal(p.variant2)

	} else {
		return json.Marshal(struct{}{})
	}
}

type RegExp struct {
	RegExpRaw *RegExpRaw
}

type MuPart struct {
}

type UnionOp struct {
}

type MeetOp struct {
}

type Integer struct {
}

type Seq struct {
	Not        bool
	Repetition []*Repetition
}

type repetitionVariant1 struct {
	RepOpt    string
	AtomQuery *AtomQuery
}

type repetitionVariant2 struct {
	OpenStructTag *OpenStructTag
}

type repetitionVariant3 struct {
	CloseStructTag *CloseStructTag
}

type Repetition struct {
	variant1 *repetitionVariant1
	variant2 *repetitionVariant2
	variant3 *repetitionVariant3
}

func (r *Repetition) MarshalJSON() ([]byte, error) {
	if r.variant1 != nil {
		return json.Marshal(r.variant1)

	} else if r.variant2 != nil {
		return json.Marshal(r.variant2)

	} else if r.variant3 != nil {
		return json.Marshal(r.variant3)

	} else {
		return json.Marshal(struct{}{})
	}
}

type atomQueryVariant1 struct {
	Position *Position
}

type atomQueryVariant2 struct {
	Sequence             *Sequence
	WithinContainingPart *WithinContainingPart
}

// AtomQuery
// var1: Position
// var2: LPAREN _ Sequence (_ NOT? (KW_WITHIN / KW_CONTAINING) _ WithinContainingPart)* _ RPAREN {
type AtomQuery struct {
	variant1 *atomQueryVariant1
	variant2 *atomQueryVariant2
}

func (aq *AtomQuery) MarshalJSON() ([]byte, error) {
	if aq.variant1 != nil {
		return json.Marshal(aq.variant1)

	} else if aq.variant2 != nil {
		return json.Marshal(aq.variant1)

	} else {
		return json.Marshal(struct{}{})
	}
}

type RepOpt struct {
}

type OpenStructTag struct {
}

type CloseStructTag struct {
}

type AlignedPart struct {
}

// AttValAnd
//
//	av1:AttVal av2:(_ BINAND _ AttVal)*
type AttValAnd struct {
	AttVal []*AttVal
}

// AttName _ (NOT)? EEQ _ RawString
type attValVariant1 struct {
	AttName   string
	Not       bool
	Eeq       string
	RawString *RawString
}

// AttName (_ NOT)? _ (EQ / LEQ / GEQ / TEQ NUMBER?) _ RegExp
type attValVariant2 struct {
	AttName string
	Not     bool
	Op      string
	RegExp  *RegExp
}

// POSNUM NUMBER DASH NUMBER
type attValVariant3 struct {
}

// POSNUM NUMBER
type attValVariant4 struct {
}

// NOT AttVal
type attValVariant5 struct {
}

// LPAREN _ AttValList _ RPAREN
type attValVariant6 struct {
}

// (KW_WS / KW_TERM) LPAREN _ (NUMBER COMMA NUMBER / RegExp COMMA RegExp COMMA RegExp) _ RPAREN
type attValVariant7 struct {
}

// KW_SWAP LPAREN _ NUMBER COMMA AttValList _ RPAREN
type attValVariant8 struct {
}

// KW_CCOLL LPAREN _ NUMBER COMMA NUMBER COMMA AttValList _ RPAREN
type attValVariant9 struct {
}

type AttVal struct {
	variant1 *attValVariant1
	variant2 *attValVariant2
	variant3 *attValVariant3
	variant4 *attValVariant4
	variant5 *attValVariant5
	variant6 *attValVariant6
	variant7 *attValVariant7
	variant8 *attValVariant8
	variant9 *attValVariant9
}

func (r *AttVal) MarshalJSON() ([]byte, error) {
	if r.variant1 != nil {
		return json.Marshal(r.variant1)

	} else if r.variant2 != nil {
		return json.Marshal(r.variant2)

	} else if r.variant3 != nil {
		return json.Marshal(r.variant3)

	} else if r.variant4 != nil {
		return json.Marshal(r.variant4)

	} else if r.variant5 != nil {
		return json.Marshal(r.variant5)

	} else if r.variant6 != nil {
		return json.Marshal(r.variant6)

	} else if r.variant7 != nil {
		return json.Marshal(r.variant7)

	} else if r.variant8 != nil {
		return json.Marshal(r.variant8)

	} else if r.variant9 != nil {
		return json.Marshal(r.variant9)

	} else {
		return json.Marshal(struct{}{})
	}
}

type WithinNumber struct {
}

type PhraseQuery struct {
}

type RegExpRaw struct {

	// RgLook / RgGrouped / RgSimple
	Values []any
}

type RawString struct {
	SimpleString *SimpleString
}

type SimpleString struct {
	Values []string
}

type RgGrouped struct {
	Value *RegExpRaw
}

type RgSimple struct {
	// RgRange / RgChar / RgAlt / RgPosixClass
	Values []any
}

type RgPosixClass struct {
}

type RgLook struct {
}

type RgLookOperator struct {
}

type RgAlt struct {
}

type RgChar struct {
}

type RgRange struct {
	RgRangeSpec *RgRangeSpec
}

type RgRangeSpec struct {
	Number1 string
	Number2 string
}

type AnyLetter struct {
}
