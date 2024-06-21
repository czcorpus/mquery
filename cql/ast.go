package cql

// Query
//
//	Sequence (_ BINAND _ GlobPart)? (_ WithinOrContaining)* EOF {
type Query struct {
	sequence           *Sequence
	globPart           *GlobPart
	withinOrContaining []WithinOrContaining
}

type Sequence struct {
}

type GlobPart struct {
}

// WithinOrContaining
//
//	NOT? (KW_WITHIN / KW_CONTAINING) _ WithinContainingPart {
type WithinOrContaining struct {
	kwWithin             string
	kwContaining         string
	withinContainingPart *WithinContainingPart
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

type GlobCond struct {
	variant1 struct {
		number1  string
		attName3 *AttName
		not4     string
		eq5      string
		number6  string
		attName8 *AttName
	}

	variant2 struct {
		kwFreq1   string
		number2   string
		attName3  *AttName
		not4      bool
		operator5 string
		number6   string
	}
}

type AttName struct {
}

// Structure
//
// AttName _ AttValList?
type Structure struct {
	attName    *AttName
	attValList *AttValList
}

type AttValList struct {
}

// NumberedPosition
//
// NUMBER COLON OnePosition
type NumberedPosition struct {
	number      string
	colon       string
	onePosition *OnePosition
}

type OnePosition struct {
}

// Position
//
//	OnePosition / NumberedPosition
type Position struct {
	variant1 struct {
		onePosition *OnePosition
	}

	variant2 struct {
		numberedPosition *NumberedPosition
	}
}

type RegExp struct {
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
}

type Repetition struct {
}

type AtomQuery struct {
}

type RepOpt struct {
}

type OpenStructTag struct {
}

type CloseStructTag struct {
}

type AlignedPart struct {
}

type AttValAnd struct {
}

type WithinNumber struct {
}

type PhraseQuery struct {
}

type RegExpRaw struct {
}

type RawString struct {
}

type SimpleString struct {
}

type RgGrouped struct {
}

type RgSimple struct {
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
}

type RgRangeSpec struct {
}

type AnyLetter struct {
}
