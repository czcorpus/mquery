package cql

import (
	"fmt"
	"reflect"
)

func fromIdxOfUntypedSlice[T any](arr any, idx int) T {
	if arr == nil {
		var t T
		return t
	}
	tmp, ok := arr.([]any)
	if !ok {
		panic("value must be a slice")
	}
	v := tmp[idx]
	vt, ok := v.(T)
	if !ok {
		panic(fmt.Sprintf("value with idx %d has invalid type %s", idx, reflect.TypeOf(v)))
	}
	return vt
}

func anyToSlice(v any) []any {
	if v == nil {
		return []any{}
	}
	vt, ok := v.([]any)
	if !ok {
		panic("expecting a slice")
	}
	return vt
}

func typedOrPanic[T any](v any) T {
	if v == nil {
		var ans T
		return ans
	}
	vt, ok := v.(T)
	if !ok {
		panic(fmt.Sprintf("unexpected type %s", reflect.TypeOf(v)))
	}
	return vt
}
