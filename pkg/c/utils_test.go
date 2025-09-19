package cmcts

import (
	"testing"
	"unsafe"
)

func boolToIntGo(b bool) int {
	return int(*(*byte)(unsafe.Pointer(&b)))
}

func BenchmarkBoolToIntC(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var s = BoolToInt(false)
		_ = s
	}
}

var result int

func BenchmarkBoolToIntGo(b *testing.B) {
	var r int
	for i := 0; i < b.N; i++ {
		r = boolToIntGo(false)
	}
	result = r
}
